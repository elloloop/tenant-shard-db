"""
Session management with TTL and token revocation for EntDB.

Provides in-memory session storage with automatic TTL eviction and
per-user session limits.

Usage:
    sm = SessionManager(session_ttl=3600, max_sessions_per_user=10)
    token = await sm.create_session("user-123", {"ip": "1.2.3.4"})
    data = await sm.validate_session(token)
    await sm.revoke_session(token)

Invariants:
    - Session tokens are cryptographically random (32-byte hex)
    - Expired sessions are lazily evicted on access
    - Per-user session count is enforced (oldest evicted on overflow)
    - Revoked sessions cannot be validated

How to change safely:
    - Replace the in-memory dict with Redis for production
    - Keep the public API surface identical
    - Eviction logic is in _evict_expired and _enforce_user_limit
"""

from __future__ import annotations

import logging
import secrets
import time
from typing import Any

logger = logging.getLogger(__name__)


class SessionError(Exception):
    """Raised on session operation failures."""


class SessionManager:
    """Manage user sessions with TTL and limits.

    Uses an in-memory dict for storage.  Each session is keyed by a
    cryptographically random token and stores the user ID, creation time,
    expiration time, and optional metadata.

    Args:
        session_ttl: Session time-to-live in seconds (default 3600).
        max_sessions_per_user: Maximum concurrent sessions per user
            (default 10).  When exceeded, the oldest session is evicted.
    """

    def __init__(
        self,
        *,
        session_ttl: int = 3600,
        max_sessions_per_user: int = 10,
    ) -> None:
        self._session_ttl = session_ttl
        self._max_sessions_per_user = max_sessions_per_user
        # token -> session data
        self._sessions: dict[str, dict[str, Any]] = {}
        # user_id -> list of tokens (ordered by creation time)
        self._user_sessions: dict[str, list[str]] = {}

    # ------------------------------------------------------------------
    # Public API
    # ------------------------------------------------------------------

    async def create_session(
        self,
        user_id: str,
        metadata: dict | None = None,
    ) -> str:
        """Create a session and return the session token.

        If the user already has ``max_sessions_per_user`` active sessions,
        the oldest is automatically evicted.

        Args:
            user_id: The user for whom to create the session.
            metadata: Optional dict of session metadata (e.g. IP, user
                agent).

        Returns:
            A cryptographically random session token string.
        """
        self._evict_expired()

        token = secrets.token_hex(32)
        now = time.time()

        self._sessions[token] = {
            "user_id": user_id,
            "created_at": now,
            "expires_at": now + self._session_ttl,
            "metadata": metadata or {},
        }

        # Track per-user sessions
        user_tokens = self._user_sessions.setdefault(user_id, [])
        user_tokens.append(token)

        # Enforce per-user limit
        self._enforce_user_limit(user_id)

        logger.debug("Created session for user %s", user_id)
        return token

    async def validate_session(self, token: str) -> dict:
        """Validate a session token and return session data.

        Args:
            token: The session token to validate.

        Returns:
            Dict with ``user_id``, ``created_at``, ``expires_at``, and
            ``metadata``.

        Raises:
            SessionError: If the token is invalid or the session has
                expired.
        """
        session = self._sessions.get(token)
        if session is None:
            raise SessionError("Invalid session token")

        if time.time() >= session["expires_at"]:
            # Lazily clean up
            self._remove_session(token)
            raise SessionError("Session has expired")

        return {
            "user_id": session["user_id"],
            "created_at": session["created_at"],
            "expires_at": session["expires_at"],
            "metadata": session["metadata"],
        }

    async def revoke_session(self, token: str) -> None:
        """Revoke a specific session.

        Args:
            token: The session token to revoke.

        Raises:
            SessionError: If the token is not found.
        """
        if token not in self._sessions:
            raise SessionError("Session not found")

        self._remove_session(token)
        logger.debug("Revoked session token")

    async def revoke_all_sessions(self, user_id: str) -> int:
        """Revoke all sessions for a user.

        Args:
            user_id: The user whose sessions should be revoked.

        Returns:
            Number of sessions revoked.
        """
        tokens = self._user_sessions.pop(user_id, [])
        count = 0
        for token in tokens:
            if token in self._sessions:
                del self._sessions[token]
                count += 1

        if count:
            logger.info("Revoked %d sessions for user %s", count, user_id)
        return count

    # ------------------------------------------------------------------
    # Internal helpers
    # ------------------------------------------------------------------

    def _remove_session(self, token: str) -> None:
        """Remove a session from both the session store and user index."""
        session = self._sessions.pop(token, None)
        if session is not None:
            user_id = session["user_id"]
            user_tokens = self._user_sessions.get(user_id, [])
            if token in user_tokens:
                user_tokens.remove(token)
                if not user_tokens:
                    self._user_sessions.pop(user_id, None)

    def _evict_expired(self) -> None:
        """Remove all expired sessions (lazy sweep)."""
        now = time.time()
        expired = [t for t, s in self._sessions.items() if now >= s["expires_at"]]
        for token in expired:
            self._remove_session(token)

    def _enforce_user_limit(self, user_id: str) -> None:
        """Evict oldest sessions if user exceeds max_sessions_per_user."""
        user_tokens = self._user_sessions.get(user_id, [])
        while len(user_tokens) > self._max_sessions_per_user:
            oldest_token = user_tokens[0]
            self._remove_session(oldest_token)
