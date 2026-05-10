// Tests for the tenant.CheckTenant gate. The four cases in
// docs/go-port/shared/error-mapping.md mirror the Python `_check_tenant`
// behaviour at api/grpc_server.py:362.
//
// The trailer-attachment case is exercised through an in-memory
// grpc.Server (same harness shape as
// server/go/internal/errs/trailers_test.go) so the redirect trailer is
// observed on the wire — a pure-context test would not exercise the
// grpc.SetTrailer code path.

package tenant_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
	"github.com/elloloop/tenant-shard-db/server/go/internal/tenant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func newStore(t *testing.T) *globalstore.GlobalStore {
	t.Helper()
	gs, err := globalstore.New(globalstore.Options{DataDir: t.TempDir(), WALMode: true})
	if err != nil {
		t.Fatalf("globalstore.New: %v", err)
	}
	t.Cleanup(func() { _ = gs.Close() })
	return gs
}

// TestCheckTenant_HappyPath: tenant exists, sharding owns it, region
// matches.
func TestCheckTenant_HappyPath(t *testing.T) {
	gs := newStore(t)
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	err := tenant.CheckTenant(ctx, "acme", gs, &tenant.Sharding{NodeID: "n1"}, tenant.Options{ServedRegion: "us-east-1"})
	if err != nil {
		t.Fatalf("CheckTenant: %v", err)
	}
}

// TestCheckTenant_SingleNodeDefault: a zero-value Sharding (nil IsMine)
// is treated as always-mine; a missing globalstore skips the existence
// check; both means CheckTenant succeeds.
func TestCheckTenant_SingleNodeDefault(t *testing.T) {
	if err := tenant.CheckTenant(context.Background(), "any", nil, nil, tenant.Options{}); err != nil {
		t.Fatalf("CheckTenant defaults: %v", err)
	}
}

// TestCheckTenant_EmptyTenantID rejects an empty argument before doing
// any I/O.
func TestCheckTenant_EmptyTenantID(t *testing.T) {
	err := tenant.CheckTenant(context.Background(), "", nil, nil, tenant.Options{})
	if errs.Code(err) != codes.InvalidArgument {
		t.Fatalf("empty tenant_id: code=%v err=%v", errs.Code(err), err)
	}
}

// TestCheckTenant_NotFound: tenant absent from globalstore -> NOT_FOUND.
func TestCheckTenant_NotFound(t *testing.T) {
	gs := newStore(t)
	err := tenant.CheckTenant(context.Background(), "ghost", gs,
		&tenant.Sharding{NodeID: "n1"}, tenant.Options{})
	if errs.Code(err) != codes.NotFound {
		t.Fatalf("ghost tenant: code=%v err=%v", errs.Code(err), err)
	}
}

// TestCheckTenant_RegionMismatch: existing tenant pinned to eu-west-1
// served by a us-east-1 node -> FAILED_PRECONDITION (permanent).
func TestCheckTenant_RegionMismatch(t *testing.T) {
	gs := newStore(t)
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", "eu-west-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	err := tenant.CheckTenant(ctx, "acme", gs,
		&tenant.Sharding{NodeID: "n1"},
		tenant.Options{ServedRegion: "us-east-1"})
	if errs.Code(err) != codes.FailedPrecondition {
		t.Fatalf("region mismatch: code=%v err=%v", errs.Code(err), err)
	}
}

// TestCheckTenant_ShardingMiss exercises the redirect-trailer path
// over a real in-memory gRPC connection so grpc.SetTrailer's stream
// lookup succeeds. The handler returns whatever CheckTenant produces;
// the test asserts the wire trailer carries the owner node id.
func TestCheckTenant_ShardingMiss(t *testing.T) {
	gs := newStore(t)
	if _, err := gs.CreateTenant(context.Background(), "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	sh := &tenant.Sharding{
		NodeID:          "node-a",
		AssignedTenants: []string{"otherco"},
		IsMine:          func(id string) bool { return id == "otherco" },
		Owner:           func(id string) string { return "node-b" },
	}

	trailer, st := runHandler(t, func(ctx context.Context) error {
		return tenant.CheckTenant(ctx, "acme", gs, sh, tenant.Options{})
	})
	if st.Code() != codes.Unavailable {
		t.Fatalf("code: %v", st.Code())
	}
	got := trailer.Get(errs.TrailerRedirectNode)
	if len(got) != 1 || got[0] != "node-b" {
		t.Fatalf("redirect trailer: got %v, want [node-b]", got)
	}
}

// TestCheckTenant_ShardingMiss_NoOwner: when the sharding registry has
// no owner mapping, the redirect trailer is omitted but the status
// remains UNAVAILABLE (the SDK simply can't redirect — Python behaves
// the same way at api/grpc_server.py:378).
func TestCheckTenant_ShardingMiss_NoOwner(t *testing.T) {
	gs := newStore(t)
	if _, err := gs.CreateTenant(context.Background(), "acme", "Acme", "us-east-1"); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	sh := &tenant.Sharding{
		NodeID:          "node-a",
		AssignedTenants: []string{"otherco"},
		IsMine:          func(id string) bool { return false },
		// Owner intentionally nil.
	}

	trailer, st := runHandler(t, func(ctx context.Context) error {
		return tenant.CheckTenant(ctx, "acme", gs, sh, tenant.Options{})
	})
	if st.Code() != codes.Unavailable {
		t.Fatalf("code: %v", st.Code())
	}
	if got := trailer.Get(errs.TrailerRedirectNode); len(got) != 0 {
		t.Fatalf("expected no redirect trailer, got %v", got)
	}
}

// runHandler is a one-shot in-memory gRPC server: it invokes a handler
// with the server-side ctx and returns the trailer and status the
// client observed. Modeled after errs/trailers_test.go's helper.
func runHandler(t *testing.T, h func(ctx context.Context) error) (metadata.MD, *status.Status) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := grpc.NewServer(grpc.ForceServerCodec(rawCodec{}))
	desc := &grpc.ServiceDesc{
		ServiceName: "tenant.test.GateSvc",
		HandlerType: (*any)(nil),
		Methods: []grpc.MethodDesc{
			{
				MethodName: "Call",
				Handler: func(_ any, ctx context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
					var unused []byte
					if err := dec(&unused); err != nil {
						return nil, err
					}
					if err := h(ctx); err != nil {
						return nil, err
					}
					return []byte{}, nil
				},
			},
		},
		Metadata: "tenant/test.proto",
	}
	srv.RegisterService(desc, struct{}{})
	go func() { _ = srv.Serve(lis) }()
	defer srv.Stop()

	conn, err := grpc.NewClient(
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(rawCodec{})),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var trailer metadata.MD
	var reply []byte
	callErr := conn.Invoke(
		ctx,
		"/tenant.test.GateSvc/Call",
		[]byte{},
		&reply,
		grpc.Trailer(&trailer),
	)
	st, _ := status.FromError(callErr)
	return trailer, st
}

type rawCodec struct{}

func (rawCodec) Marshal(v any) ([]byte, error) {
	if b, ok := v.([]byte); ok {
		return b, nil
	}
	return nil, nil
}
func (rawCodec) Unmarshal(data []byte, v any) error {
	if p, ok := v.(*[]byte); ok {
		*p = data
	}
	return nil
}
func (rawCodec) Name() string { return "tenant-test-raw" }
