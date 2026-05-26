// SPDX-License-Identifier: MIT
package entdb

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	pb "github.com/elloloop/tenant-shard-db/sdk/go/entdb/v2/internal/pb"
)

// fixedJitter is a deterministic jitterSource so backoff durations
// are reproducible in tests. It always returns the same fraction.
type fixedJitter struct{ v float64 }

func (f fixedJitter) Float64() float64 { return f.v }

func TestShortMethod(t *testing.T) {
	cases := map[string]string{
		"/entdb.v1.EntDBService/GetNode": "GetNode",
		"/entdb.v1.EntDBService/ExecuteAtomic": "ExecuteAtomic",
		"GetNode": "GetNode",
		"":        "",
	}
	for in, want := range cases {
		if got := shortMethod(in); got != want {
			t.Errorf("shortMethod(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsRetryable_Unavailable_AllMethods(t *testing.T) {
	err := status.Error(codes.Unavailable, "down")
	// UNAVAILABLE retries for reads AND writes — the request
	// almost certainly never reached the server.
	for _, m := range []string{"GetNode", "ExecuteAtomic", "ShareNode", "DeleteUser"} {
		if !isRetryable(err, m) {
			t.Errorf("isRetryable(UNAVAILABLE, %q) = false, want true", m)
		}
	}
}

func TestIsRetryable_DeadlineExceeded_ReadAllowlistOnly(t *testing.T) {
	err := status.Error(codes.DeadlineExceeded, "slow")

	// Reads in the allowlist: retryable on timeout.
	for _, m := range []string{"GetNode", "QueryNodes", "SearchNodes", "Health", "WaitForOffset", "ExportUserData"} {
		if !isRetryable(err, m) {
			t.Errorf("isRetryable(DEADLINE_EXCEEDED, %q) = false, want true (read allowlist)", m)
		}
	}

	// Writes / mutations: NEVER retried on timeout — the write may
	// already have committed server-side.
	for _, m := range []string{"ExecuteAtomic", "ShareNode", "DeleteUser", "CreateTenant", "TransferOwnership", "AddGroupMember"} {
		if isRetryable(err, m) {
			t.Errorf("isRetryable(DEADLINE_EXCEEDED, %q) = true, want false (mutation must not retry)", m)
		}
	}
}

func TestIsRetryable_TerminalCodes(t *testing.T) {
	for _, c := range []codes.Code{codes.NotFound, codes.PermissionDenied, codes.InvalidArgument, codes.AlreadyExists, codes.Internal} {
		if isRetryable(status.Error(c, "x"), "GetNode") {
			t.Errorf("isRetryable(%s) = true, want false (terminal)", c)
		}
	}
	if isRetryable(nil, "GetNode") {
		t.Error("isRetryable(nil) = true, want false")
	}
}

func TestFullJitterBackoff_BoundedByCeiling(t *testing.T) {
	// At fraction 1.0 the sleep is the full ceiling; the ceiling
	// for attempt n is base*2**n, capped at retryMaxDelay.
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{0, retryBaseDelay},      // 100ms * 1
		{1, 2 * retryBaseDelay},  // 100ms * 2
		{2, 4 * retryBaseDelay},  // 100ms * 4
		{3, 8 * retryBaseDelay},  // 100ms * 8
		{10, retryMaxDelay},      // capped
		{40, retryMaxDelay},      // shift overflow guard -> capped
	}
	for _, c := range cases {
		got := fullJitterBackoff(c.attempt, fixedJitter{1.0})
		// Float math: allow a 1ms slack.
		if got < c.want-time.Millisecond || got > c.want+time.Millisecond {
			t.Errorf("fullJitterBackoff(%d, 1.0) = %v, want ~%v", c.attempt, got, c.want)
		}
	}
}

func TestFullJitterBackoff_ZeroFractionIsZero(t *testing.T) {
	if got := fullJitterBackoff(5, fixedJitter{0.0}); got != 0 {
		t.Errorf("fullJitterBackoff with 0.0 fraction = %v, want 0", got)
	}
}

func TestFullJitterBackoff_HalfFraction(t *testing.T) {
	// Full jitter picks uniformly in [0, ceiling); a 0.5 fraction
	// yields exactly half the ceiling.
	got := fullJitterBackoff(1, fixedJitter{0.5}) // ceiling = 200ms
	want := retryBaseDelay // 100ms
	if got < want-time.Millisecond || got > want+time.Millisecond {
		t.Errorf("fullJitterBackoff(1, 0.5) = %v, want ~%v", got, want)
	}
}

// retryFakeServer fails the first failN GetNode/ExecuteAtomic calls
// with errCode, then serves normally. It counts every hit.
type retryFakeServer struct {
	pb.UnimplementedEntDBServiceServer
	errCode      codes.Code
	failN        int32
	getNodeHits  atomic.Int32
	execHits     atomic.Int32
}

func (s *retryFakeServer) GetNode(_ context.Context, req *pb.GetNodeRequest) (*pb.GetNodeResponse, error) {
	n := s.getNodeHits.Add(1)
	if n <= s.failN {
		return nil, status.Errorf(s.errCode, "transient %d", n)
	}
	return &pb.GetNodeResponse{
		Found: true,
		Node:  &pb.Node{TenantId: req.GetContext().GetTenantId(), NodeId: req.GetNodeId()},
	}, nil
}

func (s *retryFakeServer) ExecuteAtomic(_ context.Context, _ *pb.ExecuteAtomicRequest) (*pb.ExecuteAtomicResponse, error) {
	n := s.execHits.Add(1)
	if n <= s.failN {
		return nil, status.Errorf(s.errCode, "transient %d", n)
	}
	return &pb.ExecuteAtomicResponse{Success: true}, nil
}

// startRetryTransport wires a grpcTransport with the retry
// interceptor installed (via Connect) pointed at an in-process
// server. Jitter is pinned to 0.0 so backoff sleeps are
// instantaneous and the test is fast + deterministic.
func startRetryTransport(t *testing.T, svc *retryFakeServer, maxRetries int) *grpcTransport {
	t.Helper()
	lis := bufconn.Listen(1 << 16)
	srv := grpc.NewServer()
	pb.RegisterEntDBServiceServer(srv, svc)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(func() { srv.Stop() })

	dialer := func(context.Context, string) (net.Conn, error) { return lis.Dial() }

	cfg := defaultConfig()
	cfg.maxRetries = maxRetries
	cfg.retryJitter = fixedJitter{0.0}
	cfg.dialOptions = []grpc.DialOption{
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	tr := newGRPCTransport("passthrough:///bufnet", cfg)
	if err := tr.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { _ = tr.Close() })
	return tr
}

func TestRetryInterceptor_UnavailableRetriesThenSucceeds(t *testing.T) {
	svc := &retryFakeServer{errCode: codes.Unavailable, failN: 2}
	tr := startRetryTransport(t, svc, 3)

	node, err := tr.GetNode(context.Background(), "acme", "user:alice", 1, "n1")
	if err != nil {
		t.Fatalf("GetNode after 2 transient UNAVAILABLE: %v", err)
	}
	if node == nil {
		t.Fatal("expected node, got nil")
	}
	if got := svc.getNodeHits.Load(); got != 3 {
		t.Errorf("server saw %d calls, want 3 (2 fail + 1 success)", got)
	}
}

func TestRetryInterceptor_ExhaustsMaxRetries(t *testing.T) {
	svc := &retryFakeServer{errCode: codes.Unavailable, failN: 100}
	tr := startRetryTransport(t, svc, 2)

	_, err := tr.GetNode(context.Background(), "acme", "user:alice", 1, "n1")
	if err == nil {
		t.Fatal("expected error after exhausting retries, got nil")
	}
	// 1 initial + 2 retries = 3 attempts.
	if got := svc.getNodeHits.Load(); got != 3 {
		t.Errorf("server saw %d calls, want 3 (1 initial + 2 retries)", got)
	}
}

func TestRetryInterceptor_ExecuteAtomicNotRetriedOnDeadlineExceeded(t *testing.T) {
	// ExecuteAtomic is a write — DEADLINE_EXCEEDED must NOT retry.
	svc := &retryFakeServer{errCode: codes.DeadlineExceeded, failN: 100}
	tr := startRetryTransport(t, svc, 5)

	_, err := tr.ExecuteAtomic(context.Background(), "acme", "user:alice", "idem-1", nil)
	if err == nil {
		t.Fatal("expected DEADLINE_EXCEEDED error, got nil")
	}
	if got := svc.execHits.Load(); got != 1 {
		t.Errorf("ExecuteAtomic was called %d times, want exactly 1 (no retry on DEADLINE_EXCEEDED for a write)", got)
	}
}

func TestRetryInterceptor_ExecuteAtomicRetriesOnUnavailable(t *testing.T) {
	// UNAVAILABLE is still retryable even for a write — the request
	// almost certainly never reached the server.
	svc := &retryFakeServer{errCode: codes.Unavailable, failN: 1}
	tr := startRetryTransport(t, svc, 3)

	res, err := tr.ExecuteAtomic(context.Background(), "acme", "user:alice", "idem-1", nil)
	if err != nil {
		t.Fatalf("ExecuteAtomic after 1 transient UNAVAILABLE: %v", err)
	}
	if !res.Success {
		t.Error("expected Success=true")
	}
	if got := svc.execHits.Load(); got != 2 {
		t.Errorf("ExecuteAtomic was called %d times, want 2 (1 fail + 1 success)", got)
	}
}

func TestRetryInterceptor_ReadRetriedOnDeadlineExceeded(t *testing.T) {
	// GetNode is in the read allowlist — DEADLINE_EXCEEDED retries.
	svc := &retryFakeServer{errCode: codes.DeadlineExceeded, failN: 1}
	tr := startRetryTransport(t, svc, 3)

	node, err := tr.GetNode(context.Background(), "acme", "user:alice", 1, "n1")
	if err != nil {
		t.Fatalf("GetNode after 1 transient DEADLINE_EXCEEDED: %v", err)
	}
	if node == nil {
		t.Fatal("expected node, got nil")
	}
	if got := svc.getNodeHits.Load(); got != 2 {
		t.Errorf("server saw %d calls, want 2 (1 fail + 1 success)", got)
	}
}

func TestRetryInterceptor_DisabledWhenMaxRetriesZero(t *testing.T) {
	svc := &retryFakeServer{errCode: codes.Unavailable, failN: 100}
	tr := startRetryTransport(t, svc, 0)

	_, err := tr.GetNode(context.Background(), "acme", "user:alice", 1, "n1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := svc.getNodeHits.Load(); got != 1 {
		t.Errorf("server saw %d calls, want 1 (retries disabled)", got)
	}
}

func TestRetryInterceptor_WallClockBudgetStopsEarly(t *testing.T) {
	// With a non-zero jitter the per-attempt sleep grows; the
	// wall-clock budget must cut retries off well before
	// maxRetries=50 is reached. A tiny 500ms budget keeps the test
	// fast while still exercising the budget logic.
	const budget = 500 * time.Millisecond
	svc := &retryFakeServer{errCode: codes.Unavailable, failN: 1000}
	lis := bufconn.Listen(1 << 16)
	srv := grpc.NewServer()
	pb.RegisterEntDBServiceServer(srv, svc)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(func() { srv.Stop() })
	dialer := func(context.Context, string) (net.Conn, error) { return lis.Dial() }

	cfg := defaultConfig()
	cfg.maxRetries = 50
	cfg.retryBudget = budget
	cfg.retryJitter = fixedJitter{1.0} // full ceiling each time
	cfg.dialOptions = []grpc.DialOption{
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}
	tr := newGRPCTransport("passthrough:///bufnet", cfg)
	if err := tr.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { _ = tr.Close() })

	start := time.Now()
	_, err := tr.GetNode(context.Background(), "acme", "user:alice", 1, "n1")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error after budget exhaustion, got nil")
	}
	// The loop returns the moment the next sleep would exceed the
	// budget, so total elapsed stays under budget + generous slack.
	// It must NOT run the full 50-attempt schedule.
	if elapsed > budget+5*time.Second {
		t.Errorf("call took %v, expected to stop near the %v wall-clock budget", elapsed, budget)
	}
	if got := svc.getNodeHits.Load(); got >= 50 {
		t.Errorf("server saw %d calls — budget did not cut retries short", got)
	}
}
