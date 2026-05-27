//go:build integration

// Go SDK integration suite — drives the SDK against a REAL entdb-server.
//
// The unit tests run the SDK against an in-process fakeServer with canned
// responses; they prove the SDK's wire-translation logic but NOT that it
// talks to the real server correctly. This suite builds and boots the
// actual server/go/cmd/entdb-server binary on a free port and exercises
// the Go SDK transport over real gRPC — the mirror of the Python contract
// suite (tests/python/integration). It proves the SDK↔server wire
// contract end-to-end: typed int64 round-trip (ADR-028), keyset cursor
// auto-follow (ADR-029), filters, unique-key lookup, and zero-value
// patches — exactly the things fakes cannot verify.
//
// Run:  go test -tags=integration ./...
// (Builds the server; requires ../../../server/go in the same checkout.)

package entdb

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	pb "github.com/elloloop/tenant-shard-db/sdk/go/entdb/v2/internal/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/structpb"
)

// newBootstrapPayload renders the placeholder User the bootstrap write
// creates so the schema-register op has something to ride on (an empty
// operations list is rejected by the handler). Field 1 = email
// (declared unique); field 2 = name. The values are intentionally
// distinct from anything an integration test creates.
func newBootstrapPayload() (*structpb.Struct, error) {
	return structpb.NewStruct(map[string]any{
		"1": "it-bootstrap@example.invalid",
		"2": "Bootstrap",
	})
}

const (
	itTenant = "acme"
	itActor  = "user:e2e-runner"
	// e2e seed profile (server/go/internal/testseed): User type 8001 with
	// email=1, name=2, age=3 (int), active=4 (bool); Product type 8002
	// with a unique sku=1.
	itUserType    = 8001
	itProductType = 8002
)

// itClient is the live SDK client bound to the booted server.
var itClient *DbClient

func TestMain(m *testing.M) { os.Exit(runIntegration(m)) }

func runIntegration(m *testing.M) int {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "integration:", err)
		return 1
	}
	// sdk/go/entdb -> repo root is three levels up.
	repoRoot := filepath.Clean(filepath.Join(wd, "..", "..", ".."))
	serverDir := filepath.Join(repoRoot, "server", "go")
	if _, err := os.Stat(serverDir); err != nil {
		fmt.Fprintf(os.Stderr, "integration: server/go not found at %s: %v\n", serverDir, err)
		return 1
	}

	tmp, err := os.MkdirTemp("", "entdb-it-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "integration:", err)
		return 1
	}
	defer os.RemoveAll(tmp)

	bin := filepath.Join(tmp, "entdb-server")
	build := exec.Command("go", "build", "-o", bin, "./cmd/entdb-server")
	build.Dir = serverDir
	build.Stdout, build.Stderr = os.Stderr, os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "integration: build server: %v\n", err)
		return 1
	}

	port, err := freePort()
	if err != nil {
		fmt.Fprintln(os.Stderr, "integration:", err)
		return 1
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	// EMPTY BOOT (ADR-031). The server starts with no schema, tenant,
	// or users; bootstrapIntegrationContract provisions them through the
	// gRPC API below — same path a real client would use, mirror of the
	// Python conftest's `_bootstrap_contract`.
	srv := exec.Command(bin,
		"--addr", addr,
		"--data-dir", filepath.Join(tmp, "data"),
		"--wal-backend", "memory",
	)
	srv.Stdout, srv.Stderr = os.Stderr, os.Stderr
	if err := srv.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "integration: start server: %v\n", err)
		return 1
	}
	defer func() {
		_ = srv.Process.Kill()
		_, _ = srv.Process.Wait()
	}()

	client, err := NewClient(addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "integration:", err)
		return 1
	}
	ctx := context.Background()
	if err := client.Connect(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "integration: connect: %v\n", err)
		return 1
	}
	defer client.Close()

	deadline := time.Now().Add(15 * time.Second)
	ready := false
	for time.Now().Before(deadline) {
		if _, herr := client.Health(ctx); herr == nil {
			ready = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !ready {
		fmt.Fprintln(os.Stderr, "integration: server never became ready")
		return 1
	}

	if err := bootstrapIntegrationContract(ctx, addr); err != nil {
		fmt.Fprintf(os.Stderr, "integration: bootstrap: %v\n", err)
		return 1
	}

	itClient = client
	return m.Run()
}

// bootstrapIntegrationContract provisions the tenant, the e2e-runner
// user, and the integration-test schema through the live gRPC API
// (ADR-031 empty boot, mirror of tests/python/integration/conftest.py
// :_bootstrap_contract).
//
// Schema:
//
//	User    type_id 8001 — email=1 str unique, name=2 str, age=3 int,
//	                       active=4 bool
//	Product type_id 8002 — sku=1 str unique
//
// These ids match the constants the test cases below use. The schema
// rides a self-describing ExecuteAtomic with a placeholder write
// (an empty op list is rejected); the placeholder uses a node id +
// email that no integration test creates so it can't collide.
func bootstrapIntegrationContract(ctx context.Context, addr string) error {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial bootstrap: %w", err)
	}
	defer conn.Close()
	stub := pb.NewEntDBServiceClient(conn)
	admin := "system:admin"

	if _, err := stub.CreateTenant(ctx, &pb.CreateTenantRequest{
		Actor: admin, TenantId: itTenant, Name: "Acme Corp",
	}); err != nil {
		return fmt.Errorf("CreateTenant: %w", err)
	}
	if _, err := stub.CreateUser(ctx, &pb.CreateUserRequest{
		Actor: admin, UserId: "e2e-runner", Email: "e2e@example.invalid", Name: "E2E",
	}); err != nil {
		return fmt.Errorf("CreateUser: %w", err)
	}
	if _, err := stub.AddTenantMember(ctx, &pb.TenantMemberRequest{
		Actor: admin, TenantId: itTenant, UserId: "e2e-runner", Role: "owner",
	}); err != nil {
		return fmt.Errorf("AddTenantMember: %w", err)
	}
	// Mailbox node test creates a mailbox-private node owned by user
	// "mail-user"; provision them so the membership grant exists.
	if _, err := stub.CreateUser(ctx, &pb.CreateUserRequest{
		Actor: admin, UserId: "mail-user", Email: "mail@example.invalid", Name: "Mail",
	}); err != nil {
		return fmt.Errorf("CreateUser(mail): %w", err)
	}
	if _, err := stub.AddTenantMember(ctx, &pb.TenantMemberRequest{
		Actor: admin, TenantId: itTenant, UserId: "mail-user", Role: "member",
	}); err != nil {
		return fmt.Errorf("AddTenantMember(mail): %w", err)
	}

	// Poll until the tenant is visible (CreateTenant flows through the
	// global WAL → applier; the first per-tenant write must wait).
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		r, gerr := stub.GetTenant(ctx, &pb.GetTenantRequest{Actor: admin, TenantId: itTenant})
		if gerr == nil && r.GetFound() {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Name-free SchemaDescriptor (ADR-031).
	user := &pb.SchemaNodeTypeDef{
		TypeId: itUserType,
		Fields: []*pb.SchemaFieldDef{
			{FieldId: 1, Kind: "str", Unique: true},
			{FieldId: 2, Kind: "str"},
			{FieldId: 3, Kind: "int"},
			{FieldId: 4, Kind: "bool"},
		},
	}
	product := &pb.SchemaNodeTypeDef{
		TypeId: itProductType,
		Fields: []*pb.SchemaFieldDef{
			{FieldId: 1, Kind: "str", Unique: true},
		},
	}
	edgeFromUser := &pb.SchemaEdgeTypeDef{
		EdgeId: 8101, FromTypeId: itUserType, ToTypeId: itUserType,
	}
	descriptor := &pb.SchemaDescriptor{
		NodeTypes: []*pb.SchemaNodeTypeDef{user, product},
		EdgeTypes: []*pb.SchemaEdgeTypeDef{edgeFromUser},
	}

	// Send the schema register on a placeholder User write — empty op
	// list is rejected. The placeholder's id + email avoid colliding
	// with any test below.
	bootstrapPayload, err := newBootstrapPayload()
	if err != nil {
		return fmt.Errorf("bootstrap payload: %w", err)
	}
	if _, err := stub.ExecuteAtomic(ctx, &pb.ExecuteAtomicRequest{
		Context:        &pb.RequestContext{TenantId: itTenant, Actor: itActor},
		IdempotencyKey: "it-bootstrap-schema",
		Schema:         descriptor,
		Operations: []*pb.Operation{
			{Op: &pb.Operation_CreateNode{CreateNode: &pb.CreateNodeOp{
				TypeId: itUserType,
				Id:     "it-bootstrap",
				Data:   bootstrapPayload,
			}}},
		},
		WaitApplied:   true,
		WaitTimeoutMs: 10000,
	}); err != nil {
		return fmt.Errorf("schema register: %w", err)
	}
	return nil
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// eventually polls fn until it returns true or the deadline fires. Reads
// that lack a read-after-write fence (edges; and GetNodeByKey under
// applier backlog) are eventually consistent against the async applier,
// so the integration assertions poll rather than racing the apply loop.
func eventually(t *testing.T, what string, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("condition never held within 5s: %s", what)
}

// TestIntegration_TypedInt64RoundTrip proves an int64 above 2^53 survives
// the full SDK→server→SDK round trip losslessly (ADR-028 / #563 / #572) —
// the corruption fakes could never have caught.
func TestIntegration_TypedInt64RoundTrip(t *testing.T) {
	ctx := context.Background()
	const big = int64(9007199254740993) // 2^53 + 1, the first unsafe odd int

	res, err := itClient.transport.ExecuteAtomic(ctx, itTenant, itActor, "it-int64",
		[]Operation{{
			Type: OpCreateNode, TypeID: itUserType, NodeID: "u-int64",
			Data: map[string]any{"1": "int64@x", "2": "Big Age", "3": big},
		}})
	if err != nil {
		t.Fatalf("ExecuteAtomic: %v", err)
	}
	if !res.Success {
		t.Fatalf("commit not successful: %+v", res)
	}

	node, err := itClient.transport.GetNode(ctx, itTenant, itActor, itUserType, "u-int64")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if node == nil {
		t.Fatal("GetNode returned nil")
	}
	got, ok := node.Payload["3"]
	if !ok {
		t.Fatalf("payload missing field 3 (age): %v", node.Payload)
	}
	if got != big {
		t.Fatalf("age round-trip = %v (%T), want int64(%d) — int64 corrupted over the real wire", got, got, big)
	}
}

// TestIntegration_QueryNodesAutoFollowsRealCursor proves the SDK follows
// the REAL server's keyset cursor (ADR-029 / #564) — previously only
// verified against a fake paginating server.
func TestIntegration_QueryNodesAutoFollowsRealCursor(t *testing.T) {
	ctx := context.Background()
	const n = 250 // > the server's 100-row default page

	ops := make([]Operation, n)
	for i := 0; i < n; i++ {
		ops[i] = Operation{
			Type: OpCreateNode, TypeID: itUserType, NodeID: fmt.Sprintf("pag-%03d", i),
			Data: map[string]any{"1": fmt.Sprintf("pag-%03d@x", i), "2": "Pager"},
		}
	}
	res, err := itClient.transport.ExecuteAtomic(ctx, itTenant, itActor, "it-pager-seed", ops)
	if err != nil {
		t.Fatalf("seed ExecuteAtomic: %v", err)
	}
	if !res.Success {
		t.Fatalf("seed commit not successful: %+v", res)
	}

	// limit=0 => return the COMPLETE set via cursor auto-follow.
	nodes, err := itClient.transport.QueryNodes(ctx, itTenant, itActor, itUserType, nil, 0)
	if err != nil {
		t.Fatalf("QueryNodes: %v", err)
	}
	got := 0
	for _, nd := range nodes {
		if id := nd.NodeID; len(id) >= 4 && id[:4] == "pag-" {
			got++
		}
	}
	if got != n {
		t.Fatalf("auto-follow returned %d of %d seeded nodes — silent truncation against the real server", got, n)
	}
}

// TestIntegration_GetEdgesFromAutoFollowsRealCursor proves the SDK
// follows the real server's edge keyset cursor (ADR-029 / #580) over a
// high-fan-out node — previously edge reads truncated at the page default
// with no cursor.
func TestIntegration_GetEdgesFromAutoFollowsRealCursor(t *testing.T) {
	ctx := context.Background()
	const n = 150 // > the server's 100-row default page
	ops := make([]Operation, n)
	for i := 0; i < n; i++ {
		ops[i] = Operation{
			Type: OpCreateEdge, EdgeTypeID: 8101, // e2e seed: Purchased edge
			FromNodeID: "edge-hub", ToNodeID: fmt.Sprintf("edge-dst-%03d", i),
		}
	}
	res, err := itClient.transport.ExecuteAtomic(ctx, itTenant, itActor, "it-edges-seed", ops)
	if err != nil {
		t.Fatalf("seed edges: %v", err)
	}
	if !res.Success {
		t.Fatalf("seed commit not successful: %+v", res)
	}

	var count int
	eventually(t, "all 150 edges visible via auto-follow", func() bool {
		edges, gerr := itClient.transport.GetEdgesFrom(ctx, itTenant, itActor, "edge-hub", 8101)
		if gerr != nil {
			t.Fatalf("GetEdgesFrom: %v", gerr)
		}
		count = len(edges)
		return count == n
	})
	// Reaching here means auto-follow returned exactly n across pages with
	// no truncation; a stuck prefix (e.g. 100) would have failed the poll.
}

// TestIntegration_MailboxReadAndPrivacy proves USER_MAILBOX writes are
// readable via the mailbox scope and excluded from an ordinary tenant
// read (#568), end-to-end through the real server via the Go SDK.
func TestIntegration_MailboxReadAndPrivacy(t *testing.T) {
	ctx := context.Background()
	const mailUser = "mailbox-user-1"

	res, err := itClient.transport.ExecuteAtomic(ctx, itTenant, itActor, "it-mailbox",
		[]Operation{{
			Type: OpCreateNode, TypeID: itUserType, NodeID: "mbox-1",
			StorageMode: StorageModeUserMailbox, TargetUserID: mailUser,
			Data: map[string]any{"1": "inbox@x", "2": "Mailbox Node"},
		}})
	if err != nil {
		t.Fatalf("ExecuteAtomic: %v", err)
	}
	if !res.Success {
		t.Fatalf("commit not successful: %+v", res)
	}

	// Visible via the mailbox scope.
	eventually(t, "mailbox node visible via GetMailboxNode", func() bool {
		n, gerr := itClient.transport.GetMailboxNode(ctx, itTenant, itActor, mailUser, itUserType, "mbox-1")
		if gerr != nil {
			t.Fatalf("GetMailboxNode: %v", gerr)
		}
		return n != nil && n.NodeID == "mbox-1"
	})

	// Excluded from an ordinary tenant read (privacy boundary, ADR-020).
	n, err := itClient.transport.GetNode(ctx, itTenant, itActor, itUserType, "mbox-1")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if n != nil {
		t.Fatalf("mailbox-private node leaked into a tenant read: %+v", n)
	}
}

// TestIntegration_GetNodeByKeyRealServer proves the unique-key lookup
// wire path (#572 typed value) works against the real server.
func TestIntegration_GetNodeByKeyRealServer(t *testing.T) {
	ctx := context.Background()
	res, err := itClient.transport.ExecuteAtomic(ctx, itTenant, itActor, "it-bykey",
		[]Operation{{
			Type: OpCreateNode, TypeID: itProductType, NodeID: "p-bykey",
			Data: map[string]any{"1": "WIDGET-IT"},
		}})
	if err != nil {
		t.Fatalf("ExecuteAtomic: %v", err)
	}
	if !res.Success {
		t.Fatalf("commit not successful: %+v", res)
	}

	eventually(t, "p-bykey visible by unique key", func() bool {
		node, gerr := itClient.transport.GetNodeByKey(ctx, itTenant, itActor, itProductType, 1, "WIDGET-IT")
		if gerr != nil {
			t.Fatalf("GetNodeByKey: %v", gerr)
		}
		return node != nil && node.NodeID == "p-bykey"
	})
}

// TestIntegration_ZeroValueUpdateRealServer proves a patch carrying an
// explicit zero value (the wire shape Plan.UpdateFields emits, #574) sets
// the field to its zero against the real server — not a no-op.
func TestIntegration_ZeroValueUpdateRealServer(t *testing.T) {
	ctx := context.Background()
	if _, err := itClient.transport.ExecuteAtomic(ctx, itTenant, itActor, "it-zero-create",
		[]Operation{{
			Type: OpCreateNode, TypeID: itUserType, NodeID: "u-zero",
			Data: map[string]any{"1": "zero@x", "2": "Zero", "4": true},
		}}); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Explicit zero in the patch (field 4 = active -> false).
	if _, err := itClient.transport.ExecuteAtomic(ctx, itTenant, itActor, "it-zero-update",
		[]Operation{{
			Type: OpUpdateNode, TypeID: itUserType, NodeID: "u-zero",
			Patch: map[string]any{"4": false},
		}}); err != nil {
		t.Fatalf("update: %v", err)
	}

	node, err := itClient.transport.GetNode(ctx, itTenant, itActor, itUserType, "u-zero")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if got := node.Payload["4"]; got != false {
		t.Fatalf("active after zero-update = %v (%T), want false — explicit zero dropped over the real wire", got, got)
	}
}
