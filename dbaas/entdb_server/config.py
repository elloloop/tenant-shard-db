"""
Configuration management for EntDB Server.

All configuration is done via environment variables - no config files inside containers.
This module provides typed configuration classes with validation.

Invariants:
    - All settings have sensible defaults for local development
    - Production deployments MUST set explicit values for critical settings
    - Secrets are never logged or exposed in error messages

How to change safely:
    - Add new settings with defaults that maintain backward compatibility
    - Deprecate settings by logging warnings but continuing to support them
    - Document all new settings in docs/deployment.md
"""

from __future__ import annotations

import logging
import os
from dataclasses import dataclass, field
from enum import Enum

from .sharding import ShardingConfig

logger = logging.getLogger(__name__)


class WalBackend(Enum):
    """Supported WAL stream backends."""

    KAFKA = "kafka"
    KINESIS = "kinesis"
    PUBSUB = "pubsub"
    SQS = "sqs"
    SERVICEBUS = "servicebus"
    LOCAL = "local"


@dataclass(frozen=True)
class GrpcConfig:
    """gRPC server configuration.

    Attributes:
        bind_address: Address to bind gRPC server (host:port)
        max_workers: Maximum number of thread pool workers
        max_message_size: Maximum message size in bytes
        reflection_enabled: Whether to enable gRPC reflection for debugging
    """

    bind_address: str = "0.0.0.0:50051"
    max_workers: int = 10
    max_message_size: int = 64 * 1024 * 1024  # 64MB
    reflection_enabled: bool = True

    @classmethod
    def from_env(cls) -> GrpcConfig:
        """Load configuration from environment variables."""
        return cls(
            bind_address=os.getenv("GRPC_BIND", "0.0.0.0:50051"),
            max_workers=int(os.getenv("GRPC_MAX_WORKERS", "10")),
            max_message_size=int(os.getenv("GRPC_MAX_MESSAGE_SIZE", str(64 * 1024 * 1024))),
            reflection_enabled=os.getenv("GRPC_REFLECTION", "true").lower() == "true",
        )


@dataclass(frozen=True)
class KafkaConfig:
    """Kafka/Redpanda WAL backend configuration.

    Attributes:
        brokers: Comma-separated list of broker addresses
        topic: Topic name for WAL events
        consumer_group: Consumer group ID for applier
        sasl_mechanism: SASL authentication mechanism (PLAIN, SCRAM-SHA-256, etc.)
        sasl_username: SASL username (if authentication enabled)
        sasl_password: SASL password (if authentication enabled)
        security_protocol: Security protocol (PLAINTEXT, SSL, SASL_PLAINTEXT, SASL_SSL)
        ssl_cafile: Path to CA certificate file
        ssl_certfile: Path to client certificate file
        ssl_keyfile: Path to client key file
        acks: Producer acknowledgment level ('all' for strongest durability)
        enable_idempotence: Enable idempotent producer
        max_in_flight: Maximum in-flight requests per connection
    """

    brokers: str = "localhost:9092"
    topic: str = "entdb-wal"
    consumer_group: str = "entdb-applier"
    sasl_mechanism: str | None = None
    sasl_username: str | None = None
    sasl_password: str | None = None
    security_protocol: str = "PLAINTEXT"
    ssl_cafile: str | None = None
    ssl_certfile: str | None = None
    ssl_keyfile: str | None = None
    # Producer durability settings
    acks: str = "all"
    enable_idempotence: bool = True
    max_in_flight: int = 5
    # Consumer settings
    auto_offset_reset: str = "earliest"
    enable_auto_commit: bool = False
    max_poll_records: int = 100
    poll_timeout_ms: int = 100

    @classmethod
    def from_env(cls) -> KafkaConfig:
        """Load configuration from environment variables."""
        return cls(
            brokers=os.getenv("KAFKA_BROKERS", "localhost:9092"),
            topic=os.getenv("KAFKA_TOPIC", "entdb-wal"),
            consumer_group=os.getenv(
                "KAFKA_CONSUMER_GROUP",
                f"entdb-applier-{os.getenv('NODE_ID', 'default')}",
            ),
            sasl_mechanism=os.getenv("KAFKA_SASL_MECHANISM"),
            sasl_username=os.getenv("KAFKA_SASL_USERNAME"),
            sasl_password=os.getenv("KAFKA_SASL_PASSWORD"),
            security_protocol=os.getenv("KAFKA_SECURITY_PROTOCOL", "PLAINTEXT"),
            ssl_cafile=os.getenv("KAFKA_SSL_CAFILE"),
            ssl_certfile=os.getenv("KAFKA_SSL_CERTFILE"),
            ssl_keyfile=os.getenv("KAFKA_SSL_KEYFILE"),
            acks=os.getenv("KAFKA_ACKS", "all"),
            enable_idempotence=os.getenv("KAFKA_ENABLE_IDEMPOTENCE", "true").lower() == "true",
            max_in_flight=int(os.getenv("KAFKA_MAX_IN_FLIGHT", "5")),
            auto_offset_reset=os.getenv("KAFKA_AUTO_OFFSET_RESET", "earliest"),
            enable_auto_commit=os.getenv("KAFKA_AUTO_COMMIT", "false").lower() == "true",
            max_poll_records=int(os.getenv("KAFKA_MAX_POLL_RECORDS", "100")),
            poll_timeout_ms=int(os.getenv("KAFKA_POLL_TIMEOUT_MS", "100")),
        )


@dataclass(frozen=True)
class KinesisConfig:
    """AWS Kinesis WAL backend configuration.

    Attributes:
        stream_name: Kinesis stream name
        region: AWS region
        endpoint_url: Custom endpoint URL (for LocalStack testing)
        max_records_per_get: Maximum records per GetRecords call
        iterator_type: Shard iterator type (TRIM_HORIZON, LATEST, etc.)
    """

    stream_name: str = "entdb-wal"
    region: str = "us-east-1"
    endpoint_url: str | None = None
    max_records_per_get: int = 1000
    iterator_type: str = "TRIM_HORIZON"

    @classmethod
    def from_env(cls) -> KinesisConfig:
        """Load configuration from environment variables."""
        return cls(
            stream_name=os.getenv("KINESIS_STREAM_NAME", "entdb-wal"),
            region=os.getenv("AWS_REGION", os.getenv("AWS_DEFAULT_REGION", "us-east-1")),
            endpoint_url=os.getenv("KINESIS_ENDPOINT_URL"),
            max_records_per_get=int(os.getenv("KINESIS_MAX_RECORDS", "1000")),
            iterator_type=os.getenv("KINESIS_ITERATOR_TYPE", "TRIM_HORIZON"),
        )


@dataclass(frozen=True)
class PubSubConfig:
    """Google Cloud Pub/Sub WAL backend configuration.

    Attributes:
        project_id: GCP project ID
        topic_id: Pub/Sub topic ID (not full path)
        subscription_id: Pub/Sub subscription ID
        ordering_enabled: Whether to use ordering keys (recommended)
        max_messages: Maximum messages per pull request
        ack_deadline_seconds: Time before unacked messages are redelivered
        endpoint: Custom endpoint (for emulator)
    """

    project_id: str = ""
    topic_id: str = "entdb-wal"
    subscription_id: str = "entdb-applier"
    ordering_enabled: bool = True
    max_messages: int = 100
    ack_deadline_seconds: int = 60
    endpoint: str | None = None

    @classmethod
    def from_env(cls) -> PubSubConfig:
        """Load configuration from environment variables."""
        return cls(
            project_id=os.getenv("PUBSUB_PROJECT_ID", os.getenv("GCP_PROJECT_ID", "")),
            topic_id=os.getenv("PUBSUB_TOPIC_ID", "entdb-wal"),
            subscription_id=os.getenv("PUBSUB_SUBSCRIPTION_ID", "entdb-applier"),
            ordering_enabled=os.getenv("PUBSUB_ORDERING", "true").lower() == "true",
            max_messages=int(os.getenv("PUBSUB_MAX_MESSAGES", "100")),
            ack_deadline_seconds=int(os.getenv("PUBSUB_ACK_DEADLINE", "60")),
            endpoint=os.getenv("PUBSUB_ENDPOINT"),
        )


@dataclass(frozen=True)
class SqsConfig:
    """AWS SQS FIFO WAL backend configuration.

    Attributes:
        queue_url: SQS FIFO queue URL
        region: AWS region
        endpoint_url: Custom endpoint URL (for LocalStack)
        max_messages: Maximum messages per ReceiveMessage call (SQS max is 10)
        wait_time_seconds: Long polling wait time in seconds
        visibility_timeout: Visibility timeout in seconds
    """

    queue_url: str = ""
    region: str = "us-east-1"
    endpoint_url: str | None = None
    max_messages: int = 10
    wait_time_seconds: int = 5
    visibility_timeout: int = 60

    @classmethod
    def from_env(cls) -> SqsConfig:
        """Load configuration from environment variables."""
        return cls(
            queue_url=os.getenv("SQS_QUEUE_URL", ""),
            region=os.getenv("SQS_REGION", os.getenv("AWS_REGION", "us-east-1")),
            endpoint_url=os.getenv("SQS_ENDPOINT"),
            max_messages=int(os.getenv("SQS_MAX_MESSAGES", "10")),
            wait_time_seconds=int(os.getenv("SQS_WAIT_TIME", "5")),
            visibility_timeout=int(os.getenv("SQS_VISIBILITY_TIMEOUT", "60")),
        )


@dataclass(frozen=True)
class ServiceBusConfig:
    """Azure Service Bus WAL backend configuration.

    Attributes:
        connection_string: Service Bus connection string
        queue_name: Queue name
        max_messages: Maximum messages per receive call
        max_wait_time_seconds: Maximum wait time for receive operations
    """

    connection_string: str = ""
    queue_name: str = "entdb-wal"
    max_messages: int = 20
    max_wait_time_seconds: int = 5

    @classmethod
    def from_env(cls) -> ServiceBusConfig:
        """Load configuration from environment variables."""
        return cls(
            connection_string=os.getenv("SERVICEBUS_CONNECTION_STRING", ""),
            queue_name=os.getenv("SERVICEBUS_QUEUE_NAME", "entdb-wal"),
            max_messages=int(os.getenv("SERVICEBUS_MAX_MESSAGES", "20")),
            max_wait_time_seconds=int(os.getenv("SERVICEBUS_MAX_WAIT", "5")),
        )


@dataclass(frozen=True)
class S3Config:
    """S3 configuration for archiving and snapshots.

    Attributes:
        bucket: S3 bucket name
        region: AWS region
        endpoint_url: Custom endpoint URL (for MinIO)
        archive_prefix: Prefix for archived WAL segments
        snapshot_prefix: Prefix for SQLite snapshots
        access_key_id: AWS access key ID (optional, uses AWS credential chain)
        secret_access_key: AWS secret access key (optional)
    """

    bucket: str = "entdb-storage"
    region: str = "us-east-1"
    endpoint_url: str | None = None
    archive_prefix: str = "archive"
    snapshot_prefix: str = "snapshots"
    access_key_id: str | None = None
    secret_access_key: str | None = None

    @classmethod
    def from_env(cls) -> S3Config:
        """Load configuration from environment variables."""
        return cls(
            bucket=os.getenv("S3_BUCKET", "entdb-storage"),
            region=os.getenv("S3_REGION", os.getenv("AWS_REGION", "us-east-1")),
            endpoint_url=os.getenv("S3_ENDPOINT"),
            archive_prefix=os.getenv("S3_ARCHIVE_PREFIX", "archive"),
            snapshot_prefix=os.getenv("S3_SNAPSHOT_PREFIX", "snapshots"),
            access_key_id=os.getenv("AWS_ACCESS_KEY_ID"),
            secret_access_key=os.getenv("AWS_SECRET_ACCESS_KEY"),
        )


@dataclass(frozen=True)
class StorageConfig:
    """Local storage configuration.

    Attributes:
        data_dir: Directory for SQLite databases
        tenant_db_pattern: Pattern for tenant database files
        mailbox_db_pattern: Pattern for mailbox database files
        wal_mode: SQLite WAL mode enabled
        busy_timeout_ms: SQLite busy timeout in milliseconds
        cache_size_pages: SQLite cache size in pages (negative = KB)
    """

    data_dir: str = "/var/lib/entdb"
    tenant_db_pattern: str = "tenant_{tenant_id}.db"
    mailbox_db_pattern: str = "mailbox_{tenant_id}_{user_id}.db"
    wal_mode: bool = True
    busy_timeout_ms: int = 5000
    cache_size_pages: int = -64000  # 64MB

    @classmethod
    def from_env(cls) -> StorageConfig:
        """Load configuration from environment variables."""
        return cls(
            data_dir=os.getenv("DATA_DIR", "/var/lib/entdb"),
            tenant_db_pattern=os.getenv("TENANT_DB_PATTERN", "tenant_{tenant_id}.db"),
            mailbox_db_pattern=os.getenv("MAILBOX_DB_PATTERN", "mailbox_{tenant_id}_{user_id}.db"),
            wal_mode=os.getenv("SQLITE_WAL_MODE", "true").lower() == "true",
            busy_timeout_ms=int(os.getenv("SQLITE_BUSY_TIMEOUT_MS", "5000")),
            cache_size_pages=int(os.getenv("SQLITE_CACHE_SIZE", "-64000")),
        )


@dataclass(frozen=True)
class ApplierConfig:
    """Applier loop configuration.

    Attributes:
        batch_size: Maximum events to process in one batch
        commit_interval_ms: Maximum time between commits
        retry_delay_ms: Delay between retries on transient errors
        max_retries: Maximum retries for transient errors
    """

    batch_size: int = 100
    commit_interval_ms: int = 1000
    retry_delay_ms: int = 100
    max_retries: int = 3

    @classmethod
    def from_env(cls) -> ApplierConfig:
        """Load configuration from environment variables."""
        return cls(
            batch_size=int(os.getenv("APPLIER_BATCH_SIZE", "100")),
            commit_interval_ms=int(os.getenv("APPLIER_COMMIT_INTERVAL_MS", "1000")),
            retry_delay_ms=int(os.getenv("APPLIER_RETRY_DELAY_MS", "100")),
            max_retries=int(os.getenv("APPLIER_MAX_RETRIES", "3")),
        )


class ArchiveFlushMode(Enum):
    """How the archiver flushes events to S3."""
    BATCHED = "batched"          # Buffer events, flush periodically (default)
    INDIVIDUAL = "individual"    # Flush each event immediately (expensive)
    DISABLED = "disabled"        # Archiver is off — no S3 writes


@dataclass(frozen=True)
class ArchiverConfig:
    """Archiver configuration.

    Controls how WAL events are archived to S3 for long-term durability.
    The archiver can be fully disabled if snapshots + Kafka retention
    provide sufficient recovery guarantees.

    Attributes:
        enabled: Whether archiver is enabled (master switch)
        flush_mode: How events are flushed to S3 (batched, individual, disabled)
        flush_interval_seconds: Seconds between periodic batch flushes
        max_segment_size_bytes: Max segment size before forced flush
        max_segment_events: Max events per segment before forced flush
        min_segment_events: Min events required to flush (avoids tiny segments)
        compression: Compression algorithm (gzip, none)
        s3_storage_class: S3 storage class for archive objects
        s3_upload_timeout_seconds: Timeout for S3 upload operations
        deduplicate: Whether to check for duplicate events before archiving
    """

    enabled: bool = True
    flush_mode: str = "batched"
    flush_interval_seconds: int = 60
    max_segment_size_bytes: int = 100 * 1024 * 1024  # 100MB
    max_segment_events: int = 10000
    min_segment_events: int = 1
    compression: str = "gzip"
    s3_storage_class: str = "STANDARD"
    s3_upload_timeout_seconds: int = 300
    deduplicate: bool = True

    @classmethod
    def from_env(cls) -> ArchiverConfig:
        enabled = os.getenv("ARCHIVER_ENABLED", "true").lower() == "true"
        flush_mode = os.getenv("ARCHIVE_FLUSH_MODE", "batched").lower()
        # If disabled via ARCHIVER_ENABLED=false, override flush_mode
        if not enabled:
            flush_mode = "disabled"
        return cls(
            enabled=enabled,
            flush_mode=flush_mode,
            flush_interval_seconds=int(os.getenv("ARCHIVE_FLUSH_SECONDS", "60")),
            max_segment_size_bytes=int(os.getenv("ARCHIVE_MAX_SEGMENT_BYTES", str(100 * 1024 * 1024))),
            max_segment_events=int(os.getenv("ARCHIVE_MAX_SEGMENT_EVENTS", "10000")),
            min_segment_events=int(os.getenv("ARCHIVE_MIN_SEGMENT_EVENTS", "1")),
            compression=os.getenv("ARCHIVE_COMPRESSION", "gzip"),
            s3_storage_class=os.getenv("ARCHIVE_S3_STORAGE_CLASS", "STANDARD"),
            s3_upload_timeout_seconds=int(os.getenv("ARCHIVE_S3_UPLOAD_TIMEOUT", "300")),
            deduplicate=os.getenv("ARCHIVE_DEDUPLICATE", "true").lower() == "true",
        )


@dataclass(frozen=True)
class RecoveryConfig:
    """Recovery strategy configuration.

    Controls the tiered recovery behavior when restoring a tenant database.
    Recovery tries each tier in order until the database is fully restored:
      1. Snapshot — restore latest SQLite backup from S3
      2. Kafka WAL — replay events from snapshot position (fast, fresh)
      3. S3 Archive — replay archived events (last resort)

    Attributes:
        kafka_replay_enabled: Whether to attempt Kafka WAL replay during recovery
        archive_replay_enabled: Whether to attempt S3 archive replay during recovery
        kafka_replay_timeout_seconds: Max seconds to wait for Kafka replay
        verify_after_recovery: Run SQLite integrity check after recovery
    """

    kafka_replay_enabled: bool = True
    archive_replay_enabled: bool = True
    kafka_replay_timeout_seconds: int = 300
    verify_after_recovery: bool = True

    @classmethod
    def from_env(cls) -> RecoveryConfig:
        return cls(
            kafka_replay_enabled=os.getenv("RECOVERY_KAFKA_REPLAY", "true").lower() == "true",
            archive_replay_enabled=os.getenv("RECOVERY_ARCHIVE_REPLAY", "true").lower() == "true",
            kafka_replay_timeout_seconds=int(os.getenv("RECOVERY_KAFKA_TIMEOUT", "300")),
            verify_after_recovery=os.getenv("RECOVERY_VERIFY", "true").lower() == "true",
        )


@dataclass(frozen=True)
class SnapshotConfig:
    """Snapshotter configuration.

    Attributes:
        enabled: Whether snapshotter is enabled
        interval_seconds: Interval between snapshots
        min_events_since_last: Minimum events since last snapshot to trigger new one
        compression: Compression algorithm (gzip, none)
        max_concurrent: Maximum concurrent snapshot uploads
    """

    enabled: bool = True
    interval_seconds: int = 3600  # 1 hour
    min_events_since_last: int = 1000
    compression: str = "gzip"
    max_concurrent: int = 4

    @classmethod
    def from_env(cls) -> SnapshotConfig:
        """Load configuration from environment variables."""
        return cls(
            enabled=os.getenv("SNAPSHOT_ENABLED", "true").lower() == "true",
            interval_seconds=int(os.getenv("SNAPSHOT_INTERVAL_SECONDS", "3600")),
            min_events_since_last=int(os.getenv("SNAPSHOT_MIN_EVENTS", "1000")),
            compression=os.getenv("SNAPSHOT_COMPRESSION", "gzip"),
            max_concurrent=int(os.getenv("SNAPSHOT_MAX_CONCURRENT", "4")),
        )


@dataclass(frozen=True)
class ObservabilityConfig:
    """Observability and logging configuration.

    Attributes:
        log_level: Logging level (DEBUG, INFO, WARNING, ERROR)
        log_format: Log format (json, text)
        metrics_enabled: Whether to expose Prometheus metrics
        metrics_port: Port for metrics endpoint
        trace_sampling_rate: OpenTelemetry trace sampling rate (0.0-1.0)
    """

    log_level: str = "INFO"
    log_format: str = "json"
    metrics_enabled: bool = True
    metrics_port: int = 9090
    trace_sampling_rate: float = 0.1

    @classmethod
    def from_env(cls) -> ObservabilityConfig:
        """Load configuration from environment variables."""
        return cls(
            log_level=os.getenv("LOG_LEVEL", "INFO"),
            log_format=os.getenv("LOG_FORMAT", "json"),
            metrics_enabled=os.getenv("METRICS_ENABLED", "true").lower() == "true",
            metrics_port=int(os.getenv("METRICS_PORT", "9090")),
            trace_sampling_rate=float(os.getenv("TRACE_SAMPLING_RATE", "0.1")),
        )


@dataclass
class ServerConfig:
    """Complete server configuration.

    This aggregates all configuration sections and provides validation.

    For HTTP/REST access, use the EntDB Console.
    See console/

    Attributes:
        wal_backend: Which WAL backend to use
        grpc: gRPC server configuration
        kafka: Kafka configuration (if wal_backend is KAFKA)
        kinesis: Kinesis configuration (if wal_backend is KINESIS)
        s3: S3 configuration
        storage: Local storage configuration
        applier: Applier configuration
        archiver: Archiver configuration
        snapshot: Snapshot configuration
        observability: Observability configuration
    """

    wal_backend: WalBackend = WalBackend.KAFKA
    schema_file: str | None = None
    grpc: GrpcConfig = field(default_factory=GrpcConfig)
    kafka: KafkaConfig = field(default_factory=KafkaConfig)
    kinesis: KinesisConfig = field(default_factory=KinesisConfig)
    pubsub: PubSubConfig = field(default_factory=PubSubConfig)
    sqs: SqsConfig = field(default_factory=SqsConfig)
    servicebus: ServiceBusConfig = field(default_factory=ServiceBusConfig)
    s3: S3Config = field(default_factory=S3Config)
    storage: StorageConfig = field(default_factory=StorageConfig)
    applier: ApplierConfig = field(default_factory=ApplierConfig)
    archiver: ArchiverConfig = field(default_factory=ArchiverConfig)
    recovery: RecoveryConfig = field(default_factory=RecoveryConfig)
    sharding: ShardingConfig = field(default_factory=ShardingConfig)
    snapshot: SnapshotConfig = field(default_factory=SnapshotConfig)
    observability: ObservabilityConfig = field(default_factory=ObservabilityConfig)

    @classmethod
    def from_env(cls) -> ServerConfig:
        """Load complete configuration from environment variables.

        Returns:
            ServerConfig with all sections populated from environment.

        Raises:
            ValueError: If required configuration is missing or invalid.
        """
        backend_str = os.getenv("WAL_BACKEND", "kafka").lower()
        try:
            wal_backend = WalBackend(backend_str)
        except ValueError:
            raise ValueError(
                f"Invalid WAL_BACKEND '{backend_str}'. "
                "Must be one of: kafka, kinesis, pubsub, sqs, servicebus, local"
            )

        config = cls(
            wal_backend=wal_backend,
            schema_file=os.getenv("SCHEMA_FILE"),
            grpc=GrpcConfig.from_env(),
            kafka=KafkaConfig.from_env(),
            kinesis=KinesisConfig.from_env(),
            pubsub=PubSubConfig.from_env(),
            sqs=SqsConfig.from_env(),
            servicebus=ServiceBusConfig.from_env(),
            s3=S3Config.from_env(),
            storage=StorageConfig.from_env(),
            applier=ApplierConfig.from_env(),
            archiver=ArchiverConfig.from_env(),
            recovery=RecoveryConfig.from_env(),
            sharding=ShardingConfig.from_env(),
            snapshot=SnapshotConfig.from_env(),
            observability=ObservabilityConfig.from_env(),
        )

        config.validate()
        return config

    def validate(self) -> None:
        """Validate configuration consistency.

        Raises:
            ValueError: If configuration is invalid.
        """
        # Validate WAL backend specific config
        if self.wal_backend == WalBackend.KAFKA:
            if not self.kafka.brokers:
                raise ValueError("KAFKA_BROKERS is required when WAL_BACKEND=kafka")
            if not self.kafka.topic:
                raise ValueError("KAFKA_TOPIC is required when WAL_BACKEND=kafka")
        elif self.wal_backend == WalBackend.KINESIS:
            if not self.kinesis.stream_name:
                raise ValueError("KINESIS_STREAM_NAME is required when WAL_BACKEND=kinesis")
        elif self.wal_backend == WalBackend.PUBSUB:
            if not self.pubsub.project_id:
                raise ValueError("PUBSUB_PROJECT_ID is required when WAL_BACKEND=pubsub")
        elif self.wal_backend == WalBackend.SQS:
            if not self.sqs.queue_url:
                raise ValueError("SQS_QUEUE_URL is required when WAL_BACKEND=sqs")
        elif self.wal_backend == WalBackend.SERVICEBUS:
            if not self.servicebus.connection_string:
                raise ValueError(
                    "SERVICEBUS_CONNECTION_STRING is required when WAL_BACKEND=servicebus"
                )

        # Validate S3 config if archiver or snapshotter enabled
        if self.archiver.enabled or self.snapshot.enabled:
            if not self.s3.bucket:
                raise ValueError("S3_BUCKET is required when archiver or snapshotter is enabled")

        # Warn about recovery configuration
        if self.recovery.archive_replay_enabled and not self.archiver.enabled:
            logger.warning(
                "Archive replay is enabled for recovery but archiver is disabled. "
                "Recovery will only find archives from previous archiver runs."
            )
        if not self.recovery.kafka_replay_enabled and not self.recovery.archive_replay_enabled:
            logger.warning(
                "Both Kafka and archive replay are disabled for recovery. "
                "Recovery can only restore from snapshots."
            )

        # Validate storage directory exists or can be created
        import os

        if not os.path.exists(self.storage.data_dir):
            logger.warning(
                f"Data directory does not exist: {self.storage.data_dir}. "
                "It will be created on first write."
            )

    def log_config(self) -> None:
        """Log configuration (redacting secrets)."""
        logger.info(
            "Server configuration loaded",
            extra={
                "wal_backend": self.wal_backend.value,
                "grpc_bind": self.grpc.bind_address,
                "kafka_brokers": self.kafka.brokers
                if self.wal_backend == WalBackend.KAFKA
                else None,
                "kafka_topic": self.kafka.topic if self.wal_backend == WalBackend.KAFKA else None,
                "kinesis_stream": self.kinesis.stream_name
                if self.wal_backend == WalBackend.KINESIS
                else None,
                "s3_bucket": self.s3.bucket,
                "data_dir": self.storage.data_dir,
                "archiver_enabled": self.archiver.enabled,
                "archiver_flush_mode": self.archiver.flush_mode,
                "archiver_flush_interval": self.archiver.flush_interval_seconds,
                "recovery_kafka_replay": self.recovery.kafka_replay_enabled,
                "recovery_archive_replay": self.recovery.archive_replay_enabled,
                "snapshot_enabled": self.snapshot.enabled,
                "node_id": self.sharding.node_id,
                "assigned_tenants": sorted(self.sharding.assigned_tenants) if self.sharding.assigned_tenants else "all",
                "log_level": self.observability.log_level,
            },
        )
