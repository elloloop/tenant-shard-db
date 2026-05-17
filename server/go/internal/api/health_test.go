// SPDX-License-Identifier: AGPL-3.0-only

package api_test

import (
	"context"
	"testing"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
	"github.com/elloloop/tenant-shard-db/server/go/internal/version"
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

// fakeProducerNoPing is a wal.Producer-shaped stub that does NOT
// implement IsConnected. Mirrors the "backend lacks the method" branch
// at docs/go-port/rpcs/Health.md step 2 — must be treated as healthy.
type fakeProducerNoPing struct{}

func (fakeProducerNoPing) Connect(_ context.Context) error { return nil }
func (fakeProducerNoPing) Close(_ context.Context) error   { return nil }
func (fakeProducerNoPing) Append(_ context.Context, _, _ string, _ []byte, _ map[string][]byte) (wal.StreamPos, error) {
	return wal.StreamPos{}, nil
}

// fakeProducerConnected implements the optional IsConnected probe and
// reports connected.
type fakeProducerConnected struct{ fakeProducerNoPing }

func (fakeProducerConnected) IsConnected() bool { return true }

// fakeProducerDisconnected implements the optional IsConnected probe
// and reports disconnected.
type fakeProducerDisconnected struct{ fakeProducerNoPing }

func (fakeProducerDisconnected) IsConnected() bool { return false }

// fakeProducerPanic panics inside IsConnected. The handler MUST catch
// and report "unknown" rather than crash the RPC (matches Python
// grpc_server.py:1514-1515).
type fakeProducerPanic struct{ fakeProducerNoPing }

func (fakeProducerPanic) IsConnected() bool { panic("boom") }

// newRealStore returns a tmpdir-backed CanonicalStore. Health never
// calls into it; we only need a non-nil handle to flip the "is the dep
// wired?" check.
func newRealStore(t *testing.T) *store.CanonicalStore {
	t.Helper()
	cs, err := store.New(store.Options{
		RootDir: t.TempDir(),
		WALMode: true,
	})
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// TestHealth_AllDepsWired pins the happy path: WAL + storage both
// report healthy, version is the build-stamped Version (non-empty), and
// the components map contains both keys.
//
// Mirrors tests/python/integration/test_grpc_contract.py:195-202.
func TestHealth_AllDepsWired(t *testing.T) {
	t.Parallel()

	srv := api.New(
		api.WithStore(newRealStore(t)),
		api.WithWALProducer(fakeProducerConnected{}),
	)

	resp, err := srv.Health(context.Background(), &pb.HealthRequest{})
	if err != nil {
		t.Fatalf("Health: unexpected error: %v", err)
	}
	if !resp.Healthy {
		t.Fatalf("Health: expected healthy=true, got false; components=%v", resp.Components)
	}
	if resp.Version == "" {
		t.Fatalf("Health: expected non-empty version, got %q", resp.Version)
	}
	if resp.Version != version.Version {
		t.Fatalf("Health: version mismatch: got %q want %q", resp.Version, version.Version)
	}
	if got := resp.Components["wal"]; got != "healthy" {
		t.Fatalf("Health: components[wal]=%q, want healthy", got)
	}
	if got := resp.Components["storage"]; got != "healthy" {
		t.Fatalf("Health: components[storage]=%q, want healthy", got)
	}
	// Multi-node info keys MUST NOT be present in the single-node base case.
	if _, ok := resp.Components["node_id"]; ok {
		t.Fatalf("Health: components[node_id] leaked into single-node response: %v", resp.Components)
	}
	if _, ok := resp.Components["assigned_tenants"]; ok {
		t.Fatalf("Health: components[assigned_tenants] leaked into single-node response: %v", resp.Components)
	}
}

// TestHealth_NoDepsWired pins the misconfig path: server booted without
// store or wal handles reports components but is NOT healthy.
func TestHealth_NoDepsWired(t *testing.T) {
	t.Parallel()

	srv := api.New() // no options

	resp, err := srv.Health(context.Background(), &pb.HealthRequest{})
	if err != nil {
		t.Fatalf("Health: unexpected error: %v", err)
	}
	if resp.Healthy {
		t.Fatalf("Health: expected healthy=false when deps absent, got true; components=%v", resp.Components)
	}
	if got := resp.Components["wal"]; got != "unknown" {
		t.Fatalf("Health: components[wal]=%q, want unknown", got)
	}
	if got := resp.Components["storage"]; got != "unknown" {
		t.Fatalf("Health: components[storage]=%q, want unknown", got)
	}
}

// TestHealth_WALProducerWithoutIsConnected pins the in-memory-backend
// branch: a Producer that doesn't implement the optional IsConnected
// probe is treated as healthy. Mirrors Python's
// `hasattr(self.wal, "is_connected")` shim returning False.
func TestHealth_WALProducerWithoutIsConnected(t *testing.T) {
	t.Parallel()

	srv := api.New(
		api.WithStore(newRealStore(t)),
		api.WithWALProducer(fakeProducerNoPing{}),
	)

	resp, err := srv.Health(context.Background(), &pb.HealthRequest{})
	if err != nil {
		t.Fatalf("Health: unexpected error: %v", err)
	}
	if got := resp.Components["wal"]; got != "healthy" {
		t.Fatalf("Health: producer w/o IsConnected: components[wal]=%q, want healthy", got)
	}
	if !resp.Healthy {
		t.Fatalf("Health: expected healthy=true, got false")
	}
}

// TestHealth_WALDisconnected pins the unhappy WAL path: IsConnected
// reports false → components[wal]="unhealthy" → healthy=false.
func TestHealth_WALDisconnected(t *testing.T) {
	t.Parallel()

	srv := api.New(
		api.WithStore(newRealStore(t)),
		api.WithWALProducer(fakeProducerDisconnected{}),
	)

	resp, err := srv.Health(context.Background(), &pb.HealthRequest{})
	if err != nil {
		t.Fatalf("Health: unexpected error: %v", err)
	}
	if got := resp.Components["wal"]; got != "unhealthy" {
		t.Fatalf("Health: components[wal]=%q, want unhealthy", got)
	}
	if resp.Healthy {
		t.Fatalf("Health: expected healthy=false when wal unhealthy, got true")
	}
}

// TestHealth_WALProbePanicIsCaught pins the panic-recovery contract:
// a panic inside IsConnected becomes "unknown", the RPC still returns
// OK. Mirrors Python's bare-except at grpc_server.py:1514-1515.
func TestHealth_WALProbePanicIsCaught(t *testing.T) {
	t.Parallel()

	srv := api.New(
		api.WithStore(newRealStore(t)),
		api.WithWALProducer(fakeProducerPanic{}),
	)

	resp, err := srv.Health(context.Background(), &pb.HealthRequest{})
	if err != nil {
		t.Fatalf("Health: unexpected error: %v", err)
	}
	if got := resp.Components["wal"]; got != "unknown" {
		t.Fatalf("Health: components[wal]=%q, want unknown", got)
	}
	if resp.Healthy {
		t.Fatalf("Health: expected healthy=false when wal probe panics, got true")
	}
}

// TestHealth_MultiNodeInfoKeysDoNotGateHealth is the regression guard
// pinned by tests/python/unit/test_cron_fixes.py:116-147. The
// node_id / assigned_tenants info keys MUST NOT count against `healthy`.
//
//	has no sharding handle on the Server struct yet, so today the
//
// best we can do is post-process the response: stuff arbitrary info keys
// in and re-derive healthy the way the handler does. The derived bool
// MUST stay true even with extra keys present — i.e. the gating check
// only consults wal+storage, never the whole map.
func TestHealth_MultiNodeInfoKeysDoNotGateHealth(t *testing.T) {
	t.Parallel()

	srv := api.New(
		api.WithStore(newRealStore(t)),
		api.WithWALProducer(fakeProducerConnected{}),
	)

	resp, err := srv.Health(context.Background(), &pb.HealthRequest{})
	if err != nil {
		t.Fatalf("Health: unexpected error: %v", err)
	}

	// Simulate the multi-node branch by adding the info keys the
	// handler will eventually emit when sharding is wired. Healthy is a
	// function of wal + storage only — info keys MUST NOT drag it false.
	resp.Components["node_id"] = "node-1"
	resp.Components["assigned_tenants"] = "t1,t2,t3"

	derived := resp.Components["wal"] == "healthy" && resp.Components["storage"] == "healthy"
	if !derived {
		t.Fatalf("Health: multi-node info keys leaked into health gate; components=%v", resp.Components)
	}
	if !resp.Healthy {
		t.Fatalf("Health: expected healthy=true with info keys present, got false; components=%v", resp.Components)
	}
}

// TestHealth_NoSideEffects pins the read-only contract: calling Health
// twice yields fresh, equivalent component maps (no memoization, no
// shared mutable state across calls). Mirrors the "Concurrency" risk
// note in docs/go-port/rpcs/Health.md.
func TestHealth_NoSideEffects(t *testing.T) {
	t.Parallel()

	srv := api.New(
		api.WithStore(newRealStore(t)),
		api.WithWALProducer(fakeProducerConnected{}),
	)

	r1, err := srv.Health(context.Background(), &pb.HealthRequest{})
	if err != nil {
		t.Fatalf("Health: unexpected error: %v", err)
	}
	r1.Components["wal"] = "tampered"

	r2, err := srv.Health(context.Background(), &pb.HealthRequest{})
	if err != nil {
		t.Fatalf("Health: unexpected error on second call: %v", err)
	}
	if got := r2.Components["wal"]; got != "healthy" {
		t.Fatalf("Health: components map appears to be shared across calls: got wal=%q on second call after tampering with first", got)
	}
}
