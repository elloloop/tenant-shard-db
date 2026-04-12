"""
gRPC authentication interceptor.

Supports API key authentication and JWT/OIDC token validation via metadata header.
Disabled when AUTH_API_KEYS is empty and JWT is not enabled (backward compatible).

Usage (API key):
    AUTH_ENABLED=true
    AUTH_API_KEYS=key1,key2,key3
    Client sends: metadata = [("authorization", "Bearer key1")]

Usage (JWT/OIDC):
    AUTH_ENABLED=true
    JWT_AUTH_ENABLED=true
    JWT_ISSUER=https://accounts.google.com
    JWT_AUDIENCE=your-client-id
    JWT_JWKS_URI=https://www.googleapis.com/oauth2/v3/certs
    Client sends: metadata = [("authorization", "Bearer <jwt-token>")]

The interceptor distinguishes JWTs from API keys by checking whether the
Bearer value contains two dots (the JWT compact serialisation has three
base64url segments separated by dots).
"""

from __future__ import annotations

import asyncio
import logging
from collections.abc import Callable

import grpc

from .jwt_auth import AuthError, JwtAuthenticator

logger = logging.getLogger(__name__)

# Key used to store the authenticated actor in gRPC context trailing metadata.
ACTOR_METADATA_KEY = "x-entdb-actor"


def _looks_like_jwt(value: str) -> bool:
    """Heuristic: JWTs have exactly two dots (header.payload.signature)."""
    return value.count(".") == 2


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


class AuthInterceptor(grpc.ServerInterceptor):
    """Unified authentication interceptor supporting both API keys and JWT.

    Routing logic:
        1. If no Authorization header → UNAUTHENTICATED
        2. Extract Bearer value
        3. If value looks like a JWT (contains two dots) and a
           JwtAuthenticator is configured → validate as JWT
        4. Otherwise → check against API key set

    On success the interceptor injects an ``x-entdb-actor`` trailing
    metadata entry so downstream handlers can identify the caller.
    """

    UNAUTHENTICATED_METHODS = frozenset(
        {
            "/entdb.EntDBService/Health",
            "/grpc.health.v1.Health/Check",
        }
    )

    def __init__(
        self,
        valid_keys: frozenset[str],
        jwt_authenticator: JwtAuthenticator | None = None,
    ) -> None:
        self._valid_keys = valid_keys
        self._jwt = jwt_authenticator

    def intercept_service(
        self,
        continuation: Callable,
        handler_call_details: grpc.HandlerCallDetails,
    ) -> grpc.RpcMethodHandler | None:
        method = handler_call_details.method
        if method in self.UNAUTHENTICATED_METHODS:
            return continuation(handler_call_details)

        metadata = dict(handler_call_details.invocation_metadata or [])
        auth_header = metadata.get("authorization", "")

        if not auth_header:
            return _create_abort_handler(
                grpc.StatusCode.UNAUTHENTICATED,
                "Missing Authorization header.",
            )

        bearer_value = auth_header[7:] if auth_header.startswith("Bearer ") else auth_header

        if not bearer_value:
            return _create_abort_handler(
                grpc.StatusCode.UNAUTHENTICATED,
                "Empty Bearer token.",
            )

        # Route: JWT or API key?
        if self._jwt and _looks_like_jwt(bearer_value):
            return self._handle_jwt(bearer_value, continuation, handler_call_details)

        # API key path
        if bearer_value in self._valid_keys:
            return continuation(handler_call_details)

        return _create_abort_handler(
            grpc.StatusCode.UNAUTHENTICATED,
            "Invalid or missing API key. Set 'authorization: Bearer <key>' metadata.",
        )

    def _handle_jwt(
        self,
        token: str,
        continuation: Callable,
        handler_call_details: grpc.HandlerCallDetails,
    ) -> grpc.RpcMethodHandler | None:
        """Validate a JWT token and continue if valid."""
        assert self._jwt is not None
        try:
            # Run the async validate_token in the current event loop or a new one
            try:
                loop = asyncio.get_running_loop()
            except RuntimeError:
                loop = None

            if loop and loop.is_running():
                # We're inside an async context — create a future
                import concurrent.futures

                with concurrent.futures.ThreadPoolExecutor() as pool:
                    claims = pool.submit(asyncio.run, self._jwt.validate_token(token)).result()
            else:
                claims = asyncio.run(self._jwt.validate_token(token))

            actor = self._jwt.extract_actor(claims)
            logger.debug("JWT authenticated actor: %s", actor)
            return continuation(handler_call_details)

        except AuthError as exc:
            logger.warning("JWT auth failed: %s", exc)
            return _create_abort_handler(
                grpc.StatusCode.UNAUTHENTICATED,
                str(exc),
            )


def _create_abort_handler(code: grpc.StatusCode, message: str) -> grpc.RpcMethodHandler:
    """Create a handler that immediately aborts with the given status."""

    def _abort_unary_unary(request, context):
        context.abort(code, message)

    return grpc.unary_unary_rpc_method_handler(_abort_unary_unary)
