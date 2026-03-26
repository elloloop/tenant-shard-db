"""
gRPC authentication interceptor.

Supports API key authentication via metadata header.
Disabled when AUTH_API_KEYS is empty (backward compatible).

Usage:
    AUTH_ENABLED=true
    AUTH_API_KEYS=key1,key2,key3

Client sends: metadata = [("authorization", "Bearer key1")]
"""

from __future__ import annotations

import logging
from collections.abc import Callable

import grpc

logger = logging.getLogger(__name__)


class ApiKeyInterceptor(grpc.ServerInterceptor):
    """Validates API key from authorization metadata header."""

    # RPCs that don't require authentication
    UNAUTHENTICATED_METHODS = frozenset(
        {
            "/entdb.EntDBService/Health",
            "/grpc.health.v1.Health/Check",
        }
    )

    def __init__(self, valid_keys: frozenset[str]) -> None:
        self._valid_keys = valid_keys

    def intercept_service(
        self,
        continuation: Callable,
        handler_call_details: grpc.HandlerCallDetails,
    ) -> grpc.RpcMethodHandler | None:
        # Skip auth for health checks
        method = handler_call_details.method
        if method in self.UNAUTHENTICATED_METHODS:
            return continuation(handler_call_details)

        # Extract API key from metadata
        metadata = dict(handler_call_details.invocation_metadata or [])
        auth_header = metadata.get("authorization", "")

        # Support "Bearer <key>" and raw "<key>"
        api_key = auth_header[7:] if auth_header.startswith("Bearer ") else auth_header

        if not api_key or api_key not in self._valid_keys:
            # Return an abort handler
            return _create_abort_handler(
                grpc.StatusCode.UNAUTHENTICATED,
                "Invalid or missing API key. Set 'authorization: Bearer <key>' metadata.",
            )

        return continuation(handler_call_details)


def _create_abort_handler(code: grpc.StatusCode, message: str) -> grpc.RpcMethodHandler:
    """Create a handler that immediately aborts with the given status."""

    def _abort_unary_unary(request, context):
        context.abort(code, message)

    return grpc.unary_unary_rpc_method_handler(_abort_unary_unary)
