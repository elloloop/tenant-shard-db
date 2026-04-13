"""
OAuth 2.0 / OIDC token validation for EntDB.

Validates JWT tokens from any OIDC-compliant provider using JWKS (JSON Web
Key Sets) fetched from the provider's discovery endpoint.

Usage:
    validator = OAuthValidator(
        issuer_url="https://accounts.google.com",
        audience="your-client-id",
    )
    claims = await validator.validate_token(token)

Invariants:
    - JWKS keys are cached with a configurable TTL
    - Tokens are validated for signature, expiry, issuer, and audience
    - RS256 and ES256 algorithms are supported
    - On cache miss for a kid, a single refetch is attempted before failing

How to change safely:
    - Add new algorithms to SUPPORTED_ALGORITHMS
    - Extend claim validation in validate_token
    - Keep _fetch_jwks idempotent
"""

from __future__ import annotations

import asyncio
import json
import logging
import time
import urllib.request
from typing import Any

import jwt
from jwt import algorithms as jwt_algorithms

logger = logging.getLogger(__name__)

SUPPORTED_ALGORITHMS = ("RS256", "ES256")


class AuthenticationError(Exception):
    """Raised when token validation fails."""

    def __init__(self, message: str, code: str = "UNAUTHENTICATED") -> None:
        super().__init__(message)
        self.code = code


class OAuthValidator:
    """Validate JWT tokens from an OIDC provider.

    Fetches the provider's JWKS from ``{issuer_url}/.well-known/jwks.json``
    and caches the keys for ``jwks_cache_ttl`` seconds.

    Attributes:
        issuer_url: The expected token issuer (``iss`` claim).
        audience: The expected token audience (``aud`` claim).
    """

    def __init__(
        self,
        issuer_url: str,
        audience: str,
        *,
        jwks_cache_ttl: int = 3600,
        unknown_kid_negative_ttl: float = 60.0,
    ) -> None:
        self.issuer_url = issuer_url.rstrip("/")
        self.audience = audience
        self._jwks_cache_ttl = jwks_cache_ttl
        self._unknown_kid_negative_ttl = unknown_kid_negative_ttl
        self._jwks_cache: dict[str, dict[str, Any]] = {}
        self._jwks_fetched_at: float = 0.0
        # Dedup lock so concurrent cache misses don't trigger N
        # simultaneous JWKS refetches — exactly one fetch in flight
        # at a time, everyone else waits on its result.
        self._fetch_lock: asyncio.Lock | None = None
        # Negative cache for unknown kids: attacker-supplied random
        # kids blow up to a cache miss + full JWKS fetch on every
        # request without this. Short TTL so legitimate key rotations
        # still work.
        self._unknown_kid_cache: dict[str, float] = {}

    # ------------------------------------------------------------------
    # Public API
    # ------------------------------------------------------------------

    async def validate_token(self, token: str) -> dict:
        """Validate and decode a JWT token.

        Steps:
            1. Decode the header (unverified) to extract ``kid`` and ``alg``
            2. Look up the signing key from cached JWKS (refetch once on miss)
            3. Verify signature, expiry, issuer, and audience claims
            4. Return the decoded claims dict

        Args:
            token: The raw JWT string (compact serialization).

        Returns:
            Decoded claims dictionary.

        Raises:
            AuthenticationError: On invalid, expired, or otherwise
                unacceptable tokens.
        """
        # 1. Decode header
        try:
            unverified_header = jwt.get_unverified_header(token)
        except jwt.DecodeError as exc:
            raise AuthenticationError(f"Malformed JWT header: {exc}")

        kid = unverified_header.get("kid")
        if not kid:
            raise AuthenticationError("JWT missing 'kid' header")

        alg = unverified_header.get("alg", "RS256")
        if alg not in SUPPORTED_ALGORITHMS:
            raise AuthenticationError(
                f"Unsupported algorithm '{alg}'. Supported: {SUPPORTED_ALGORITHMS}"
            )

        # 2. Resolve signing key
        key_data = self._get_cached_key(kid)
        if key_data is None:
            # Negative cache: reject known-unknown kids without
            # triggering another JWKS fetch. Mitigates cache-miss
            # amplification attacks.
            if self._is_unknown_kid(kid):
                raise AuthenticationError(f"No JWKS key found for kid '{kid}'")
            # Cache miss — force a single deduped refetch. Concurrent
            # callers for the same kid wait on one in-flight fetch
            # instead of each firing their own network call.
            await self._fetch_jwks_deduped()
            key_data = self._get_cached_key(kid)
            if key_data is None:
                self._remember_unknown_kid(kid)
                raise AuthenticationError(f"No JWKS key found for kid '{kid}'")

        public_key = self._construct_public_key(key_data, alg)

        # 3. Decode and verify
        try:
            claims = jwt.decode(
                token,
                public_key,
                algorithms=[alg],
                issuer=self.issuer_url,
                audience=self.audience,
                options={
                    "verify_exp": True,
                    "verify_iat": True,
                    "verify_iss": True,
                    "verify_aud": True,
                },
            )
        except jwt.ExpiredSignatureError:
            raise AuthenticationError("Token has expired")
        except jwt.InvalidIssuerError:
            raise AuthenticationError(f"Invalid issuer. Expected '{self.issuer_url}'")
        except jwt.InvalidAudienceError:
            raise AuthenticationError(f"Invalid audience. Expected '{self.audience}'")
        except jwt.InvalidTokenError as exc:
            raise AuthenticationError(f"Invalid token: {exc}")

        return claims

    # ------------------------------------------------------------------
    # JWKS management
    # ------------------------------------------------------------------

    def _is_unknown_kid(self, kid: str) -> bool:
        """Return ``True`` if ``kid`` is in the negative cache and still fresh."""
        expires_at = self._unknown_kid_cache.get(kid)
        if expires_at is None:
            return False
        if time.time() >= expires_at:
            # Expired: evict and miss so we refetch.
            self._unknown_kid_cache.pop(kid, None)
            return False
        return True

    def _remember_unknown_kid(self, kid: str) -> None:
        """Remember that ``kid`` was not found in JWKS for a short TTL."""
        if self._unknown_kid_negative_ttl <= 0:
            return
        self._unknown_kid_cache[kid] = time.time() + self._unknown_kid_negative_ttl
        # Keep the negative cache bounded. 1024 entries cap is more
        # than enough; any attacker hitting this limit is already
        # failing the first-layer rate limit.
        if len(self._unknown_kid_cache) > 1024:
            # Drop the oldest ~25% by expiry time.
            ordered = sorted(self._unknown_kid_cache.items(), key=lambda kv: kv[1])
            keep_from = len(ordered) // 4
            self._unknown_kid_cache = dict(ordered[keep_from:])

    async def _fetch_jwks_deduped(self) -> dict[str, dict[str, Any]]:
        """Run one JWKS fetch at a time across concurrent callers.

        Without this, 1000 concurrent requests with fresh random
        ``kid``s each trigger 1000 blocking HTTP round-trips, which is
        both a DoS vector against the issuer and a thread-pool burner
        on the EntDB server. The lock collapses them to a single
        in-flight fetch whose result is shared.
        """
        if self._fetch_lock is None:
            # Lazily created so the validator works in environments
            # where no event loop exists at construction time (tests).
            self._fetch_lock = asyncio.Lock()
        async with self._fetch_lock:
            # A previous waiter may already have refreshed the cache.
            # Check before re-fetching.
            if time.time() - self._jwks_fetched_at < 1.0:
                return self._jwks_cache
            return await self._fetch_jwks()

    async def _fetch_jwks(self) -> dict[str, dict[str, Any]]:
        """Fetch JWKS from ``{issuer_url}/.well-known/jwks.json`` with caching.

        Uses ``urllib`` from the standard library so no extra HTTP dependency
        is required at import time. The blocking ``urlopen`` call is
        dispatched to the default executor so it does not starve the
        asyncio event loop — without that, a burst of unknown-kid
        requests is a one-shot DoS against the entire server.

        Returns:
            Mapping of ``kid`` -> JWK dict.

        Raises:
            AuthenticationError: If fetching or parsing fails.
        """
        uri = f"{self.issuer_url}/.well-known/jwks.json"

        def _do_fetch() -> dict:
            req = urllib.request.Request(uri, headers={"Accept": "application/json"})
            with urllib.request.urlopen(req, timeout=10) as resp:  # noqa: S310
                return json.loads(resp.read())

        try:
            loop = asyncio.get_running_loop()
            data = await loop.run_in_executor(None, _do_fetch)
        except RuntimeError:
            # No running loop (sync test path) — call directly.
            data = _do_fetch()
        except Exception as exc:
            raise AuthenticationError(f"Failed to fetch JWKS from {uri}: {exc}")

        keys: dict[str, dict[str, Any]] = {}
        for key in data.get("keys", []):
            key_kid = key.get("kid")
            if key_kid:
                keys[key_kid] = key

        if not keys:
            raise AuthenticationError(f"JWKS from {uri} contains no keys")

        self._jwks_cache = keys
        self._jwks_fetched_at = time.monotonic()
        logger.info("Refreshed JWKS cache from %s (%d keys)", uri, len(keys))
        return keys

    def _get_cached_key(self, kid: str) -> dict[str, Any] | None:
        """Return a cached JWK for the given kid, or None if stale / missing."""
        now = time.monotonic()
        if self._jwks_cache and (now - self._jwks_fetched_at) < self._jwks_cache_ttl:
            return self._jwks_cache.get(kid)
        return None

    @staticmethod
    def _construct_public_key(key_data: dict[str, Any], alg: str) -> Any:
        """Construct a public key object from a JWK dict."""
        try:
            if alg == "RS256":
                return jwt_algorithms.RSAAlgorithm.from_jwk(key_data)
            elif alg == "ES256":
                return jwt_algorithms.ECAlgorithm.from_jwk(key_data)
            else:
                raise AuthenticationError(f"Unsupported algorithm: {alg}")
        except AuthenticationError:
            raise
        except Exception as exc:
            raise AuthenticationError(f"Failed to construct public key from JWKS: {exc}")
