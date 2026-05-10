// SPDX-License-Identifier: AGPL-3.0-only

package auth

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"

	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
)

// invoke runs the unary interceptor against a fake gRPC method and
// returns the augmented context (or error). The handler simply
// captures the context for the test to inspect.
func invoke(t *testing.T, i *Interceptor, fullMethod string, md metadata.MD) (context.Context, error) {
	t.Helper()
	ctx := metadata.NewIncomingContext(context.Background(), md)
	info := &grpc.UnaryServerInfo{FullMethod: fullMethod}
	var captured context.Context
	handler := func(ctx context.Context, req any) (any, error) {
		captured = ctx
		return nil, nil
	}
	_, err := i.Unary()(ctx, struct{}{}, info, handler)
	if err != nil {
		return nil, err
	}
	return captured, nil
}

func TestInterceptor_HealthBypass(t *testing.T) {
	i := NewInterceptor(nil, nil, nil) // no validators -> any other RPC fails
	for _, m := range []string{
		"/entdb.v1.EntDBService/Health",
		"/grpc.health.v1.Health/Check",
	} {
		ctx, err := invoke(t, i, m, nil)
		if err != nil {
			t.Errorf("%s: expected bypass, got err=%v", m, err)
			continue
		}
		// Health bypass MUST NOT install an Identity on context.
		if _, ok := IdentityFromContext(ctx); ok {
			t.Errorf("%s: expected no identity on bypassed context", m)
		}
	}
}

func TestInterceptor_NoCredentials(t *testing.T) {
	i := NewInterceptor(nil, nil, nil)
	_, err := invoke(t, i, "/entdb.v1.EntDBService/GetNode", nil)
	if err == nil {
		t.Fatal("expected UNAUTHENTICATED")
	}
	if errs.Code(err) != codes.Unauthenticated {
		t.Errorf("code = %v, want Unauthenticated", errs.Code(err))
	}
}

func TestInterceptor_OAuthHappyPath(t *testing.T) {
	secret := []byte("oauth-secret")
	v := NewMemoryOAuthValidator("https://issuer.test", "entdb-test")
	v.AddKey("k1", JWKKey{Alg: "HS256", HS256Secret: secret})
	i := NewInterceptor(v, nil, nil)

	tok, err := SignHS256ForTest(secret, "k1", map[string]any{
		"sub": "alice",
		"iss": "https://issuer.test",
		"aud": "entdb-test",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	md := metadata.Pairs("authorization", "Bearer "+tok)
	ctx, err := invoke(t, i, "/entdb.v1.EntDBService/GetNode", md)
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	id, ok := IdentityFromContext(ctx)
	if !ok {
		t.Fatal("expected identity on context")
	}
	if id.Method != MethodOAuth || id.Subject != "alice" {
		t.Errorf("identity = %+v", id)
	}
	// Authoritative wraps a bare sub as user:.
	if got := Authoritative(ctx, System("admin")); got != User("alice") {
		t.Errorf("Authoritative = %v, want User(alice)", got)
	}
}

func TestInterceptor_OAuthInvalidToken(t *testing.T) {
	v := NewMemoryOAuthValidator("iss", "aud")
	v.AddKey("k", JWKKey{Alg: "HS256", HS256Secret: []byte("right")})
	i := NewInterceptor(v, nil, nil)

	bad, _ := SignHS256ForTest([]byte("wrong"), "k", map[string]any{
		"sub": "alice", "iss": "iss", "aud": "aud",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	md := metadata.Pairs("authorization", "Bearer "+bad)
	_, err := invoke(t, i, "/entdb.v1.EntDBService/GetNode", md)
	if err == nil || errs.Code(err) != codes.Unauthenticated {
		t.Fatalf("want UNAUTHENTICATED, got %v", err)
	}
}

func TestInterceptor_APIKeyHappyPath(t *testing.T) {
	keys := NewMemoryAPIKeyManager()
	keys.Add("k-secret", "ci-bot", []string{"read"})
	i := NewInterceptor(nil, keys, nil)

	md := metadata.Pairs("x-api-key", "k-secret")
	ctx, err := invoke(t, i, "/entdb.v1.EntDBService/GetNode", md)
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	id, _ := IdentityFromContext(ctx)
	if id.Method != MethodAPIKey || id.Subject != "ci-bot" {
		t.Errorf("identity = %+v", id)
	}
	if len(id.Scopes) != 1 || id.Scopes[0] != "read" {
		t.Errorf("scopes = %v", id.Scopes)
	}
}

func TestInterceptor_APIKeyInvalid(t *testing.T) {
	keys := NewMemoryAPIKeyManager()
	i := NewInterceptor(nil, keys, nil)
	md := metadata.Pairs("x-api-key", "nope")
	_, err := invoke(t, i, "/entdb.v1.EntDBService/GetNode", md)
	if err == nil || errs.Code(err) != codes.Unauthenticated {
		t.Fatalf("want UNAUTHENTICATED, got %v", err)
	}
}

func TestInterceptor_SessionHappyPath(t *testing.T) {
	sess := NewMemorySessionManager(time.Hour)
	tok, _ := sess.Create("user:alice", nil)
	i := NewInterceptor(nil, nil, sess)

	md := metadata.Pairs("x-session-token", tok)
	ctx, err := invoke(t, i, "/entdb.v1.EntDBService/GetNode", md)
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	id, _ := IdentityFromContext(ctx)
	if id.Method != MethodSession || id.Subject != "user:alice" {
		t.Errorf("identity = %+v", id)
	}
	// A session for "user:alice" stays user:alice through Authoritative.
	if got := Authoritative(ctx, Admin("root")); got != User("alice") {
		t.Errorf("Authoritative = %v, want User(alice)", got)
	}
}

func TestInterceptor_SessionInvalid(t *testing.T) {
	sess := NewMemorySessionManager(time.Hour)
	i := NewInterceptor(nil, nil, sess)
	md := metadata.Pairs("x-session-token", "nope")
	_, err := invoke(t, i, "/entdb.v1.EntDBService/GetNode", md)
	if err == nil || errs.Code(err) != codes.Unauthenticated {
		t.Fatalf("want UNAUTHENTICATED, got %v", err)
	}
}

// TestInterceptor_OrderBearerBeatsAPIKey pins the priority chain:
// when both Authorization (with a JWT shape) and x-api-key are
// present, the bearer path is taken. Mirrors auth_interceptor.py:243-264.
func TestInterceptor_OrderBearerBeatsAPIKey(t *testing.T) {
	secret := []byte("s")
	v := NewMemoryOAuthValidator("iss", "aud")
	v.AddKey("k", JWKKey{Alg: "HS256", HS256Secret: secret})
	keys := NewMemoryAPIKeyManager()
	keys.Add("api-secret", "ci-bot", nil)
	i := NewInterceptor(v, keys, nil)

	tok, _ := SignHS256ForTest(secret, "k", map[string]any{
		"sub": "alice", "iss": "iss", "aud": "aud",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	md := metadata.Pairs("authorization", "Bearer "+tok, "x-api-key", "api-secret")
	ctx, err := invoke(t, i, "/entdb.v1.EntDBService/GetNode", md)
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	id, _ := IdentityFromContext(ctx)
	if id.Method != MethodOAuth {
		t.Errorf("expected oauth to win, got %s", id.Method)
	}
}

// TestInterceptor_NonJWTBearerFallsThrough pins the _is_jwt heuristic:
// an opaque (non-JWT) authorization value must NOT be treated as a JWT
// and the chain must fall through to API-key / session. Mirrors
// auth_interceptor.py:246 / :278-281.
func TestInterceptor_NonJWTBearerFallsThrough(t *testing.T) {
	keys := NewMemoryAPIKeyManager()
	keys.Add("api-secret", "ci-bot", nil)
	i := NewInterceptor(NewMemoryOAuthValidator("iss", "aud"), keys, nil)

	md := metadata.Pairs("authorization", "Bearer not-a-jwt-no-dots", "x-api-key", "api-secret")
	ctx, err := invoke(t, i, "/entdb.v1.EntDBService/GetNode", md)
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	id, _ := IdentityFromContext(ctx)
	if id.Method != MethodAPIKey {
		t.Errorf("expected api_key fallback, got %s", id.Method)
	}
}

// fakeServerStream implements grpc.ServerStream just enough for the
// stream interceptor test.
type fakeServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (f *fakeServerStream) Context() context.Context { return f.ctx }

func TestInterceptor_StreamInstallsIdentity(t *testing.T) {
	keys := NewMemoryAPIKeyManager()
	keys.Add("k", "bot", nil)
	i := NewInterceptor(nil, keys, nil)

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-api-key", "k"))
	ss := &fakeServerStream{ctx: ctx}
	info := &grpc.StreamServerInfo{FullMethod: "/entdb.v1.EntDBService/StreamThing"}
	var captured context.Context
	handler := func(srv any, stream grpc.ServerStream) error {
		captured = stream.Context()
		return nil
	}
	if err := i.Stream()(nil, ss, info, handler); err != nil {
		t.Fatalf("Stream: %v", err)
	}
	if _, ok := IdentityFromContext(captured); !ok {
		t.Fatal("expected identity on stream context")
	}
}

func TestInterceptor_StreamHealthBypass(t *testing.T) {
	i := NewInterceptor(nil, nil, nil)
	ss := &fakeServerStream{ctx: context.Background()}
	info := &grpc.StreamServerInfo{FullMethod: "/grpc.health.v1.Health/Check"}
	called := false
	handler := func(srv any, stream grpc.ServerStream) error {
		called = true
		return nil
	}
	if err := i.Stream()(nil, ss, info, handler); err != nil {
		t.Fatalf("Stream: %v", err)
	}
	if !called {
		t.Error("expected handler to run on health bypass")
	}
}
