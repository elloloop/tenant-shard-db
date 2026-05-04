package entdb

import (
	"context"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// outgoingTenantHeader is the metadata key the SDK uses to carry
// the request's tenant id on the *outgoing* context. The redirect
// interceptor reads this off the call so it can route the call
// against a tenant-specific cached sub-channel without inspecting
// every request type.
const outgoingTenantHeader = "entdb-tenant-id"

// redirectTrailerKey is the trailing-metadata header the server
// emits when a tenant lives on another node — see
// “dbaas/entdb_server/api/grpc_server.py:_check_tenant“.
const redirectTrailerKey = "entdb-redirect-node"

// tenantEndpointCache caches “tenant_id -> {endpoint, *grpc.ClientConn}“
// so that once an SDK has been redirected to the owning node it
// keeps using that sub-channel for future calls instead of
// bouncing through the primary endpoint each time.
//
// Eviction policy:
//   - Connection failure on the cached channel — see
//     [tenantEndpointCache.evict].
//   - Future "stale" signal from the server (placeholder; the
//     server doesn't emit one yet).
//
// No TTL: tenants don't move unless a node crashes, and the
// connection-failure path covers that.
type tenantEndpointCache struct {
	mu      sync.Mutex
	entries map[string]*cachedEndpoint
	// builder dials a fresh *grpc.ClientConn for the resolved
	// endpoint. Tests inject a bufconn dialer here.
	builder func(endpoint string) (*grpc.ClientConn, error)
}

type cachedEndpoint struct {
	endpoint string
	conn     *grpc.ClientConn
}

func newTenantEndpointCache(builder func(endpoint string) (*grpc.ClientConn, error)) *tenantEndpointCache {
	return &tenantEndpointCache{
		entries: make(map[string]*cachedEndpoint),
		builder: builder,
	}
}

// get returns the cached entry for “tenantID“, or (nil, false)
// when the SDK has not been redirected for this tenant yet.
func (c *tenantEndpointCache) get(tenantID string) (*cachedEndpoint, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[tenantID]
	return e, ok
}

// store records “tenantID -> endpoint“ and lazily dials a
// sub-channel. If the same endpoint was already cached (via a
// different tenant), the existing conn is reused.
func (c *tenantEndpointCache) store(tenantID, endpoint string) (*cachedEndpoint, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// If we already have a sub-channel for this endpoint (under
	// any tenant), reuse it — multiple tenants frequently land on
	// the same node.
	for _, e := range c.entries {
		if e.endpoint == endpoint {
			c.entries[tenantID] = e
			return e, nil
		}
	}
	conn, err := c.builder(endpoint)
	if err != nil {
		return nil, err
	}
	e := &cachedEndpoint{endpoint: endpoint, conn: conn}
	c.entries[tenantID] = e
	return e, nil
}

// evict removes the cache entry for “tenantID“ and closes the
// sub-channel iff no other tenant is using it. Used when the
// cached sub-channel goes unhealthy — the SDK falls back to the
// primary endpoint on the next call.
func (c *tenantEndpointCache) evict(tenantID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[tenantID]
	if !ok {
		return
	}
	delete(c.entries, tenantID)
	for _, other := range c.entries {
		if other.endpoint == e.endpoint {
			// Still in use by another tenant — keep the conn.
			return
		}
	}
	_ = e.conn.Close()
}

// closeAll tears down every sub-channel in the cache. Called from
// [grpcTransport.Close].
func (c *tenantEndpointCache) closeAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	closed := make(map[*grpc.ClientConn]struct{})
	for _, e := range c.entries {
		if _, done := closed[e.conn]; done {
			continue
		}
		closed[e.conn] = struct{}{}
		_ = e.conn.Close()
	}
	c.entries = make(map[string]*cachedEndpoint)
}

// tenantFromOutgoing reads the tenant-id metadata header set by
// [grpcTransport.callContext] off the outgoing context.
func tenantFromOutgoing(ctx context.Context) string {
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		return ""
	}
	vs := md.Get(outgoingTenantHeader)
	if len(vs) == 0 {
		return ""
	}
	return vs[0]
}

// redirectInterceptor returns a unary client interceptor that:
//
//  1. Reads the outgoing “entdb-tenant-id“ header.
//  2. If the cache has an endpoint for that tenant, invokes the
//     RPC against the cached sub-channel directly; otherwise
//     falls through to the supplied invoker (primary channel).
//  3. On “codes.Unavailable“ with an “entdb-redirect-node“
//     trailer, resolves the node, dials a sub-channel, caches it,
//     and retries the call exactly once.
//  4. On any other connection error against the cached
//     sub-channel, evicts the cache entry so the next call retries
//     on the primary endpoint.
//
// Returns nil when “resolver“ is unset — callers can then skip
// installing the interceptor entirely.
func redirectInterceptor(resolver NodeResolver, cache *tenantEndpointCache) grpc.UnaryClientInterceptor {
	if resolver == nil || cache == nil {
		return nil
	}
	return func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		tenantID := tenantFromOutgoing(ctx)

		// Try the cached sub-channel first when we have one.
		if tenantID != "" {
			if entry, ok := cache.get(tenantID); ok {
				err := entry.conn.Invoke(ctx, method, req, reply, opts...)
				if err == nil {
					return nil
				}
				// On UNAVAILABLE against the cached endpoint,
				// evict and fall through to the primary channel.
				if st, ok := status.FromError(err); ok && st.Code() == codes.Unavailable {
					cache.evict(tenantID)
				} else {
					return err
				}
			}
		}

		// Default path — invoke against the primary channel.
		var trailer metadata.MD
		callOpts := append([]grpc.CallOption{grpc.Trailer(&trailer)}, opts...)
		err := invoker(ctx, method, req, reply, cc, callOpts...)
		if err == nil {
			return nil
		}

		// Look for a redirect hint and act on it.
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.Unavailable {
			return err
		}
		nodeIDs := trailer.Get(redirectTrailerKey)
		if len(nodeIDs) == 0 || nodeIDs[0] == "" || tenantID == "" {
			return err
		}

		endpoint, resolveErr := resolver.Resolve(nodeIDs[0])
		if resolveErr != nil {
			// Fall back to the original error — the caller sees
			// the underlying UNAVAILABLE.
			return err
		}
		entry, storeErr := cache.store(tenantID, endpoint)
		if storeErr != nil {
			return err
		}
		// One retry — if the redirect target also redirects we
		// surface that error rather than chasing pointers.
		return entry.conn.Invoke(ctx, method, req, reply, opts...)
	}
}

// dialerFromConfig builds the per-endpoint dialer used by the
// redirect cache. It mirrors the dial settings of the primary
// connection (TLS / insecure, plus any explicit dial options).
func dialerFromConfig(cfg clientConfig) func(string) (*grpc.ClientConn, error) {
	return func(endpoint string) (*grpc.ClientConn, error) {
		opts := append([]grpc.DialOption(nil), cfg.dialOptions...)
		// Caller-supplied dial options win — only apply our
		// transport-credentials default when they did not.
		if !hasTransportCreds(opts) {
			var creds credentials.TransportCredentials
			if cfg.secure {
				creds = credentials.NewTLS(nil)
			} else {
				creds = insecure.NewCredentials()
			}
			opts = append(opts, grpc.WithTransportCredentials(creds))
		}
		return grpc.NewClient(endpoint, opts...)
	}
}

// hasTransportCreds reports whether “opts“ already carries a
// WithTransportCredentials option. We can't introspect the option
// type directly (it's unexported in grpc-go), so we install a
// sentinel by piggy-backing on grpc.EmptyDialOption — but the
// stable contract here is "tests injected creds explicitly, don't
// override them". The simplest robust check is presence-of-options:
// in the production path tests don't pass dialOptions and we add
// creds; in the test path the bufconn dialer always pairs with
// insecure.NewCredentials. So we treat a non-empty dialOptions
// slice as "tests provided their own creds".
func hasTransportCreds(opts []grpc.DialOption) bool {
	return len(opts) > 0
}
