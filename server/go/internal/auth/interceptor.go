// SPDX-License-Identifier: AGPL-3.0-only

package auth

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// Metadata header names. Lower-case because gRPC normalises metadata
// keys to lower-case on the wire; using lower-case here lets us read
// directly without normalising.
const (
	headerAuthorization = "authorization"
	headerAPIKey        = "x-api-key"
	headerSessionToken  = "x-session-token"

	bearerPrefix = "Bearer "
)

// unauthenticatedMethods is the allow-list of gRPC methods that bypass
// authentication. It MUST mirror the Python set verbatim
// (server/python/entdb_server/auth/auth_interceptor.py:157-162) --
// orchestrators (k8s, ECS) probe the standard health service without
// credentials, and the EntDB-namespaced Health RPC is documented in
// docs/go-port/rpcs/Health.md as unauthenticated.
var unauthenticatedMethods = map[string]struct{}{
	"/entdb.v1.EntDBService/Health": {},
	"/grpc.health.v1.Health/Check":  {},
}

// Interceptor is the gRPC unary+stream auth interceptor. It reads
// credentials from request metadata in a fixed order (bearer -> API key
// -> session) and parks the verified Identity on the request context
// via WithIdentity, where every handler retrieves it through
// Authoritative.
//
// Mirrors AuthInterceptor in
// server/python/entdb_server/auth/auth_interceptor.py:140-220. Each of
// OAuth, APIKeys, Sessions is optional; a nil validator simply means
// that credential type is not accepted (the request continues to the
// next type in the chain).
//
// This struct is safe for concurrent use as long as the configured
// validators are -- the in-memory implementations in this package are.
type Interceptor struct {
	OAuth    OAuthValidator
	APIKeys  APIKeyManager
	Sessions SessionManager
}

// NewInterceptor returns an Interceptor wired with the given backends.
// Any backend may be nil; nil simply disables that auth method.
func NewInterceptor(oauth OAuthValidator, keys APIKeyManager, sessions SessionManager) *Interceptor {
	return &Interceptor{OAuth: oauth, APIKeys: keys, Sessions: sessions}
}

// Unary returns a grpc.UnaryServerInterceptor that authenticates every
// inbound request before handing it to the next interceptor in the
// chain. Health and grpc.health.v1.Health/Check bypass auth entirely.
//
// Order matters: install this BEFORE any quota or authorization
// interceptor so they can read the verified Identity off the context.
// See docs/go-port/shared/auth-interceptor.md "Dependencies".
func (i *Interceptor) Unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if _, bypass := unauthenticatedMethods[info.FullMethod]; bypass {
			return handler(ctx, req)
		}
		newCtx, err := i.authenticate(ctx)
		if err != nil {
			return nil, err
		}
		return handler(newCtx, req)
	}
}

// Stream returns a grpc.StreamServerInterceptor with the same
// behaviour as Unary, wrapping the ServerStream so its Context()
// returns the augmented context for the duration of the stream.
func (i *Interceptor) Stream() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if _, bypass := unauthenticatedMethods[info.FullMethod]; bypass {
			return handler(srv, ss)
		}
		newCtx, err := i.authenticate(ss.Context())
		if err != nil {
			return err
		}
		return handler(srv, &serverStreamWithContext{ServerStream: ss, ctx: newCtx})
	}
}

// authenticate runs the credential chain. It returns either the
// augmented context (carrying a trusted Identity) or an
// UNAUTHENTICATED error. Mirrors AuthInterceptor.authenticate in
// auth_interceptor.py:222-276.
func (i *Interceptor) authenticate(ctx context.Context) (context.Context, error) {
	md, _ := metadata.FromIncomingContext(ctx)

	// 1. Bearer token (OAuth/OIDC). Only treated as a JWT if it has
	//    the two-dot shape -- this matches _is_jwt at
	//    auth_interceptor.py:278-281, which lets us coexist with
	//    other authorization schemes a future deployment might add
	//    without misclassifying them as malformed JWTs.
	if raw := firstHeader(md, headerAuthorization); raw != "" {
		bearer := strings.TrimPrefix(raw, bearerPrefix)
		if bearer != "" && i.OAuth != nil && isJWT(bearer) {
			claims, err := i.OAuth.Validate(ctx, bearer)
			if err != nil {
				return nil, err
			}
			subject := identityFromClaims(claims)
			id := Identity{
				Method:  MethodOAuth,
				Subject: subject,
				Claims:  claims,
			}
			return WithIdentity(ctx, id), nil
		}
	}

	// 2. API key.
	if key := firstHeader(md, headerAPIKey); key != "" && i.APIKeys != nil {
		info, err := i.APIKeys.Validate(ctx, key)
		if err != nil {
			return nil, err
		}
		id := Identity{
			Method:   MethodAPIKey,
			Subject:  info.Name,
			Scopes:   info.Scopes,
			Metadata: map[string]any{"key_id": info.KeyID},
		}
		return WithIdentity(ctx, id), nil
	}

	// 3. Session token.
	if tok := firstHeader(md, headerSessionToken); tok != "" && i.Sessions != nil {
		info, err := i.Sessions.Validate(ctx, tok)
		if err != nil {
			return nil, err
		}
		id := Identity{
			Method:   MethodSession,
			Subject:  info.UserID,
			Metadata: info.Metadata,
		}
		return WithIdentity(ctx, id), nil
	}

	return nil, unauthenticatedf("no valid authentication credentials provided")
}

// identityFromClaims picks the identity string out of a claims map.
// Mirrors auth_interceptor.py:248: prefer sub, fall back to email,
// fall back to the literal "unknown".
func identityFromClaims(claims map[string]any) string {
	if s, ok := claims["sub"].(string); ok && s != "" {
		return s
	}
	if s, ok := claims["email"].(string); ok && s != "" {
		return s
	}
	return "unknown"
}

// isJWT is the same heuristic Python uses: a JWT has exactly two dots
// (header.payload.signature). See auth_interceptor.py:278-281.
func isJWT(s string) bool {
	return strings.Count(s, ".") == 2
}

// firstHeader returns the first value for name in md, or "" if absent.
// gRPC metadata keys are lower-case on the wire; metadata.MD's lookup
// is case-insensitive but returns a slice, so we normalise that here.
func firstHeader(md metadata.MD, name string) string {
	if md == nil {
		return ""
	}
	vs := md.Get(name)
	if len(vs) == 0 {
		return ""
	}
	return vs[0]
}

// serverStreamWithContext wraps a grpc.ServerStream so Context() returns
// the augmented context.
type serverStreamWithContext struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *serverStreamWithContext) Context() context.Context { return s.ctx }
