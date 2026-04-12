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
    ) -> None:
        self.issuer_url = issuer_url.rstrip("/")
        self.audience = audience
        self._jwks_cache_ttl = jwks_cache_ttl
        self._jwks_cache: dict[str, dict[str, Any]] = {}
        self._jwks_fetched_at: float = 0.0

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
            # Cache miss — force refetch once
            await self._fetch_jwks()
            key_data = self._get_cached_key(kid)
            if key_data is None:
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

    async def _fetch_jwks(self) -> dict[str, dict[str, Any]]:
        """Fetch JWKS from ``{issuer_url}/.well-known/jwks.json`` with caching.

        Uses ``urllib`` from the standard library so no extra HTTP dependency
        is required at import time.

        Returns:
            Mapping of ``kid`` -> JWK dict.

        Raises:
            AuthenticationError: If fetching or parsing fails.
        """
        uri = f"{self.issuer_url}/.well-known/jwks.json"
        try:
            req = urllib.request.Request(uri, headers={"Accept": "application/json"})
            with urllib.request.urlopen(req, timeout=10) as resp:  # noqa: S310
                data = json.loads(resp.read())
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
