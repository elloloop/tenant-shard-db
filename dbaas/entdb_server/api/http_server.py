"""
HTTP server implementation for EntDB.

This module provides an optional REST API that mirrors the gRPC interface.
It's useful for:
- Manual testing and debugging
- Clients that can't use gRPC
- Quick integration during development

Invariants:
    - HTTP endpoints have same semantics as gRPC
    - All operations require X-Tenant-ID and X-Actor headers
    - JSON request/response format

How to change safely:
    - Keep endpoints in sync with gRPC proto
    - Document all endpoints with OpenAPI
    - Version the API if breaking changes are needed
"""

from __future__ import annotations

import json
import logging
from collections.abc import Callable
from dataclasses import dataclass
from typing import Any

logger = logging.getLogger(__name__)

# Try to import aiohttp
try:
    from aiohttp import web

    AIOHTTP_AVAILABLE = True
except ImportError:
    AIOHTTP_AVAILABLE = False
    web = None


@dataclass
class HttpConfig:
    """HTTP server configuration."""

    host: str = "0.0.0.0"
    port: int = 8081
    cors_origins: tuple[str, ...] = ("*",)


def create_http_app(
    servicer: Any,
    config: HttpConfig | None = None,
) -> Any:
    """Create an HTTP application for EntDB.

    Args:
        servicer: EntDBServicer instance
        config: HTTP server configuration

    Returns:
        aiohttp Application instance

    Raises:
        ImportError: If aiohttp is not installed
    """
    if not AIOHTTP_AVAILABLE:
        raise ImportError("aiohttp is required for HTTP server. Install with: pip install aiohttp")

    config = config or HttpConfig()
    app = web.Application()

    # Add routes
    app.router.add_post("/v1/execute", lambda r: handle_execute(r, servicer))
    app.router.add_get(
        "/v1/receipt/{idempotency_key}", lambda r: handle_receipt_status(r, servicer)
    )
    app.router.add_get("/v1/nodes/{node_id}", lambda r: handle_get_node(r, servicer))
    app.router.add_get("/v1/nodes", lambda r: handle_query_nodes(r, servicer))
    app.router.add_get("/v1/nodes/{node_id}/edges/from", lambda r: handle_edges_from(r, servicer))
    app.router.add_get("/v1/nodes/{node_id}/edges/to", lambda r: handle_edges_to(r, servicer))
    app.router.add_get("/v1/mailbox/{user_id}/search", lambda r: handle_search_mailbox(r, servicer))
    app.router.add_get("/v1/mailbox/{user_id}", lambda r: handle_get_mailbox(r, servicer))
    app.router.add_get("/v1/health", lambda r: handle_health(r, servicer))
    app.router.add_get("/v1/schema", lambda r: handle_schema(r, servicer))

    # Add CORS middleware
    @web.middleware
    async def cors_middleware(request: web.Request, handler: Callable) -> web.Response:
        if request.method == "OPTIONS":
            response = web.Response()
        else:
            try:
                response = await handler(request)
            except web.HTTPException as e:
                response = e

        # Add CORS headers
        origin = request.headers.get("Origin", "*")
        if "*" in config.cors_origins or origin in config.cors_origins:
            response.headers["Access-Control-Allow-Origin"] = origin
        response.headers["Access-Control-Allow-Methods"] = "GET, POST, PUT, DELETE, OPTIONS"
        response.headers["Access-Control-Allow-Headers"] = (
            "Content-Type, X-Tenant-ID, X-Actor, X-Trace-ID"
        )

        return response

    app.middlewares.append(cors_middleware)

    # Add error handler
    @web.middleware
    async def error_middleware(request: web.Request, handler: Callable) -> web.Response:
        try:
            return await handler(request)
        except web.HTTPException:
            raise
        except Exception as e:
            logger.error(f"HTTP handler error: {e}", exc_info=True)
            return web.json_response(
                {"error": str(e), "error_code": "INTERNAL"},
                status=500,
            )

    app.middlewares.insert(0, error_middleware)

    return app


def extract_context(request: web.Request) -> tuple[str, str, str | None]:
    """Extract request context from headers.

    Args:
        request: HTTP request

    Returns:
        Tuple of (tenant_id, actor, trace_id)

    Raises:
        web.HTTPBadRequest: If required headers are missing
    """
    tenant_id = request.headers.get("X-Tenant-ID")
    actor = request.headers.get("X-Actor")
    trace_id = request.headers.get("X-Trace-ID")

    if not tenant_id:
        raise web.HTTPBadRequest(
            text=json.dumps({"error": "X-Tenant-ID header is required"}),
            content_type="application/json",
        )
    if not actor:
        raise web.HTTPBadRequest(
            text=json.dumps({"error": "X-Actor header is required"}),
            content_type="application/json",
        )

    return tenant_id, actor, trace_id


async def handle_execute(request: web.Request, servicer: Any) -> web.Response:
    """Handle POST /v1/execute - Execute atomic transaction."""
    tenant_id, actor, trace_id = extract_context(request)

    try:
        body = await request.json()
    except json.JSONDecodeError:
        raise web.HTTPBadRequest(
            text=json.dumps({"error": "Invalid JSON body"}),
            content_type="application/json",
        )

    operations = body.get("operations", [])
    if not operations:
        raise web.HTTPBadRequest(
            text=json.dumps({"error": "operations list is required"}),
            content_type="application/json",
        )

    result = await servicer.execute_atomic(
        tenant_id=tenant_id,
        actor=actor,
        operations=operations,
        idempotency_key=body.get("idempotency_key"),
        schema_fingerprint=body.get("schema_fingerprint"),
        wait_applied=body.get("wait_applied", False),
        wait_timeout_ms=body.get("wait_timeout_ms", 30000),
    )

    status = 200 if result.get("success") else 400
    return web.json_response(result, status=status)


async def handle_receipt_status(request: web.Request, servicer: Any) -> web.Response:
    """Handle GET /v1/receipt/{idempotency_key} - Get receipt status."""
    tenant_id, actor, _ = extract_context(request)
    idempotency_key = request.match_info["idempotency_key"]

    result = await servicer.get_receipt_status(tenant_id, idempotency_key)
    return web.json_response(result)


async def handle_get_node(request: web.Request, servicer: Any) -> web.Response:
    """Handle GET /v1/nodes/{node_id} - Get node by ID."""
    tenant_id, actor, _ = extract_context(request)
    node_id = request.match_info["node_id"]
    type_id = int(request.query.get("type_id", 0))

    result = await servicer.get_node(tenant_id, actor, type_id, node_id)

    if not result.get("found"):
        return web.json_response(result, status=404)
    return web.json_response(result)


async def handle_query_nodes(request: web.Request, servicer: Any) -> web.Response:
    """Handle GET /v1/nodes - Query nodes by type."""
    tenant_id, actor, _ = extract_context(request)
    type_id = int(request.query.get("type_id", 0))
    limit = int(request.query.get("limit", 100))
    offset = int(request.query.get("offset", 0))

    result = await servicer.query_nodes(tenant_id, actor, type_id, limit=limit, offset=offset)
    return web.json_response(result)


async def handle_edges_from(request: web.Request, servicer: Any) -> web.Response:
    """Handle GET /v1/nodes/{node_id}/edges/from - Get outgoing edges."""
    tenant_id, actor, _ = extract_context(request)
    node_id = request.match_info["node_id"]
    edge_type_id = request.query.get("edge_type_id")
    limit = int(request.query.get("limit", 100))

    result = await servicer.get_edges_from(
        tenant_id,
        actor,
        node_id,
        edge_type_id=int(edge_type_id) if edge_type_id else None,
        limit=limit,
    )
    return web.json_response(result)


async def handle_edges_to(request: web.Request, servicer: Any) -> web.Response:
    """Handle GET /v1/nodes/{node_id}/edges/to - Get incoming edges."""
    tenant_id, actor, _ = extract_context(request)
    node_id = request.match_info["node_id"]
    edge_type_id = request.query.get("edge_type_id")
    limit = int(request.query.get("limit", 100))

    result = await servicer.get_edges_to(
        tenant_id,
        actor,
        node_id,
        edge_type_id=int(edge_type_id) if edge_type_id else None,
        limit=limit,
    )
    return web.json_response(result)


async def handle_search_mailbox(request: web.Request, servicer: Any) -> web.Response:
    """Handle GET /v1/mailbox/{user_id}/search - Search mailbox."""
    tenant_id, actor, _ = extract_context(request)
    user_id = request.match_info["user_id"]
    query = request.query.get("q", "")
    limit = int(request.query.get("limit", 20))
    offset = int(request.query.get("offset", 0))

    source_type_ids = None
    if "source_type_ids" in request.query:
        source_type_ids = [int(x) for x in request.query["source_type_ids"].split(",")]

    result = await servicer.search_mailbox(
        tenant_id,
        actor,
        user_id,
        query,
        source_type_ids=source_type_ids,
        limit=limit,
        offset=offset,
    )
    return web.json_response(result)


async def handle_get_mailbox(request: web.Request, servicer: Any) -> web.Response:
    """Handle GET /v1/mailbox/{user_id} - Get mailbox items."""
    tenant_id, actor, _ = extract_context(request)
    user_id = request.match_info["user_id"]
    source_type_id = request.query.get("source_type_id")
    thread_id = request.query.get("thread_id")
    unread_only = request.query.get("unread_only", "false").lower() == "true"
    limit = int(request.query.get("limit", 50))
    offset = int(request.query.get("offset", 0))

    result = await servicer.get_mailbox(
        tenant_id,
        actor,
        user_id,
        source_type_id=int(source_type_id) if source_type_id else None,
        thread_id=thread_id,
        unread_only=unread_only,
        limit=limit,
        offset=offset,
    )
    return web.json_response(result)


async def handle_health(request: web.Request, servicer: Any) -> web.Response:
    """Handle GET /v1/health - Health check."""
    result = await servicer.health()
    status = 200 if result.get("healthy") else 503
    return web.json_response(result, status=status)


async def handle_schema(request: web.Request, servicer: Any) -> web.Response:
    """Handle GET /v1/schema - Get schema information."""
    type_id = request.query.get("type_id")

    result = await servicer.get_schema(type_id=int(type_id) if type_id else None)
    return web.json_response(result)


async def run_http_server(
    servicer: Any,
    host: str = "0.0.0.0",
    port: int = 8081,
) -> None:
    """Run the HTTP server.

    Args:
        servicer: EntDBServicer instance
        host: Host to bind to
        port: Port to listen on
    """
    if not AIOHTTP_AVAILABLE:
        logger.error("aiohttp not available, cannot start HTTP server")
        return

    config = HttpConfig(host=host, port=port)
    app = create_http_app(servicer, config)

    runner = web.AppRunner(app)
    await runner.setup()

    site = web.TCPSite(runner, host, port)
    await site.start()

    logger.info(f"HTTP server running on http://{host}:{port}")

    # Keep running until cancelled
    try:
        while True:
            await asyncio.sleep(3600)
    except asyncio.CancelledError:
        pass
    finally:
        await runner.cleanup()


# Import asyncio for the run function
import asyncio
