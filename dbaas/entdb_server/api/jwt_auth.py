"""
JWT / OIDC token validation for EntDB.

Supports Google, Microsoft, and generic OIDC providers.
Validates JWT tokens using JWKS (JSON Web Key Sets) fetched from the provider.

Usage:
    JWT_AUTH_ENABLED=true
    JWT_ISSUER=https://accounts.google.com
    JWT_AUDIENCE=your-client-id
    JWT_JWKS_URI=https://www.googleapis.com/oauth2/v3/certs

Client sends: metadata = [("authorization", "Bearer <jwt-token>")]

Invariants:
    - JWKS keys are cached with a configurable TTL
    - Tokens are validated for signature, expiry, issuer, and audience
    - Pre-configured providers available for Google and Microsoft

How to change safely:
    - Add new providers as module-level constants
    - Extend JwtConfig for new validation options
    - Keep JwtAuthenticator stateless except for JWKS cache
"""

from __future__ import annotations

import json
import logging
import time
import urllib.request
from dataclasses import dataclass, field

import jwt

logger = logging.getLogger(__name__)


class AuthError(Exception):
    """Raised when JWT authentication fails."""

    def __init__(self, message: str, code: str = "UNAUTHENTICATED") -> None:
        super().__init__(message)
        self.code = code


@dataclass
class JwtConfig:
    """Configuration for JWT/OIDC authentication.

    Attributes:
        enabled: Whether JWT authentication is enabled
        issuer: Expected token issuer (iss claim)
        audience: Expected token audience (aud claim)
        jwks_uri: URI to fetch JSON Web Key Set
        jwks_cache_ttl: Seconds to cache JWKS keys
        actor_claim: Claim to use for actor extraction (sub or email)
    """

    enabled: bool = False
    issuer: str = ""
    audience: str = ""
    jwks_uri: str = ""
    jwks_cache_ttl: int = 3600
    actor_claim: str = "sub"


# ---------------------------------------------------------------------------
# Pre-configured OIDC providers
# ---------------------------------------------------------------------------

GOOGLE_OIDC = JwtConfig(
    issuer="https://accounts.google.com",
    jwks_uri="https://www.googleapis.com/oauth2/v3/certs",
)

MICROSOFT_OIDC = JwtConfig(
    issuer="https://login.microsoftonline.com/common/v2.0",
    jwks_uri="https://login.microsoftonline.com/common/discovery/v2.0/keys",
)


@dataclass
class _JwksCache:
    """Internal JWKS cache entry."""

    keys: dict[str, dict] = field(default_factory=dict)
    fetched_at: float = 0.0


class JwtAuthenticator:
    """Validates JWT tokens from OIDC providers.

    Thread-safe: the JWKS cache is replaced atomically (dict assignment).
    """

    def __init__(self, config: JwtConfig) -> None:
        self._config = config
        self._cache = _JwksCache()

    @property
    def config(self) -> JwtConfig:
        return self._config

    # ------------------------------------------------------------------
    # Public API
    # ------------------------------------------------------------------

    async def validate_token(self, token: str) -> dict:
        """Validate a JWT token and return its claims.

        Steps:
            1. Decode header (unverified) to extract ``kid``
            2. Fetch JWKS from ``jwks_uri`` (cached)
            3. Find matching signing key by ``kid``
            4. Verify signature, expiry, issuer, audience
            5. Return decoded claims

        Args:
            token: The raw JWT string.

        Returns:
            Decoded claims dictionary.

        Raises:
            AuthError: If the token is invalid for any reason.
        """
        try:
            unverified_header = jwt.get_unverified_header(token)
        except jwt.DecodeError as exc:
            raise AuthError(f"Malformed JWT header: {exc}")

        kid = unverified_header.get("kid")
        if not kid:
            raise AuthError("JWT missing 'kid' header")

        algorithm = unverified_header.get("alg", "RS256")

        # Fetch (or use cached) JWKS
        jwks_keys = self._get_jwks()
        key_data = jwks_keys.get(kid)
        if key_data is None:
            # Key may have rotated — force refresh once
            jwks_keys = self._fetch_jwks()
            key_data = jwks_keys.get(kid)
            if key_data is None:
                raise AuthError(f"No JWKS key found for kid '{kid}'")

        try:
            public_key = jwt.algorithms.RSAAlgorithm.from_jwk(key_data)
        except Exception as exc:
            raise AuthError(f"Failed to construct public key from JWKS: {exc}")

        try:
            claims = jwt.decode(
                token,
                public_key,
                algorithms=[algorithm],
                issuer=self._config.issuer or None,
                audience=self._config.audience or None,
                options={
                    "verify_iss": bool(self._config.issuer),
                    "verify_aud": bool(self._config.audience),
                    "verify_exp": True,
                },
            )
        except jwt.ExpiredSignatureError:
            raise AuthError("Token has expired")
        except jwt.InvalidIssuerError:
            raise AuthError(f"Invalid issuer. Expected '{self._config.issuer}'")
        except jwt.InvalidAudienceError:
            raise AuthError(f"Invalid audience. Expected '{self._config.audience}'")
        except jwt.InvalidTokenError as exc:
            raise AuthError(f"Invalid token: {exc}")

        return claims

    def extract_actor(self, claims: dict) -> str:
        """Extract an actor identifier string from JWT claims.

        Returns ``"user:<value>"`` where ``<value>`` is taken from the
        configured ``actor_claim`` (defaults to ``sub``).  Falls back to
        ``email`` then ``sub`` if the preferred claim is missing.

        Args:
            claims: Decoded JWT claims dictionary.

        Returns:
            Actor string in the form ``"user:<identifier>"``.

        Raises:
            AuthError: If no suitable claim is found.
        """
        value = claims.get(self._config.actor_claim)
        if not value:
            value = claims.get("email") or claims.get("sub")
        if not value:
            raise AuthError("Cannot extract actor: no 'sub' or 'email' in claims")
        return f"user:{value}"

    # ------------------------------------------------------------------
    # JWKS helpers
    # ------------------------------------------------------------------

    def _get_jwks(self) -> dict[str, dict]:
        """Return cached JWKS keys, refreshing if TTL has elapsed."""
        now = time.monotonic()
        if self._cache.keys and (now - self._cache.fetched_at) < self._config.jwks_cache_ttl:
            return self._cache.keys
        return self._fetch_jwks()

    def _fetch_jwks(self) -> dict[str, dict]:
        """Fetch JWKS from the configured URI and update the cache.

        Uses ``urllib`` from the standard library so no extra HTTP
        dependency is required.

        Returns:
            Mapping of ``kid`` -> JWK dict.

        Raises:
            AuthError: If fetching or parsing fails.
        """
        uri = self._config.jwks_uri
        if not uri:
            raise AuthError("JWKS URI is not configured")

        try:
            req = urllib.request.Request(uri, headers={"Accept": "application/json"})
            with urllib.request.urlopen(req, timeout=10) as resp:
                data = json.loads(resp.read())
        except Exception as exc:
            raise AuthError(f"Failed to fetch JWKS from {uri}: {exc}")

        keys: dict[str, dict] = {}
        for key in data.get("keys", []):
            kid = key.get("kid")
            if kid:
                keys[kid] = key

        if not keys:
            raise AuthError(f"JWKS from {uri} contains no keys")

        self._cache = _JwksCache(keys=keys, fetched_at=time.monotonic())
        logger.info("Refreshed JWKS cache from %s (%d keys)", uri, len(keys))
        return keys
