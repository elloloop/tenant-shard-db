"""
Enhanced authentication system for EntDB.

Provides OAuth 2.0 / OIDC token validation, scoped API key management,
session management with TTL and revocation, and a unified gRPC auth
interceptor.

Modules:
    oauth_validator  - JWT / OIDC token validation with JWKS caching
    api_key_manager  - API key creation, rotation, revocation, and scoping
    session_manager  - In-memory session management with TTL eviction
    auth_interceptor - Unified gRPC interceptor supporting all auth methods
"""

from .api_key_manager import ApiKeyManager
from .auth_interceptor import AuthContext, AuthInterceptor
from .oauth_validator import AuthenticationError, OAuthValidator
from .session_manager import SessionManager

__all__ = [
    "ApiKeyManager",
    "AuthContext",
    "AuthInterceptor",
    "AuthenticationError",
    "OAuthValidator",
    "SessionManager",
]
