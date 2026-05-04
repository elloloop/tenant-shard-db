package entdb

import (
	"context"
	"net"
	"sync/atomic"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	pb "github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/pb"
)

// redirectFakeServer is a minimal EntDBService backend that
// optionally answers GetNode with an UNAVAILABLE redirect carrying
// “entdb-redirect-node“ trailing metadata. After “redirectN“
// requests it stops redirecting and answers normally — this lets a
// single test exercise both the redirect path and the cached
// follow-up.
type redirectFakeServer struct {
	pb.UnimplementedEntDBServiceServer

	nodeID       string // "node-a" / "node-b" — used in redirect trailer
	redirectTo   string // empty = serve normally
	hits         atomic.Int32
	lastTenantID atomic.Value // string — tenant on the last received call
}

func (s *redirectFakeServer) GetNode(ctx context.Context, req *pb.GetNodeRequest) (*pb.GetNodeResponse, error) {
	s.hits.Add(1)
	s.lastTenantID.Store(req.GetContext().GetTenantId())
	if s.redirectTo != "" {
		_ = grpc.SetTrailer(ctx, metadata.Pairs("entdb-redirect-node", s.redirectTo))
		return nil, status.Errorf(
			codes.Unavailable,
			"Tenant %q is not served by this node (try node %s)",
			req.GetContext().GetTenantId(),
			s.redirectTo,
		)
	}
	return &pb.GetNodeResponse{
		Found: true,
		Node: &pb.Node{
			TenantId: req.GetContext().GetTenantId(),
			NodeId:   req.GetNodeId(),
			TypeId:   req.GetTypeId(),
		},
	}, nil
}

// startRedirectServer launches an in-process gRPC server backed by
// “svc“ and returns the bufconn listener so callers can register
// it with a [StaticMapResolver] keyed by the desired node id.
func startRedirectServer(t *testing.T, svc *redirectFakeServer) *bufconn.Listener {
	t.Helper()
	lis := bufconn.Listen(1 << 16)
	srv := grpc.NewServer()
	pb.RegisterEntDBServiceServer(srv, svc)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(func() { srv.Stop() })
	return lis
}

// bufconnResolver maps node ids to [bufconn.Listener]s. We use it
// instead of [StaticMapResolver] in these tests because the
// transport must dial via a contextDialer rather than a real
// host:port.
type bufconnResolver struct {
	listeners map[string]*bufconn.Listener
}

func (r *bufconnResolver) Resolve(nodeID string) (string, error) {
	if _, ok := r.listeners[nodeID]; !ok {
		return "", &EntDBError{Message: "unknown node " + nodeID, Code: "RESOLVE"}
	}
	// passthrough:// preserves the literal endpoint string so the
	// contextDialer (installed in newRedirectTransport) can match
	// it back to the right bufconn listener.
	return "passthrough:///" + nodeID + ".bufnet", nil
}

func newRedirectTransport(t *testing.T, primaryNode string, lis map[string]*bufconn.Listener) *grpcTransport {
	t.Helper()
	resolver := &bufconnResolver{listeners: lis}

	dialer := func(_ context.Context, addr string) (net.Conn, error) {
		// addr arrives stripped of the ``passthrough:///`` scheme
		// since gRPC's passthrough resolver hands the bare endpoint
		// string to the dialer.
		for nodeID, l := range lis {
			if addr == nodeID+".bufnet" {
				return l.Dial()
			}
		}
		return nil, net.UnknownNetworkError(addr)
	}

	cfg := defaultConfig()
	cfg.nodeResolver = resolver
	cfg.dialOptions = []grpc.DialOption{
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	// Use the passthrough:// scheme so the gRPC name resolver
	// hands our literal endpoint to the dialer rather than trying
	// DNS lookups on "node-a.bufnet".
	tr := newGRPCTransport("passthrough:///"+primaryNode+".bufnet", cfg)
	if err := tr.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { _ = tr.Close() })
	return tr
}

func TestRedirect_FollowsHintAndCaches(t *testing.T) {
	// node-a redirects everything to node-b; node-b serves it.
	srvA := &redirectFakeServer{nodeID: "node-a", redirectTo: "node-b"}
	srvB := &redirectFakeServer{nodeID: "node-b"}
	listeners := map[string]*bufconn.Listener{
		"node-a": startRedirectServer(t, srvA),
		"node-b": startRedirectServer(t, srvB),
	}
	tr := newRedirectTransport(t, "node-a", listeners)

	// First call hits node-a, gets redirected to node-b, succeeds.
	got, err := tr.GetNode(context.Background(), "tenant-b", "user:alice", 1, "n1")
	if err != nil {
		t.Fatalf("GetNode #1: %v", err)
	}
	if got == nil || got.NodeID != "n1" {
		t.Fatalf("expected node n1, got %+v", got)
	}
	if srvA.hits.Load() != 1 {
		t.Errorf("node-a hits = %d, want 1", srvA.hits.Load())
	}
	if srvB.hits.Load() != 1 {
		t.Errorf("node-b hits = %d, want 1 after first redirect", srvB.hits.Load())
	}

	// Second call for the same tenant must skip node-a entirely:
	// the cache should route directly to node-b.
	_, err = tr.GetNode(context.Background(), "tenant-b", "user:alice", 1, "n2")
	if err != nil {
		t.Fatalf("GetNode #2: %v", err)
	}
	if srvA.hits.Load() != 1 {
		t.Errorf("node-a should not be hit again, got %d", srvA.hits.Load())
	}
	if srvB.hits.Load() != 2 {
		t.Errorf("node-b hits = %d, want 2", srvB.hits.Load())
	}
}

func TestRedirect_NoMetadataFallsThroughAsError(t *testing.T) {
	// Server returns UNAVAILABLE *without* the redirect trailer —
	// the SDK must surface it as a ConnectionError, not retry.
	svc := &redirectFakeServer{nodeID: "node-a", redirectTo: ""}
	// Start a servicer that always errors.
	lis := bufconn.Listen(1 << 16)
	srv := grpc.NewServer()
	pb.RegisterEntDBServiceServer(srv, &noisyAlwaysUnavailable{})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(func() { srv.Stop() })

	listeners := map[string]*bufconn.Listener{"node-a": lis}
	_ = svc
	tr := newRedirectTransport(t, "node-a", listeners)

	_, err := tr.GetNode(context.Background(), "tenant-x", "user:alice", 1, "n1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// Must be a ConnectionError per translateGRPCError mapping for UNAVAILABLE.
	if _, ok := err.(*ConnectionError); !ok {
		t.Errorf("expected *ConnectionError, got %T: %v", err, err)
	}
}

type noisyAlwaysUnavailable struct {
	pb.UnimplementedEntDBServiceServer
}

func (s *noisyAlwaysUnavailable) GetNode(ctx context.Context, req *pb.GetNodeRequest) (*pb.GetNodeResponse, error) {
	return nil, status.Error(codes.Unavailable, "transient")
}

func TestRedirect_DefaultEndpointWhenNoCacheAndNoRedirect(t *testing.T) {
	// Plain happy path with redirect plumbing installed but no
	// redirect needed — the transport must still talk to the
	// primary endpoint.
	srvA := &redirectFakeServer{nodeID: "node-a"}
	listeners := map[string]*bufconn.Listener{"node-a": startRedirectServer(t, srvA)}
	tr := newRedirectTransport(t, "node-a", listeners)

	got, err := tr.GetNode(context.Background(), "tenant-a", "user:alice", 1, "n1")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if got == nil || got.NodeID != "n1" {
		t.Fatalf("expected n1, got %+v", got)
	}
	if srvA.hits.Load() != 1 {
		t.Errorf("node-a hits = %d, want 1", srvA.hits.Load())
	}
}

func TestWithBaseDomain_InstallsDNSResolver(t *testing.T) {
	// WithBaseDomain is the convenience option that wires up a
	// DNSTemplateResolver. We just need to confirm it lands in the
	// config — actually exercising DNS isn't appropriate in a unit
	// test.
	cfg := defaultConfig()
	WithBaseDomain("entdb.svc.cluster.local")(&cfg)
	if cfg.nodeResolver == nil {
		t.Fatal("WithBaseDomain did not install a resolver")
	}
	dnsR, ok := cfg.nodeResolver.(*DNSTemplateResolver)
	if !ok {
		t.Fatalf("WithBaseDomain installed %T, want *DNSTemplateResolver", cfg.nodeResolver)
	}
	if dnsR.BaseDomain != "entdb.svc.cluster.local" {
		t.Errorf("BaseDomain = %q", dnsR.BaseDomain)
	}
}

func TestWithNodeResolver_StoresInConfig(t *testing.T) {
	r := &StaticMapResolver{Endpoints: map[string]string{"node-a": "1.2.3.4:50051"}}
	cfg := defaultConfig()
	WithNodeResolver(r)(&cfg)
	if cfg.nodeResolver != r {
		t.Errorf("WithNodeResolver did not store the supplied resolver")
	}
}
