"""
SDK client manager for the gateway.

Manages a pool of SDK clients for connecting to EntDB.
"""

import asyncio
import logging
from contextlib import asynccontextmanager
from typing import AsyncGenerator

from entdb_sdk import DbClient

from .config import Settings

logger = logging.getLogger(__name__)


class SdkClientManager:
    """Manages SDK client connections to EntDB."""

    def __init__(self, settings: Settings):
        self.settings = settings
        self._clients: asyncio.Queue[DbClient] = asyncio.Queue()
        self._active_clients: list[DbClient] = []
        self._started = False

    async def start(self) -> None:
        """Initialize connection pool."""
        if self._started:
            return

        logger.info(
            f"Initializing SDK client pool (size={self.settings.max_connections}) "
            f"to {self.settings.entdb_endpoint}"
        )

        for _ in range(self.settings.max_connections):
            client = DbClient(
                endpoint=self.settings.entdb_endpoint,
                tenant_id=self.settings.default_tenant_id,
                actor="gateway:system",
            )
            await client.connect()
            self._active_clients.append(client)
            await self._clients.put(client)

        self._started = True
        logger.info("SDK client pool initialized")

    async def stop(self) -> None:
        """Close all connections."""
        if not self._started:
            return

        logger.info("Closing SDK client pool")
        for client in self._active_clients:
            await client.close()

        self._active_clients.clear()
        self._started = False

    @asynccontextmanager
    async def acquire(
        self,
        tenant_id: str | None = None,
        actor: str | None = None,
    ) -> AsyncGenerator[DbClient, None]:
        """
        Acquire a client from the pool.

        Args:
            tenant_id: Override tenant ID for this request
            actor: Override actor for this request

        Yields:
            Configured DbClient
        """
        client = await asyncio.wait_for(
            self._clients.get(),
            timeout=self.settings.connection_timeout,
        )

        try:
            # Create a request-scoped client with overrides
            if tenant_id or actor:
                # Return a wrapper that uses specific tenant/actor
                request_client = _RequestScopedClient(
                    client,
                    tenant_id or self.settings.default_tenant_id,
                    actor or "gateway:anonymous",
                )
                yield request_client
            else:
                yield client
        finally:
            await self._clients.put(client)


class _RequestScopedClient:
    """
    Wrapper that overrides tenant_id and actor for a request.

    This allows reusing pooled connections with different request contexts.
    """

    def __init__(self, client: DbClient, tenant_id: str, actor: str):
        self._client = client
        self.tenant_id = tenant_id
        self.actor = actor

    async def get(self, node_id: str):
        """Get a node by ID."""
        return await self._client.get(
            node_id,
            tenant_id=self.tenant_id,
            actor=self.actor,
        )

    async def query(self, type_id: int, **kwargs):
        """Query nodes by type."""
        return await self._client.query(
            type_id=type_id,
            tenant_id=self.tenant_id,
            actor=self.actor,
            **kwargs,
        )

    async def edges_out(self, node_id: str, edge_type_id: int | None = None):
        """Get outgoing edges from a node."""
        return await self._client.edges_out(
            node_id,
            edge_type_id=edge_type_id,
            tenant_id=self.tenant_id,
            actor=self.actor,
        )

    async def edges_in(self, node_id: str, edge_type_id: int | None = None):
        """Get incoming edges to a node."""
        return await self._client.edges_in(
            node_id,
            edge_type_id=edge_type_id,
            tenant_id=self.tenant_id,
            actor=self.actor,
        )

    async def search(self, query: str, **kwargs):
        """Search mailbox."""
        return await self._client.search(
            query,
            tenant_id=self.tenant_id,
            actor=self.actor,
            **kwargs,
        )

    async def atomic(self, build_fn):
        """Execute atomic transaction."""
        return await self._client.atomic(
            build_fn,
            tenant_id=self.tenant_id,
            actor=self.actor,
        )

    async def get_schema(self):
        """Get schema information."""
        return await self._client.get_schema()
