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
)

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

	srv := exec.Command(bin,
		"--addr", addr,
		"--data-dir", filepath.Join(tmp, "data"),
		"--wal-backend", "memory",
		"--seed-profile", "e2e",
		"--seed-tenant", itTenant,
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

	itClient = client
	return m.Run()
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
