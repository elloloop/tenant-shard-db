package api

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

// newStoreForTest builds a CanonicalStore rooted at t.TempDir() and
// pre-opens a single tenant. Returned cleanup must run via t.Cleanup.
func newStoreForTest(t *testing.T, tenant string) *store.CanonicalStore {
	t.Helper()
	cs, err := store.New(store.Options{
		RootDir: t.TempDir(),
		WALMode: true,
	})
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	if err := cs.OpenTenant(context.Background(), tenant); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}
	return cs
}

// TestWaitForOffset_AlreadyReached: the applier has already crossed the
// requested offset, so the handler returns OK + reached=true without
// blocking.
func TestWaitForOffset_AlreadyReached(t *testing.T) {
	t.Parallel()
	cs := newStoreForTest(t, "t1")
	if err := cs.UpdateAppliedOffset(context.Background(), "t1", "entdb-wal", 0, 42); err != nil {
		t.Fatalf("UpdateAppliedOffset: %v", err)
	}
	srv := New(WithStore(cs))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	resp, err := srv.WaitForOffset(ctx, &pb.WaitForOffsetRequest{
		Context:        &pb.RequestContext{TenantId: "t1"},
		StreamPosition: "entdb-wal:0:10",
	})
	if err != nil {
		t.Fatalf("WaitForOffset: %v", err)
	}
	if !resp.GetReached() {
		t.Fatalf("Reached: got false, want true")
	}
	if got, want := resp.GetCurrentPosition(), "entdb-wal:0:42"; got != want {
		t.Fatalf("CurrentPosition: got %q, want %q", got, want)
	}
}

// TestWaitForOffset_ConcurrentUpdateUnblocks: the handler blocks on a
// target the applier has not yet reached, then a concurrent
// UpdateAppliedOffset crosses the target and the handler returns OK.
func TestWaitForOffset_ConcurrentUpdateUnblocks(t *testing.T) {
	t.Parallel()
	cs := newStoreForTest(t, "t1")
	srv := New(WithStore(cs))

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Fire the update from a goroutine after a short delay so the
	// handler is parked inside sync.Cond.Wait when we cross the target.
	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = cs.UpdateAppliedOffset(context.Background(), "t1", "entdb-wal", 0, 7)
	}()

	resp, err := srv.WaitForOffset(ctx, &pb.WaitForOffsetRequest{
		Context:        &pb.RequestContext{TenantId: "t1"},
		StreamPosition: "entdb-wal:0:7",
	})
	if err != nil {
		t.Fatalf("WaitForOffset: %v", err)
	}
	if !resp.GetReached() {
		t.Fatalf("Reached: got false, want true")
	}
	if got, want := resp.GetCurrentPosition(), "entdb-wal:0:7"; got != want {
		t.Fatalf("CurrentPosition: got %q, want %q", got, want)
	}
}

// TestWaitForOffset_DeadlineExceeded: the applier never reaches the
// requested offset; the per-RPC ctx deadline fires and the handler
// returns codes.DeadlineExceeded.
func TestWaitForOffset_DeadlineExceeded(t *testing.T) {
	t.Parallel()
	cs := newStoreForTest(t, "t1")
	srv := New(WithStore(cs))

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := srv.WaitForOffset(ctx, &pb.WaitForOffsetRequest{
		Context:        &pb.RequestContext{TenantId: "t1"},
		StreamPosition: "entdb-wal:0:999",
	})
	if err == nil {
		t.Fatalf("WaitForOffset: expected error, got nil")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("WaitForOffset: error is not a grpc status: %v", err)
	}
	if st.Code() != codes.DeadlineExceeded {
		t.Fatalf("WaitForOffset: got code %v, want DeadlineExceeded", st.Code())
	}
}

// TestWaitForOffset_TimeoutMsRequestField: when the request carries an
// explicit timeout_ms shorter than the outer ctx deadline, that timeout
// drives the cancel (still surfaced as DEADLINE_EXCEEDED per spec).
func TestWaitForOffset_TimeoutMsRequestField(t *testing.T) {
	t.Parallel()
	cs := newStoreForTest(t, "t1")
	srv := New(WithStore(cs))

	// Outer ctx has plenty of headroom; timeout_ms=50 is the limit.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := time.Now()
	_, err := srv.WaitForOffset(ctx, &pb.WaitForOffsetRequest{
		Context:        &pb.RequestContext{TenantId: "t1"},
		StreamPosition: "entdb-wal:0:999",
		TimeoutMs:      50,
	})
	elapsed := time.Since(start)
	if err == nil {
		t.Fatalf("WaitForOffset: expected error, got nil")
	}
	if status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("WaitForOffset: got code %v, want DeadlineExceeded", status.Code(err))
	}
	if elapsed > time.Second {
		t.Fatalf("WaitForOffset: took %v, expected <1s (timeout_ms should bound the wait)", elapsed)
	}
}

// TestWaitForOffset_MissingTenant: tenant_id is required by the handler.
func TestWaitForOffset_MissingTenant(t *testing.T) {
	t.Parallel()
	cs := newStoreForTest(t, "t1")
	srv := New(WithStore(cs))

	_, err := srv.WaitForOffset(context.Background(), &pb.WaitForOffsetRequest{
		Context: &pb.RequestContext{TenantId: ""},
	})
	if err == nil {
		t.Fatalf("expected InvalidArgument, got nil")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("got %v, want InvalidArgument", status.Code(err))
	}
}

// TestParseStreamOffset pins the partner of canonical_store.py
// :_parse_stream_offset (90-109): split on last `:`, parse trailing int,
// non-numeric → 0.
func TestParseStreamOffset(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want int64
	}{
		{"", 0},
		{"42", 42},
		{"entdb-wal:0:42", 42},
		{"0:42", 42},
		{"entdb-wal:0:not-a-number", 0},
		{"entdb-wal:0:", 0},
	}
	for _, tc := range cases {
		got := parseStreamOffset(tc.in)
		if got != tc.want {
			t.Errorf("parseStreamOffset(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

// Sanity: an error from the handler should be a gRPC status, not a
// naked Go error — protects the chokepoint contract documented in
// internal/errs.
func TestWaitForOffset_ErrorsAreStatuses(t *testing.T) {
	t.Parallel()
	cs := newStoreForTest(t, "t1")
	srv := New(WithStore(cs))

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := srv.WaitForOffset(ctx, &pb.WaitForOffsetRequest{
		Context:        &pb.RequestContext{TenantId: "t1"},
		StreamPosition: "entdb-wal:0:9999",
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if _, ok := status.FromError(err); !ok {
		t.Fatalf("error is not a grpc status: %v", err)
	}
	// And not a raw context.DeadlineExceeded.
	if errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error must be wrapped in a status, not a raw context.DeadlineExceeded")
	}
}
