"""Unit tests for the enhanced authentication system (issues #86, #87, #88).

Covers:
    - OAuthValidator: JWT validation, JWKS caching, algorithm support
    - ApiKeyManager: create, validate, rotate, revoke, scopes
    - SessionManager: create, validate, TTL, revoke, max sessions
    - AuthInterceptor: all three auth paths + rejection
"""

from __future__ import annotations

import hashlib
import json
import time
from unittest.mock import AsyncMock, MagicMock, patch

import jwt as pyjwt
import pytest
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import ec, rsa

from dbaas.entdb_server.auth.api_key_manager import VALID_SCOPES, ApiKeyError, ApiKeyManager
from dbaas.entdb_server.auth.auth_interceptor import AuthContext, AuthInterceptor
from dbaas.entdb_server.auth.oauth_validator import AuthenticationError, OAuthValidator
from dbaas.entdb_server.auth.session_manager import SessionError, SessionManager

# ---------------------------------------------------------------------------
# Helpers: RSA and EC key pairs, mock JWKS, JWT encoding
# ---------------------------------------------------------------------------


def _generate_rsa_keypair():
    private_key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    return private_key, private_key.public_key()


def _generate_ec_keypair():
    private_key = ec.generate_private_key(ec.SECP256R1())
    return private_key, private_key.public_key()


def _private_key_pem(private_key) -> bytes:
    return private_key.private_bytes(
        encoding=serialization.Encoding.PEM,
        format=serialization.PrivateFormat.PKCS8,
        encryption_algorithm=serialization.NoEncryption(),
    )


def _build_rsa_jwks(public_key, kid: str = "test-kid-1") -> dict:
    jwk = json.loads(pyjwt.algorithms.RSAAlgorithm.to_jwk(public_key))
    jwk["kid"] = kid
    jwk["use"] = "sig"
    jwk["alg"] = "RS256"
    return {"keys": [jwk]}


def _build_ec_jwks(public_key, kid: str = "ec-kid-1") -> dict:
    jwk = json.loads(pyjwt.algorithms.ECAlgorithm.to_jwk(public_key))
    jwk["kid"] = kid
    jwk["use"] = "sig"
    jwk["alg"] = "ES256"
    return {"keys": [jwk]}


def _encode_rsa_token(
    private_key,
    kid: str = "test-kid-1",
    issuer: str = "https://test.example.com",
    audience: str = "test-audience",
    subject: str = "user-123",
    exp_offset: int = 3600,
    extra_claims: dict | None = None,
) -> str:
    now = int(time.time())
    payload = {
        "iss": issuer,
        "aud": audience,
        "sub": subject,
        "email": "user@example.com",
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


def _encode_ec_token(
    private_key,
    kid: str = "ec-kid-1",
    issuer: str = "https://test.example.com",
    audience: str = "test-audience",
    subject: str = "user-456",
    exp_offset: int = 3600,
) -> str:
    now = int(time.time())
    payload = {
        "iss": issuer,
        "aud": audience,
        "sub": subject,
        "iat": now,
        "exp": now + exp_offset,
    }
    return pyjwt.encode(
        payload,
        _private_key_pem(private_key),
        algorithm="ES256",
        headers={"kid": kid},
    )


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------


@pytest.fixture()
def rsa_keys():
    return _generate_rsa_keypair()


@pytest.fixture()
def ec_keys():
    return _generate_ec_keypair()


@pytest.fixture()
def oauth_validator():
    return OAuthValidator(
        issuer_url="https://test.example.com",
        audience="test-audience",
        jwks_cache_ttl=300,
    )


@pytest.fixture()
def api_key_manager():
    return ApiKeyManager()


@pytest.fixture()
def session_manager():
    return SessionManager(session_ttl=3600, max_sessions_per_user=3)


# ===========================================================================
# OAuthValidator tests (#86)
# ===========================================================================


@pytest.mark.unit
class TestOAuthValidatorValidToken:
    """Test that a valid RS256 JWT decodes correctly with mocked JWKS."""

    @pytest.mark.asyncio
    async def test_valid_rsa_token(self, oauth_validator, rsa_keys):
        priv, pub = rsa_keys
        jwks = _build_rsa_jwks(pub)
        token = _encode_rsa_token(priv)

        # Seed the cache directly
        oauth_validator._jwks_cache = {k["kid"]: k for k in jwks["keys"]}
        oauth_validator._jwks_fetched_at = time.monotonic()

        claims = await oauth_validator.validate_token(token)
        assert claims["sub"] == "user-123"
        assert claims["iss"] == "https://test.example.com"
        assert claims["aud"] == "test-audience"


@pytest.mark.unit
class TestOAuthValidatorExpired:
    """Expired tokens must be rejected."""

    @pytest.mark.asyncio
    async def test_expired_token_rejected(self, oauth_validator, rsa_keys):
        priv, pub = rsa_keys
        jwks = _build_rsa_jwks(pub)
        token = _encode_rsa_token(priv, exp_offset=-3600)

        oauth_validator._jwks_cache = {k["kid"]: k for k in jwks["keys"]}
        oauth_validator._jwks_fetched_at = time.monotonic()

        with pytest.raises(AuthenticationError, match="expired"):
            await oauth_validator.validate_token(token)


@pytest.mark.unit
class TestOAuthValidatorWrongAudience:
    """Tokens with the wrong audience must be rejected."""

    @pytest.mark.asyncio
    async def test_wrong_audience_rejected(self, oauth_validator, rsa_keys):
        priv, pub = rsa_keys
        jwks = _build_rsa_jwks(pub)
        token = _encode_rsa_token(priv, audience="wrong-audience")

        oauth_validator._jwks_cache = {k["kid"]: k for k in jwks["keys"]}
        oauth_validator._jwks_fetched_at = time.monotonic()

        with pytest.raises(AuthenticationError, match="[Aa]udience"):
            await oauth_validator.validate_token(token)


@pytest.mark.unit
class TestOAuthValidatorJwksCaching:
    """JWKS caching respects TTL and avoids unnecessary refetches."""

    @pytest.mark.asyncio
    async def test_cache_hit_no_refetch(self, oauth_validator, rsa_keys):
        priv, pub = rsa_keys
        jwks = _build_rsa_jwks(pub)
        token = _encode_rsa_token(priv)

        oauth_validator._jwks_cache = {k["kid"]: k for k in jwks["keys"]}
        oauth_validator._jwks_fetched_at = time.monotonic()

        with patch.object(oauth_validator, "_fetch_jwks") as mock_fetch:
            claims = await oauth_validator.validate_token(token)

        mock_fetch.assert_not_called()
        assert claims["sub"] == "user-123"

    @pytest.mark.asyncio
    async def test_cache_expired_triggers_refetch(self, oauth_validator, rsa_keys):
        priv, pub = rsa_keys
        jwks = _build_rsa_jwks(pub)
        token = _encode_rsa_token(priv)

        # Expired cache
        oauth_validator._jwks_cache = {k["kid"]: k for k in jwks["keys"]}
        oauth_validator._jwks_fetched_at = time.monotonic() - 9999

        async def mock_fetch():
            keys = {k["kid"]: k for k in jwks["keys"]}
            oauth_validator._jwks_cache = keys
            oauth_validator._jwks_fetched_at = time.monotonic()
            return keys

        with patch.object(oauth_validator, "_fetch_jwks", side_effect=mock_fetch) as mf:
            claims = await oauth_validator.validate_token(token)

        mf.assert_called_once()
        assert claims["sub"] == "user-123"


@pytest.mark.unit
class TestOAuthValidatorES256:
    """ES256 (ECDSA) tokens should also validate correctly."""

    @pytest.mark.asyncio
    async def test_valid_ec_token(self, oauth_validator, ec_keys):
        priv, pub = ec_keys
        jwks = _build_ec_jwks(pub)
        token = _encode_ec_token(priv)

        oauth_validator._jwks_cache = {k["kid"]: k for k in jwks["keys"]}
        oauth_validator._jwks_fetched_at = time.monotonic()

        claims = await oauth_validator.validate_token(token)
        assert claims["sub"] == "user-456"


@pytest.mark.unit
class TestOAuthValidatorMalformed:
    """Malformed tokens must be rejected."""

    @pytest.mark.asyncio
    async def test_malformed_token(self, oauth_validator):
        with pytest.raises(AuthenticationError, match="[Mm]alformed"):
            await oauth_validator.validate_token("not-a-jwt")

    @pytest.mark.asyncio
    async def test_missing_kid(self, oauth_validator, rsa_keys):
        priv, _ = rsa_keys
        now = int(time.time())
        token = pyjwt.encode(
            {
                "sub": "x",
                "iss": "https://test.example.com",
                "aud": "test-audience",
                "exp": now + 3600,
                "iat": now,
            },
            _private_key_pem(priv),
            algorithm="RS256",
        )
        with pytest.raises(AuthenticationError, match="kid"):
            await oauth_validator.validate_token(token)


# ===========================================================================
# ApiKeyManager tests (#87)
# ===========================================================================


@pytest.mark.unit
class TestApiKeyCreateAndValidate:
    """Create + validate round-trip."""

    @pytest.mark.asyncio
    async def test_create_and_validate(self, api_key_manager):
        result = await api_key_manager.create_key("test-key", ["read", "write"])
        assert "key_id" in result
        assert "key_secret" in result
        assert result["scopes"] == ["read", "write"]

        info = await api_key_manager.validate_key(result["key_secret"])
        assert info["key_id"] == result["key_id"]
        assert info["name"] == "test-key"
        assert info["scopes"] == ["read", "write"]


@pytest.mark.unit
class TestApiKeyScopesEnforced:
    """Invalid scopes are rejected at creation time."""

    @pytest.mark.asyncio
    async def test_invalid_scope_rejected(self, api_key_manager):
        with pytest.raises(ApiKeyError, match="Invalid scopes"):
            await api_key_manager.create_key("bad-key", ["read", "superadmin"])

    @pytest.mark.asyncio
    async def test_all_valid_scopes_accepted(self, api_key_manager):
        result = await api_key_manager.create_key("full-key", list(VALID_SCOPES))
        assert set(result["scopes"]) == VALID_SCOPES


@pytest.mark.unit
class TestApiKeyRotation:
    """Rotation generates a new secret and invalidates the old one."""

    @pytest.mark.asyncio
    async def test_rotation_invalidates_old(self, api_key_manager):
        original = await api_key_manager.create_key("rot-key", ["read"])
        old_secret = original["key_secret"]

        rotated = await api_key_manager.rotate_key(original["key_id"])
        new_secret = rotated["key_secret"]

        assert new_secret != old_secret
        assert rotated["rotated_at"] is not None

        # New secret works
        info = await api_key_manager.validate_key(new_secret)
        assert info["key_id"] == original["key_id"]

        # Old secret fails
        with pytest.raises(ApiKeyError, match="Invalid"):
            await api_key_manager.validate_key(old_secret)


@pytest.mark.unit
class TestApiKeyRevocation:
    """Revoked keys are permanently rejected."""

    @pytest.mark.asyncio
    async def test_revoked_key_rejected(self, api_key_manager):
        result = await api_key_manager.create_key("revoke-me", ["read"])
        await api_key_manager.revoke_key(result["key_id"])

        with pytest.raises(ApiKeyError):
            await api_key_manager.validate_key(result["key_secret"])

    @pytest.mark.asyncio
    async def test_rotate_revoked_key_rejected(self, api_key_manager):
        result = await api_key_manager.create_key("revoke-then-rotate", ["read"])
        await api_key_manager.revoke_key(result["key_id"])

        with pytest.raises(ApiKeyError, match="revoked"):
            await api_key_manager.rotate_key(result["key_id"])


@pytest.mark.unit
class TestApiKeyListKeys:
    """list_keys returns all keys without exposing secrets."""

    @pytest.mark.asyncio
    async def test_list_keys_no_secrets(self, api_key_manager):
        await api_key_manager.create_key("k1", ["read"])
        await api_key_manager.create_key("k2", ["write"])

        keys = await api_key_manager.list_keys()
        assert len(keys) == 2
        for k in keys:
            assert "key_secret" not in k
            assert "secret_hash" not in k
            assert "key_id" in k


@pytest.mark.unit
class TestApiKeyExpiration:
    """Expired keys are rejected on validation."""

    @pytest.mark.asyncio
    async def test_expired_key_rejected(self, api_key_manager):
        result = await api_key_manager.create_key(
            "expiring-key", ["read"], expires_at=int(time.time()) - 1
        )
        with pytest.raises(ApiKeyError, match="expired"):
            await api_key_manager.validate_key(result["key_secret"])


@pytest.mark.unit
class TestApiKeySecretHashing:
    """Secrets are stored as SHA-256 hashes, never plaintext."""

    @pytest.mark.asyncio
    async def test_secret_stored_as_hash(self, api_key_manager):
        result = await api_key_manager.create_key("hash-test", ["read"])
        key_id = result["key_id"]
        entry = api_key_manager._keys[key_id]

        expected_hash = hashlib.sha256(result["key_secret"].encode()).hexdigest()
        assert entry["secret_hash"] == expected_hash
        # Plaintext secret is NOT stored
        assert result["key_secret"] not in str(entry)


# ===========================================================================
# SessionManager tests (#88)
# ===========================================================================


@pytest.mark.unit
class TestSessionCreateAndValidate:
    """Create + validate round-trip."""

    @pytest.mark.asyncio
    async def test_create_and_validate(self, session_manager):
        token = await session_manager.create_session("user-1", {"ip": "1.2.3.4"})
        assert isinstance(token, str)
        assert len(token) == 64  # 32-byte hex

        data = await session_manager.validate_session(token)
        assert data["user_id"] == "user-1"
        assert data["metadata"]["ip"] == "1.2.3.4"


@pytest.mark.unit
class TestSessionTTLExpiry:
    """Expired sessions are rejected."""

    @pytest.mark.asyncio
    async def test_expired_session_rejected(self):
        sm = SessionManager(session_ttl=0, max_sessions_per_user=10)
        token = await sm.create_session("user-1")

        # Force expiration by manipulating the session
        sm._sessions[token]["expires_at"] = time.time() - 1

        with pytest.raises(SessionError, match="expired"):
            await sm.validate_session(token)


@pytest.mark.unit
class TestSessionRevokeSingle:
    """Revoking a single session invalidates only that token."""

    @pytest.mark.asyncio
    async def test_revoke_single(self, session_manager):
        t1 = await session_manager.create_session("user-1")
        t2 = await session_manager.create_session("user-1")

        await session_manager.revoke_session(t1)

        with pytest.raises(SessionError):
            await session_manager.validate_session(t1)

        # t2 still valid
        data = await session_manager.validate_session(t2)
        assert data["user_id"] == "user-1"


@pytest.mark.unit
class TestSessionRevokeAll:
    """Revoking all sessions for a user invalidates all tokens."""

    @pytest.mark.asyncio
    async def test_revoke_all(self, session_manager):
        t1 = await session_manager.create_session("user-1")
        t2 = await session_manager.create_session("user-1")
        t3 = await session_manager.create_session("user-2")

        count = await session_manager.revoke_all_sessions("user-1")
        assert count == 2

        with pytest.raises(SessionError):
            await session_manager.validate_session(t1)
        with pytest.raises(SessionError):
            await session_manager.validate_session(t2)

        # user-2 unaffected
        data = await session_manager.validate_session(t3)
        assert data["user_id"] == "user-2"


@pytest.mark.unit
class TestSessionMaxPerUser:
    """Max sessions per user is enforced by evicting the oldest."""

    @pytest.mark.asyncio
    async def test_max_sessions_evicts_oldest(self):
        sm = SessionManager(session_ttl=3600, max_sessions_per_user=2)
        t1 = await sm.create_session("user-1")
        t2 = await sm.create_session("user-1")
        t3 = await sm.create_session("user-1")  # Should evict t1

        with pytest.raises(SessionError):
            await sm.validate_session(t1)

        # t2 and t3 still valid
        await sm.validate_session(t2)
        await sm.validate_session(t3)


@pytest.mark.unit
class TestSessionInvalidToken:
    """Validating a non-existent token raises SessionError."""

    @pytest.mark.asyncio
    async def test_invalid_token_rejected(self, session_manager):
        with pytest.raises(SessionError, match="Invalid"):
            await session_manager.validate_session("nonexistent-token")


@pytest.mark.unit
class TestSessionRevokeNonexistent:
    """Revoking a non-existent session raises SessionError."""

    @pytest.mark.asyncio
    async def test_revoke_nonexistent(self, session_manager):
        with pytest.raises(SessionError, match="not found"):
            await session_manager.revoke_session("nonexistent-token")


# ===========================================================================
# AuthInterceptor tests
# ===========================================================================


@pytest.mark.unit
class TestAuthInterceptorBearerToken:
    """Bearer token path through the interceptor."""

    @pytest.mark.asyncio
    async def test_bearer_token_oauth(self):
        mock_oauth = AsyncMock(spec=OAuthValidator)
        mock_oauth.validate_token = AsyncMock(
            return_value={"sub": "user-123", "iss": "issuer", "aud": "aud"}
        )

        interceptor = AuthInterceptor(oauth=mock_oauth)
        metadata = {"authorization": "Bearer header.payload.signature"}

        ctx = await interceptor.authenticate(metadata)
        assert ctx is not None
        assert ctx.method == "oauth"
        assert ctx.identity == "user-123"
        mock_oauth.validate_token.assert_awaited_once()


@pytest.mark.unit
class TestAuthInterceptorApiKey:
    """API key path through the interceptor."""

    @pytest.mark.asyncio
    async def test_api_key_auth(self):
        mock_keys = AsyncMock(spec=ApiKeyManager)
        mock_keys.validate_key = AsyncMock(
            return_value={
                "key_id": "ek_abc",
                "name": "test-key",
                "scopes": ["read"],
                "created_at": 0,
                "expires_at": None,
            }
        )

        interceptor = AuthInterceptor(api_keys=mock_keys)
        metadata = {"x-api-key": "eks_secret"}

        ctx = await interceptor.authenticate(metadata)
        assert ctx is not None
        assert ctx.method == "api_key"
        assert ctx.identity == "test-key"
        assert ctx.scopes == ["read"]


@pytest.mark.unit
class TestAuthInterceptorSessionToken:
    """Session token path through the interceptor."""

    @pytest.mark.asyncio
    async def test_session_token_auth(self):
        mock_sessions = AsyncMock(spec=SessionManager)
        mock_sessions.validate_session = AsyncMock(
            return_value={
                "user_id": "user-789",
                "created_at": 0,
                "expires_at": 9999999999,
                "metadata": {"ip": "1.2.3.4"},
            }
        )

        interceptor = AuthInterceptor(sessions=mock_sessions)
        metadata = {"x-session-token": "abcdef1234"}

        ctx = await interceptor.authenticate(metadata)
        assert ctx is not None
        assert ctx.method == "session"
        assert ctx.identity == "user-789"


@pytest.mark.unit
class TestAuthInterceptorNoAuth:
    """Requests with no credentials are rejected."""

    @pytest.mark.asyncio
    async def test_no_auth_returns_none(self):
        interceptor = AuthInterceptor(
            oauth=AsyncMock(spec=OAuthValidator),
            api_keys=AsyncMock(spec=ApiKeyManager),
            sessions=AsyncMock(spec=SessionManager),
        )
        metadata = {}

        ctx = await interceptor.authenticate(metadata)
        assert ctx is None


@pytest.mark.unit
class TestAuthInterceptorHealthBypass:
    """Health check endpoints bypass authentication."""

    def test_health_check_bypasses_auth(self):
        interceptor = AuthInterceptor()
        continuation = MagicMock(return_value="handler")
        details = MagicMock()
        details.method = "/entdb.EntDBService/Health"
        details.invocation_metadata = []

        result = interceptor.intercept_service(continuation, details)
        continuation.assert_called_once()
        assert result == "handler"


@pytest.mark.unit
class TestAuthInterceptorGrpcReject:
    """The gRPC interceptor rejects unauthenticated requests."""

    def test_no_creds_rejected_grpc(self):
        interceptor = AuthInterceptor()
        continuation = MagicMock()
        details = MagicMock()
        details.method = "/entdb.EntDBService/ExecuteAtomic"
        details.invocation_metadata = []

        result = interceptor.intercept_service(continuation, details)
        continuation.assert_not_called()
        assert result is not None  # abort handler


@pytest.mark.unit
class TestAuthContextDataclass:
    """AuthContext default construction."""

    def test_auth_context_defaults(self):
        ctx = AuthContext()
        assert ctx.method == ""
        assert ctx.identity == ""
        assert ctx.scopes == []
        assert ctx.claims == {}
        assert ctx.metadata == {}

    def test_auth_context_populated(self):
        ctx = AuthContext(
            method="oauth",
            identity="user-123",
            scopes=["read"],
            claims={"sub": "user-123"},
        )
        assert ctx.method == "oauth"
        assert ctx.identity == "user-123"


@pytest.mark.unit
class TestAuthInterceptorPriority:
    """OAuth takes priority when both Bearer and x-api-key are present."""

    @pytest.mark.asyncio
    async def test_oauth_takes_priority_over_api_key(self):
        mock_oauth = AsyncMock(spec=OAuthValidator)
        mock_oauth.validate_token = AsyncMock(return_value={"sub": "oauth-user"})
        mock_keys = AsyncMock(spec=ApiKeyManager)

        interceptor = AuthInterceptor(oauth=mock_oauth, api_keys=mock_keys)
        metadata = {
            "authorization": "Bearer header.payload.signature",
            "x-api-key": "eks_secret",
        }

        ctx = await interceptor.authenticate(metadata)
        assert ctx.method == "oauth"
        mock_keys.validate_key.assert_not_awaited()
