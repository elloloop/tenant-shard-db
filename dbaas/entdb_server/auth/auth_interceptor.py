"""
Unified gRPC authentication interceptor for EntDB.

Supports three authentication methods, checked in order:
    1. Bearer token (OAuth/OIDC JWT) via ``Authorization: Bearer <jwt>``
    2. API key via ``x-api-key`` header
    3. Session token via ``x-session-token`` header

Usage:
    interceptor = AuthInterceptor(
        oauth=OAuthValidator(...),
        api_keys=ApiKeyManager(...),
        sessions=SessionManager(...),
    )
    # Add to gRPC server interceptors list

Invariants:
    - At least one auth method must succeed or the request is rejected
    - Health check endpoints bypass authentication
    - AuthContext is populated on successful authentication
    - All errors return gRPC UNAUTHENTICATED status

How to change safely:
    - Add new auth methods by extending the authenticate() chain
    - Keep UNAUTHENTICATED_METHODS in sync with the server
    - Do not modify the AuthContext dataclass without updating consumers
"""

from __future__ import annotations

import contextvars
import logging
from collections.abc import Awaitable, Callable
from dataclasses import dataclass, field
from typing import Any

import grpc

from .api_key_manager import ApiKeyError, ApiKeyManager
from .oauth_validator import AuthenticationError, OAuthValidator
from .session_manager import SessionError, SessionManager

logger = logging.getLogger(__name__)


# ---------------------------------------------------------------------------
# Trusted identity propagation
#
# The interceptor publishes the verified caller identity into a ContextVar so
# downstream gRPC handlers can perform authorization based on the identity
# established by AuthInterceptor — *not* on the ``actor`` field of the
# untrusted client request payload. Reading ``request.actor`` for authz is a
# privilege escalation: a malicious client can authenticate as themselves and
# still claim ``actor: "system:admin"`` in the message body.
#
# Handlers should call :func:`get_current_identity` (or
# :func:`get_authoritative_actor`) to obtain the trusted caller id.
# ---------------------------------------------------------------------------

_current_identity: contextvars.ContextVar[str | None] = contextvars.ContextVar(
    "entdb_auth_identity", default=None
)


def get_current_identity() -> str | None:
    """Return the verified caller identity for the current request, or None.

    Populated by :class:`AuthInterceptor` after a successful
    ``authenticate()`` call. ``None`` outside of an authenticated request
    (e.g. unit tests that bypass the interceptor, or servers running with
    auth disabled).
    """
    return _current_identity.get()


def set_current_identity(identity: str | None) -> contextvars.Token:
    """Set the trusted identity for the current context.

    Returns a token suitable for :meth:`contextvars.ContextVar.reset`.
    Intended for use by the interceptor and by tests that need to simulate
    an authenticated request.
    """
    return _current_identity.set(identity)


def reset_current_identity(token: contextvars.Token) -> None:
    """Reset the trusted identity using a token from :func:`set_current_identity`."""
    _current_identity.reset(token)


def get_authoritative_actor(request_actor: str) -> str:
    """Return the actor string that should be used for authorization.

    If the interceptor has populated a trusted identity for this request,
    the trusted value (normalized to a ``user:<id>`` actor string when the
    raw identity does not already carry an actor prefix) is returned.
    Otherwise the caller's request payload is used as a fallback for
    no-auth deployments and for unit tests that bypass the interceptor.

    The trusted identity ALWAYS wins when present, even if ``request_actor``
    claims a different (or more privileged) actor — this is the fix for
    the privilege-escalation bug.
    """
    trusted = get_current_identity()
    if trusted is None:
        return request_actor
    if (
        trusted.startswith("user:")
        or trusted.startswith("system:")
        or trusted.startswith("admin:")
        or trusted == "__system__"
    ):
        return trusted
    return f"user:{trusted}"


@dataclass
class AuthContext:
    """Authentication context populated by the interceptor.

    Attributes:
        method: How the caller authenticated (``"oauth"``, ``"api_key"``,
            or ``"session"``).
        identity: A string identifying the caller (e.g. JWT ``sub``, key
            name, or user ID).
        scopes: List of granted scopes (from API key; empty for other
            methods).
        claims: Raw JWT claims (only for OAuth; empty dict otherwise).
        metadata: Extra metadata from the auth source.
    """

    method: str = ""
    identity: str = ""
    scopes: list[str] = field(default_factory=list)
    claims: dict[str, Any] = field(default_factory=dict)
    metadata: dict[str, Any] = field(default_factory=dict)


class AuthInterceptor(grpc.aio.ServerInterceptor):
    """Async gRPC interceptor that validates auth on every request.

    Inherits from :class:`grpc.aio.ServerInterceptor` so it can run natively
    on the async gRPC server without bridging sync/async per request.

    Supports three auth methods (checked in order):
        1. Bearer token (OAuth/OIDC JWT) -- ``Authorization: Bearer <jwt>``
        2. API key -- ``x-api-key`` header
        3. Session token -- ``x-session-token`` header

    Args:
        oauth: Optional OAuthValidator for JWT validation.
        api_keys: Optional ApiKeyManager for API key validation.
        sessions: Optional SessionManager for session token validation.
    """

    UNAUTHENTICATED_METHODS = frozenset(
        {
            "/entdb.EntDBService/Health",
            "/grpc.health.v1.Health/Check",
        }
    )

    def __init__(
        self,
        oauth: OAuthValidator | None = None,
        api_keys: ApiKeyManager | None = None,
        sessions: SessionManager | None = None,
    ) -> None:
        self._oauth = oauth
        self._api_keys = api_keys
        self._sessions = sessions

    async def intercept_service(
        self,
        continuation: Callable[[grpc.HandlerCallDetails], Awaitable[grpc.RpcMethodHandler | None]],
        handler_call_details: grpc.HandlerCallDetails,
    ) -> grpc.RpcMethodHandler | None:
        """Intercept incoming gRPC calls and validate authentication.

        This is a native async interceptor: no thread pool, no nested event
        loop. The request is authenticated via a direct ``await`` on
        :meth:`authenticate`.
        """
        method = handler_call_details.method
        if method in self.UNAUTHENTICATED_METHODS:
            return await continuation(handler_call_details)

        metadata = dict(handler_call_details.invocation_metadata or [])

        try:
            auth_ctx = await self.authenticate(metadata)
        except (AuthenticationError, ApiKeyError, SessionError) as exc:
            logger.warning("Auth failed for %s: %s", method, exc)
            return _create_abort_handler(
                grpc.StatusCode.UNAUTHENTICATED,
                str(exc),
            )

        if auth_ctx is None:
            return _create_abort_handler(
                grpc.StatusCode.UNAUTHENTICATED,
                "No valid authentication credentials provided.",
            )

        logger.debug(
            "Authenticated via %s: %s",
            auth_ctx.method,
            auth_ctx.identity,
        )

        # Publish the verified identity into the request-scoped ContextVar
        # so downstream handlers can authorize on a trusted value rather
        # than the untrusted ``request.actor`` payload field. The token is
        # always reset after the handler returns to keep requests isolated.
        token = _current_identity.set(auth_ctx.identity)
        try:
            return await continuation(handler_call_details)
        finally:
            _current_identity.reset(token)

    async def authenticate(self, metadata: dict) -> AuthContext | None:
        """Authenticate a request from its gRPC metadata.

        Tries each configured auth method in order:
            1. Bearer token (Authorization header with JWT)
            2. API key (x-api-key header)
            3. Session token (x-session-token header)

        Args:
            metadata: Dict of gRPC metadata key-value pairs.

        Returns:
            An AuthContext on success, or None if no credentials are
            present.

        Raises:
            AuthenticationError: If OAuth token is invalid.
            ApiKeyError: If API key is invalid.
            SessionError: If session token is invalid.
        """
        # 1. Bearer token (OAuth/OIDC)
        auth_header = metadata.get("authorization", "")
        if auth_header:
            bearer_value = auth_header[7:] if auth_header.startswith("Bearer ") else auth_header
            if bearer_value and self._oauth and self._is_jwt(bearer_value):
                claims = await self._oauth.validate_token(bearer_value)
                identity = claims.get("sub", claims.get("email", "unknown"))
                return AuthContext(
                    method="oauth",
                    identity=identity,
                    claims=claims,
                )

        # 2. API key
        api_key = metadata.get("x-api-key", "")
        if api_key and self._api_keys:
            key_info = await self._api_keys.validate_key(api_key)
            return AuthContext(
                method="api_key",
                identity=key_info["name"],
                scopes=key_info["scopes"],
                metadata={"key_id": key_info["key_id"]},
            )

        # 3. Session token
        session_token = metadata.get("x-session-token", "")
        if session_token and self._sessions:
            session_data = await self._sessions.validate_session(session_token)
            return AuthContext(
                method="session",
                identity=session_data["user_id"],
                metadata=session_data.get("metadata", {}),
            )

        return None

    @staticmethod
    def _is_jwt(value: str) -> bool:
        """Heuristic: JWTs have exactly two dots (header.payload.signature)."""
        return value.count(".") == 2


def _create_abort_handler(code: grpc.StatusCode, message: str) -> grpc.RpcMethodHandler:
    """Create a handler that immediately aborts with the given status.

    The abort callable is ``async`` because this interceptor runs inside an
    async gRPC server (``grpc.aio``).
    """

    async def _abort_unary_unary(request, context):
        await context.abort(code, message)

    return grpc.unary_unary_rpc_method_handler(_abort_unary_unary)
