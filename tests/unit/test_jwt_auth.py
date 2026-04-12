"""Unit tests for JWT/OIDC authentication."""

from __future__ import annotations

import json
import time
from unittest.mock import MagicMock, patch

import jwt as pyjwt
import pytest
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import rsa

from dbaas.entdb_server.api.auth import (
    AuthInterceptor,
    _looks_like_jwt,
)
from dbaas.entdb_server.api.jwt_auth import (
    GOOGLE_OIDC,
    MICROSOFT_OIDC,
    AuthError,
    JwtAuthenticator,
    JwtConfig,
)


# ---------------------------------------------------------------------------
# Helpers: self-signed RSA key pair and mock JWKS
# ---------------------------------------------------------------------------

def _generate_rsa_keypair():
    """Generate an RSA private key and return (private_key, public_key) objects."""
    private_key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    return private_key, private_key.public_key()


def _private_key_pem(private_key) -> bytes:
    return private_key.private_bytes(
        encoding=serialization.Encoding.PEM,
        format=serialization.PrivateFormat.PKCS8,
        encryption_algorithm=serialization.NoEncryption(),
    )


def _build_jwks(public_key, kid: str = "test-kid-1") -> dict:
    """Build a JWKS dict from a public key."""
    jwk = json.loads(pyjwt.algorithms.RSAAlgorithm.to_jwk(public_key))
    jwk["kid"] = kid
    jwk["use"] = "sig"
    jwk["alg"] = "RS256"
    return {"keys": [jwk]}


def _encode_token(
    private_key,
    kid: str = "test-kid-1",
    issuer: str = "https://test.example.com",
    audience: str = "test-audience",
    subject: str = "user-123",
    email: str = "user@example.com",
    exp_offset: int = 3600,
    extra_claims: dict | None = None,
) -> str:
    """Encode a JWT token with the given parameters."""
    now = int(time.time())
    payload = {
        "iss": issuer,
        "aud": audience,
        "sub": subject,
        "email": email,
        "name": "Test User",
        "iat": now,
        "exp": now + exp_offset,
    }
    if extra_claims:
        payload.update(extra_claims)
    return pyjwt.encode(
        payload,
        _private_key_pem(private_key),
        algorithm="RS256",
        headers={"kid": kid},
    )


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------

@pytest.fixture()
def rsa_keys():
    """Provide a fresh RSA key pair."""
    return _generate_rsa_keypair()


@pytest.fixture()
def jwt_config():
    return JwtConfig(
        enabled=True,
        issuer="https://test.example.com",
        audience="test-audience",
        jwks_uri="https://test.example.com/.well-known/jwks.json",
        jwks_cache_ttl=300,
    )


@pytest.fixture()
def authenticator(jwt_config):
    return JwtAuthenticator(jwt_config)


# ---------------------------------------------------------------------------
# Tests: JwtConfig and pre-configured providers
# ---------------------------------------------------------------------------

@pytest.mark.unit
class TestJwtConfig:
    def test_defaults(self):
        cfg = JwtConfig()
        assert cfg.enabled is False
        assert cfg.issuer == ""
        assert cfg.audience == ""
        assert cfg.jwks_uri == ""
        assert cfg.jwks_cache_ttl == 3600
        assert cfg.actor_claim == "sub"

    def test_google_oidc_preset(self):
        assert GOOGLE_OIDC.issuer == "https://accounts.google.com"
        assert "googleapis.com" in GOOGLE_OIDC.jwks_uri

    def test_microsoft_oidc_preset(self):
        assert "microsoftonline.com" in MICROSOFT_OIDC.issuer
        assert "microsoftonline.com" in MICROSOFT_OIDC.jwks_uri


# ---------------------------------------------------------------------------
# Tests: JwtAuthenticator.validate_token
# ---------------------------------------------------------------------------

@pytest.mark.unit
class TestValidateToken:
    @pytest.mark.asyncio
    async def test_valid_token_returns_claims(self, authenticator, rsa_keys):
        priv, pub = rsa_keys
        jwks = _build_jwks(pub)
        token = _encode_token(priv)

        with patch.object(authenticator, "_fetch_jwks", return_value=jwks["keys"][0] and {k["kid"]: k for k in jwks["keys"]}):
            # Seed the cache
            authenticator._cache.keys = {k["kid"]: k for k in jwks["keys"]}
            authenticator._cache.fetched_at = time.monotonic()

            claims = await authenticator.validate_token(token)

        assert claims["sub"] == "user-123"
        assert claims["email"] == "user@example.com"
        assert claims["iss"] == "https://test.example.com"

    @pytest.mark.asyncio
    async def test_expired_token_rejected(self, authenticator, rsa_keys):
        priv, pub = rsa_keys
        jwks = _build_jwks(pub)
        token = _encode_token(priv, exp_offset=-3600)  # Expired 1h ago

        authenticator._cache.keys = {k["kid"]: k for k in jwks["keys"]}
        authenticator._cache.fetched_at = time.monotonic()

        with pytest.raises(AuthError, match="expired"):
            await authenticator.validate_token(token)

    @pytest.mark.asyncio
    async def test_wrong_issuer_rejected(self, authenticator, rsa_keys):
        priv, pub = rsa_keys
        jwks = _build_jwks(pub)
        token = _encode_token(priv, issuer="https://evil.example.com")

        authenticator._cache.keys = {k["kid"]: k for k in jwks["keys"]}
        authenticator._cache.fetched_at = time.monotonic()

        with pytest.raises(AuthError, match="[Ii]ssuer"):
            await authenticator.validate_token(token)

    @pytest.mark.asyncio
    async def test_wrong_audience_rejected(self, authenticator, rsa_keys):
        priv, pub = rsa_keys
        jwks = _build_jwks(pub)
        token = _encode_token(priv, audience="wrong-audience")

        authenticator._cache.keys = {k["kid"]: k for k in jwks["keys"]}
        authenticator._cache.fetched_at = time.monotonic()

        with pytest.raises(AuthError, match="[Aa]udience"):
            await authenticator.validate_token(token)

    @pytest.mark.asyncio
    async def test_wrong_signature_rejected(self, authenticator, rsa_keys):
        """Token signed with a different key should be rejected."""
        _, pub = rsa_keys
        other_priv, _ = _generate_rsa_keypair()
        jwks = _build_jwks(pub)
        token = _encode_token(other_priv)  # Signed with wrong key

        authenticator._cache.keys = {k["kid"]: k for k in jwks["keys"]}
        authenticator._cache.fetched_at = time.monotonic()

        with pytest.raises(AuthError, match="[Ii]nvalid"):
            await authenticator.validate_token(token)

    @pytest.mark.asyncio
    async def test_missing_kid_rejected(self, authenticator, rsa_keys):
        """Token without a kid header should be rejected."""
        priv, _ = rsa_keys
        now = int(time.time())
        token = pyjwt.encode(
            {"sub": "x", "iss": "https://test.example.com", "aud": "test-audience",
             "exp": now + 3600, "iat": now},
            _private_key_pem(priv),
            algorithm="RS256",
            # No kid in headers
        )

        with pytest.raises(AuthError, match="kid"):
            await authenticator.validate_token(token)

    @pytest.mark.asyncio
    async def test_unknown_kid_triggers_refetch(self, authenticator, rsa_keys):
        """If kid is not in cache, authenticator should refetch JWKS once."""
        priv, pub = rsa_keys
        jwks = _build_jwks(pub, kid="new-kid")
        token = _encode_token(priv, kid="new-kid")

        # Start with empty cache
        authenticator._cache.keys = {}
        authenticator._cache.fetched_at = time.monotonic()

        fetched_keys = {k["kid"]: k for k in jwks["keys"]}
        with patch.object(authenticator, "_fetch_jwks", return_value=fetched_keys) as mock_fetch:
            claims = await authenticator.validate_token(token)

        mock_fetch.assert_called_once()
        assert claims["sub"] == "user-123"

    @pytest.mark.asyncio
    async def test_malformed_token_rejected(self, authenticator):
        with pytest.raises(AuthError, match="[Mm]alformed"):
            await authenticator.validate_token("not-a-jwt")

    @pytest.mark.asyncio
    async def test_unknown_kid_not_in_refetch_rejected(self, authenticator, rsa_keys):
        """If kid is not found even after refetch, raise AuthError."""
        priv, pub = rsa_keys
        token = _encode_token(priv, kid="unknown-kid")

        authenticator._cache.keys = {}
        authenticator._cache.fetched_at = time.monotonic()

        with patch.object(authenticator, "_fetch_jwks", return_value={}):
            with pytest.raises(AuthError, match="No JWKS key found"):
                await authenticator.validate_token(token)


# ---------------------------------------------------------------------------
# Tests: JWKS cache behavior
# ---------------------------------------------------------------------------

@pytest.mark.unit
class TestJwksCache:
    def test_cache_hit_does_not_refetch(self, authenticator, rsa_keys):
        """Within TTL, _get_jwks should return cached keys without calling _fetch_jwks."""
        _, pub = rsa_keys
        jwks = _build_jwks(pub)
        cached = {k["kid"]: k for k in jwks["keys"]}
        authenticator._cache.keys = cached
        authenticator._cache.fetched_at = time.monotonic()

        with patch.object(authenticator, "_fetch_jwks") as mock_fetch:
            result = authenticator._get_jwks()

        mock_fetch.assert_not_called()
        assert result == cached

    def test_cache_expired_triggers_refetch(self, authenticator, rsa_keys):
        """After TTL, _get_jwks should refetch."""
        _, pub = rsa_keys
        jwks = _build_jwks(pub)
        cached = {k["kid"]: k for k in jwks["keys"]}
        authenticator._cache.keys = cached
        authenticator._cache.fetched_at = time.monotonic() - 9999  # Way past TTL

        with patch.object(authenticator, "_fetch_jwks", return_value=cached) as mock_fetch:
            authenticator._get_jwks()

        mock_fetch.assert_called_once()

    def test_empty_cache_triggers_fetch(self, authenticator):
        """Empty cache should trigger a fetch."""
        with patch.object(authenticator, "_fetch_jwks", return_value={"k": {}}) as mock_fetch:
            authenticator._get_jwks()
        mock_fetch.assert_called_once()


# ---------------------------------------------------------------------------
# Tests: extract_actor
# ---------------------------------------------------------------------------

@pytest.mark.unit
class TestExtractActor:
    def test_actor_from_sub(self, authenticator):
        claims = {"sub": "user-123", "email": "user@example.com"}
        assert authenticator.extract_actor(claims) == "user:user-123"

    def test_actor_from_email_config(self, jwt_config):
        jwt_config.actor_claim = "email"
        auth = JwtAuthenticator(jwt_config)
        claims = {"sub": "user-123", "email": "user@example.com"}
        assert auth.extract_actor(claims) == "user:user@example.com"

    def test_actor_fallback_to_email(self, authenticator):
        claims = {"email": "user@example.com"}  # No sub
        assert authenticator.extract_actor(claims) == "user:user@example.com"

    def test_actor_fallback_to_sub(self, jwt_config):
        jwt_config.actor_claim = "email"
        auth = JwtAuthenticator(jwt_config)
        claims = {"sub": "user-123"}  # No email
        assert auth.extract_actor(claims) == "user:user-123"

    def test_actor_missing_all_claims_raises(self, authenticator):
        with pytest.raises(AuthError, match="Cannot extract actor"):
            authenticator.extract_actor({})


# ---------------------------------------------------------------------------
# Tests: AuthInterceptor (unified interceptor)
# ---------------------------------------------------------------------------

@pytest.mark.unit
class TestAuthInterceptor:
    def test_health_check_bypasses_auth(self):
        interceptor = AuthInterceptor(frozenset({"key-1"}), jwt_authenticator=None)
        continuation = MagicMock(return_value="handler")
        details = MagicMock()
        details.method = "/entdb.EntDBService/Health"
        details.invocation_metadata = []
        result = interceptor.intercept_service(continuation, details)
        continuation.assert_called_once()
        assert result == "handler"

    def test_missing_auth_header_rejected(self):
        interceptor = AuthInterceptor(frozenset({"key-1"}))
        continuation = MagicMock()
        details = MagicMock()
        details.method = "/entdb.EntDBService/ExecuteAtomic"
        details.invocation_metadata = []
        result = interceptor.intercept_service(continuation, details)
        continuation.assert_not_called()
        assert result is not None

    def test_valid_api_key_passes(self):
        interceptor = AuthInterceptor(frozenset({"key-1", "key-2"}))
        continuation = MagicMock(return_value="handler")
        details = MagicMock()
        details.method = "/entdb.EntDBService/ExecuteAtomic"
        details.invocation_metadata = [("authorization", "Bearer key-1")]
        result = interceptor.intercept_service(continuation, details)
        continuation.assert_called_once()
        assert result == "handler"

    def test_invalid_api_key_rejected(self):
        interceptor = AuthInterceptor(frozenset({"key-1"}))
        continuation = MagicMock()
        details = MagicMock()
        details.method = "/entdb.EntDBService/ExecuteAtomic"
        details.invocation_metadata = [("authorization", "Bearer wrong-key")]
        result = interceptor.intercept_service(continuation, details)
        continuation.assert_not_called()
        assert result is not None

    def test_jwt_token_routed_to_jwt_authenticator(self):
        """A Bearer value with two dots should be routed to JwtAuthenticator."""
        mock_jwt_auth = MagicMock(spec=JwtAuthenticator)
        mock_jwt_auth.config = JwtConfig(enabled=True)

        interceptor = AuthInterceptor(frozenset(), jwt_authenticator=mock_jwt_auth)
        continuation = MagicMock(return_value="handler")
        details = MagicMock()
        details.method = "/entdb.EntDBService/ExecuteAtomic"
        # Use a realistic looking JWT (three segments)
        details.invocation_metadata = [("authorization", "Bearer header.payload.signature")]

        # Patch _handle_jwt to just call continuation
        with patch.object(interceptor, "_handle_jwt", return_value="handler") as mock_handle:
            result = interceptor.intercept_service(continuation, details)

        mock_handle.assert_called_once_with("header.payload.signature", continuation, details)
        assert result == "handler"

    def test_api_key_not_routed_to_jwt(self):
        """An API key (no dots) should NOT be routed to JWT even if JWT is configured."""
        mock_jwt_auth = MagicMock(spec=JwtAuthenticator)
        interceptor = AuthInterceptor(frozenset({"my-api-key"}), jwt_authenticator=mock_jwt_auth)
        continuation = MagicMock(return_value="handler")
        details = MagicMock()
        details.method = "/entdb.EntDBService/ExecuteAtomic"
        details.invocation_metadata = [("authorization", "Bearer my-api-key")]

        result = interceptor.intercept_service(continuation, details)
        continuation.assert_called_once()
        assert result == "handler"

    def test_empty_bearer_rejected(self):
        interceptor = AuthInterceptor(frozenset({"key-1"}))
        continuation = MagicMock()
        details = MagicMock()
        details.method = "/entdb.EntDBService/ExecuteAtomic"
        details.invocation_metadata = [("authorization", "Bearer ")]
        result = interceptor.intercept_service(continuation, details)
        continuation.assert_not_called()
        assert result is not None


# ---------------------------------------------------------------------------
# Tests: _looks_like_jwt helper
# ---------------------------------------------------------------------------

@pytest.mark.unit
class TestLooksLikeJwt:
    def test_jwt_with_three_segments(self):
        assert _looks_like_jwt("eyJ.eyJ.sig") is True

    def test_api_key_no_dots(self):
        assert _looks_like_jwt("my-api-key-123") is False

    def test_single_dot(self):
        assert _looks_like_jwt("part1.part2") is False

    def test_three_dots(self):
        assert _looks_like_jwt("a.b.c.d") is False
