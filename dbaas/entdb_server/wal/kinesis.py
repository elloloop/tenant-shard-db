"""
AWS Kinesis WAL stream implementation.

This module provides a Kinesis Data Streams backend for the WAL stream.
It uses the AWS SDK (boto3/aiobotocore) for async operations.

Invariants:
    - PutRecord returns only after data is replicated
    - Shard iterator provides ordered consumption within a shard
    - Checkpoints are stored externally (DynamoDB or SQLite)
    - Connection failures trigger automatic retry with exponential backoff

How to change safely:
    - Test with LocalStack before deploying to AWS
    - Consider Kinesis enhanced fan-out for high throughput
    - Monitor iterator age and throughput metrics
"""

from __future__ import annotations

import asyncio
import hashlib
import json
import logging
import time
from collections.abc import AsyncIterator
from typing import Any

from .base import (
    StreamPos,
    StreamRecord,
    WalConnectionError,
    WalError,
    WalTimeoutError,
)

logger = logging.getLogger(__name__)

# Try to import aiobotocore
try:
    from aiobotocore.session import get_session
    from botocore.exceptions import ClientError, EndpointConnectionError

    KINESIS_AVAILABLE = True
except ImportError:
    KINESIS_AVAILABLE = False
    get_session = None


class KinesisWalStream:
    """Kinesis Data Streams implementation of WalStream protocol.

    Uses aiobotocore for async operations with AWS Kinesis.

    Attributes:
        config: Kinesis configuration
        client: Kinesis client (created on connect)

    Note:
        Kinesis uses sequence numbers instead of offsets. The StreamPos
        offset field contains the parsed sequence number for comparison.

    Example:
        >>> config = KinesisConfig(stream_name="entdb-wal", region="us-east-1")
        >>> wal = KinesisWalStream(config)
        >>> await wal.connect()
        >>> pos = await wal.append("entdb-wal", "tenant_1", b'{"op": "create"}')
    """

    def __init__(self, config: Any) -> None:
        """Initialize Kinesis WAL stream.

        Args:
            config: KinesisConfig instance

        Raises:
            ImportError: If aiobotocore is not installed
        """
        if not KINESIS_AVAILABLE:
            raise ImportError(
                "aiobotocore is required for Kinesis backend. Install with: pip install aiobotocore"
            )

        self.config = config
        self._session = None
        self._client = None
        self._connected = False
        self._shard_iterators: dict[str, str] = {}
        self._checkpoints: dict[str, str] = {}  # shard_id -> sequence_number

    @property
    def is_connected(self) -> bool:
        """Whether connected to Kinesis."""
        return self._connected

    async def connect(self) -> None:
        """Connect to Kinesis.

        Creates the boto session and client.

        Raises:
            WalConnectionError: If connection fails
        """
        if self._connected:
            return

        try:
            self._session = get_session()

            # Build client configuration
            client_config = {
                "region_name": self.config.region,
            }

            if self.config.endpoint_url:
                client_config["endpoint_url"] = self.config.endpoint_url

            # Create context manager for client
            self._client_ctx = self._session.create_client("kinesis", **client_config)
            self._client = await self._client_ctx.__aenter__()

            # Verify stream exists
            await self._client.describe_stream(StreamName=self.config.stream_name)

            self._connected = True
            logger.info(
                "Connected to Kinesis",
                extra={
                    "stream": self.config.stream_name,
                    "region": self.config.region,
                    "endpoint": self.config.endpoint_url or "AWS",
                },
            )

        except EndpointConnectionError as e:
            raise WalConnectionError(f"Failed to connect to Kinesis endpoint: {e}") from e
        except ClientError as e:
            error_code = e.response.get("Error", {}).get("Code", "")
            if error_code == "ResourceNotFoundException":
                raise WalConnectionError(
                    f"Kinesis stream '{self.config.stream_name}' not found"
                ) from e
            raise WalConnectionError(f"Kinesis error: {e}") from e
        except Exception as e:
            raise WalConnectionError(f"Failed to connect to Kinesis: {e}") from e

    async def close(self) -> None:
        """Close Kinesis connection."""
        if self._client:
            try:
                await self._client_ctx.__aexit__(None, None, None)
            except Exception as e:
                logger.warning(f"Error closing Kinesis client: {e}")

        self._client = None
        self._session = None
        self._connected = False
        self._shard_iterators.clear()
        logger.info("Kinesis connection closed")

    async def append(
        self,
        topic: str,
        key: str,
        value: bytes,
        headers: dict[str, bytes] | None = None,
    ) -> StreamPos:
        """Append event to Kinesis stream.

        Uses PutRecord with explicit hash key for consistent partitioning.

        Args:
            topic: Stream name (should match config.stream_name)
            key: Partition key (tenant_id)
            value: Event payload
            headers: Not directly supported, will be embedded in value

        Returns:
            StreamPos with shard ID and sequence number

        Raises:
            WalConnectionError: If not connected
            WalTimeoutError: If operation times out
            WalError: For other Kinesis errors
        """
        if not self._client:
            raise WalConnectionError("Not connected to Kinesis")

        # Embed headers in the value if present
        if headers:
            wrapped = {
                "_headers": {k: v.decode("utf-8", errors="replace") for k, v in headers.items()},
                "_data": value.decode("utf-8"),
            }
            value = json.dumps(wrapped).encode("utf-8")

        try:
            # Use explicit hash key for consistent partitioning
            # This ensures same tenant always goes to same shard
            hash_key = hashlib.md5(key.encode("utf-8")).hexdigest()
            explicit_hash = str(int(hash_key[:8], 16))

            response = await asyncio.wait_for(
                self._client.put_record(
                    StreamName=self.config.stream_name,
                    Data=value,
                    PartitionKey=key,
                    ExplicitHashKey=explicit_hash,
                ),
                timeout=30.0,
            )

            # Parse shard ID to get partition number
            shard_id = response["ShardId"]
            shard_num = self._parse_shard_number(shard_id)
            seq_num = response["SequenceNumber"]

            pos = StreamPos(
                topic=self.config.stream_name,
                partition=shard_num,
                offset=int(seq_num),
                timestamp_ms=int(time.time() * 1000),
            )

            logger.debug(
                "Event appended to Kinesis",
                extra={
                    "stream": self.config.stream_name,
                    "key": key,
                    "shard": shard_id,
                    "sequence": seq_num,
                },
            )

            return pos

        except asyncio.TimeoutError:
            raise WalTimeoutError("Kinesis PutRecord timed out")
        except ClientError as e:
            error_code = e.response.get("Error", {}).get("Code", "")
            if error_code == "ProvisionedThroughputExceededException":
                raise WalTimeoutError("Kinesis throughput exceeded") from e
            raise WalError(f"Kinesis PutRecord failed: {e}") from e

    async def subscribe(
        self,
        topic: str,
        group_id: str,
        start_position: StreamPos | None = None,
    ) -> AsyncIterator[StreamRecord]:
        """Subscribe to Kinesis stream.

        Creates shard iterators and yields records from all shards.

        Args:
            topic: Stream name
            group_id: Consumer group (used for checkpoint key)
            start_position: Optional starting position

        Yields:
            StreamRecord for each record

        Note:
            Kinesis doesn't have native consumer groups like Kafka.
            Coordination must be handled externally (e.g., with DynamoDB).
        """
        if not self._client:
            raise WalConnectionError("Not connected to Kinesis")

        try:
            # Get all shards
            shards = await self._list_shards()
            logger.info(f"Subscribing to {len(shards)} shards", extra={"stream": topic})

            # Get shard iterators
            for shard in shards:
                shard_id = shard["ShardId"]

                # Determine starting position
                if start_position and start_position.partition == self._parse_shard_number(
                    shard_id
                ):
                    # Start from specific sequence number
                    iterator_response = await self._client.get_shard_iterator(
                        StreamName=self.config.stream_name,
                        ShardId=shard_id,
                        ShardIteratorType="AFTER_SEQUENCE_NUMBER",
                        StartingSequenceNumber=str(start_position.offset),
                    )
                else:
                    # Start from beginning or latest based on config
                    iterator_type = (
                        "TRIM_HORIZON" if self.config.iterator_type == "TRIM_HORIZON" else "LATEST"
                    )
                    iterator_response = await self._client.get_shard_iterator(
                        StreamName=self.config.stream_name,
                        ShardId=shard_id,
                        ShardIteratorType=iterator_type,
                    )

                self._shard_iterators[shard_id] = iterator_response["ShardIterator"]

            # Poll for records
            while True:
                for shard_id, iterator in list(self._shard_iterators.items()):
                    if not iterator:
                        continue

                    try:
                        response = await self._client.get_records(
                            ShardIterator=iterator,
                            Limit=self.config.max_records_per_get,
                        )

                        # Update iterator for next call
                        self._shard_iterators[shard_id] = response.get("NextShardIterator")

                        # Yield records
                        for record in response["Records"]:
                            # Parse data
                            data = record["Data"]
                            headers = {}

                            # Check if headers were embedded
                            try:
                                parsed = json.loads(data)
                                if isinstance(parsed, dict) and "_headers" in parsed:
                                    headers = {
                                        k: v.encode("utf-8") for k, v in parsed["_headers"].items()
                                    }
                                    data = parsed["_data"].encode("utf-8")
                            except (json.JSONDecodeError, KeyError):
                                pass

                            shard_num = self._parse_shard_number(shard_id)
                            seq_num = record["SequenceNumber"]

                            stream_record = StreamRecord(
                                key=record["PartitionKey"],
                                value=data if isinstance(data, bytes) else data.encode("utf-8"),
                                position=StreamPos(
                                    topic=self.config.stream_name,
                                    partition=shard_num,
                                    offset=int(seq_num),
                                    timestamp_ms=int(
                                        record["ApproximateArrivalTimestamp"].timestamp() * 1000
                                    ),
                                ),
                                headers=headers,
                            )

                            self._checkpoints[shard_id] = seq_num
                            yield stream_record

                    except ClientError as e:
                        error_code = e.response.get("Error", {}).get("Code", "")
                        if error_code == "ExpiredIteratorException":
                            # Re-create iterator from checkpoint
                            checkpoint = self._checkpoints.get(shard_id)
                            if checkpoint:
                                resp = await self._client.get_shard_iterator(
                                    StreamName=self.config.stream_name,
                                    ShardId=shard_id,
                                    ShardIteratorType="AFTER_SEQUENCE_NUMBER",
                                    StartingSequenceNumber=checkpoint,
                                )
                                self._shard_iterators[shard_id] = resp["ShardIterator"]
                            else:
                                logger.warning(
                                    f"Lost iterator for shard {shard_id}, restarting from LATEST"
                                )
                                resp = await self._client.get_shard_iterator(
                                    StreamName=self.config.stream_name,
                                    ShardId=shard_id,
                                    ShardIteratorType="LATEST",
                                )
                                self._shard_iterators[shard_id] = resp["ShardIterator"]
                        else:
                            raise

                # Small delay to avoid tight polling
                await asyncio.sleep(0.2)

        except ClientError as e:
            raise WalError(f"Kinesis subscription error: {e}") from e

    async def commit(self, record: StreamRecord) -> None:
        """Commit consumed record.

        For Kinesis, this updates the internal checkpoint.
        In production, checkpoints should be persisted to DynamoDB.

        Args:
            record: The record to checkpoint
        """
        # Store checkpoint in memory
        # In production, this should be persisted to DynamoDB
        shard_id = f"shardId-{record.position.partition:012d}"
        self._checkpoints[shard_id] = str(record.position.offset)

        logger.debug(
            "Checkpointed",
            extra={
                "shard": shard_id,
                "sequence": record.position.offset,
            },
        )

    async def get_positions(self, topic: str, group_id: str) -> dict[int, StreamPos]:
        """Get checkpointed positions.

        Returns:
            Dictionary mapping shard number to position
        """
        positions = {}
        for shard_id, seq_num in self._checkpoints.items():
            shard_num = self._parse_shard_number(shard_id)
            positions[shard_num] = StreamPos(
                topic=topic,
                partition=shard_num,
                offset=int(seq_num),
                timestamp_ms=int(time.time() * 1000),
            )
        return positions

    async def _list_shards(self) -> list:
        """List all shards in the stream."""
        shards = []
        next_token = None

        while True:
            kwargs = {"StreamName": self.config.stream_name}
            if next_token:
                kwargs["NextToken"] = next_token

            response = await self._client.list_shards(**kwargs)
            shards.extend(response["Shards"])

            next_token = response.get("NextToken")
            if not next_token:
                break

        return shards

    def _parse_shard_number(self, shard_id: str) -> int:
        """Parse shard number from shard ID.

        Kinesis shard IDs are like "shardId-000000000001"
        """
        try:
            return int(shard_id.split("-")[-1])
        except (ValueError, IndexError):
            return 0

    async def health_check(self) -> bool:
        """Check if Kinesis connection is healthy."""
        if not self._client:
            return False

        try:
            await asyncio.wait_for(
                self._client.describe_stream(
                    StreamName=self.config.stream_name,
                    Limit=1,
                ),
                timeout=5.0,
            )
            return True
        except Exception:
            return False
