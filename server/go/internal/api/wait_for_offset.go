package api

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

// WaitForOffset blocks until the applier on this node has materialized at
// least the requested WAL stream position into the per-tenant SQLite
// view, or the per-RPC deadline elapses, or the context is cancelled.
//
// Spec: docs/go-port/rpcs/WaitForOffset.md. The Go
// port uses the store-level sync.Cond watcher implemented in W1.8
// (store/offset.go) so the handler parks on a single select and does not
// burn a goroutine on a busy-loop.
//
// Differences from the Python handler:
//   - Python swallows all internal errors and returns OK + reached=false.
//     The Go port honours ctx cancellation as a real gRPC status
//     (DEADLINE_EXCEEDED) — this matches grpc-go idiom and is the
//     contract pinned by the W2 task spec.
//   - Topic/partition prefix in stream_position is ignored (parity with
//     canonical_store.py:_parse_stream_offset: split on last `:`, parse
//     trailing int; non-numeric → 0).
func (s *Server) WaitForOffset(ctx context.Context, req *pb.WaitForOffsetRequest) (*pb.WaitForOffsetResponse, error) {
	tenantID := req.GetContext().GetTenantId()
	if tenantID == "" {
		return nil, errs.Errorf(codes.InvalidArgument, "WaitForOffset: tenant_id is required")
	}
	if s.store == nil {
		return nil, errs.Errorf(codes.Unimplemented, "WaitForOffset: store not wired")
	}

	target := parseStreamOffset(req.GetStreamPosition())

	// Honour an explicit request-side timeout_ms by wrapping ctx; the
	// outer per-RPC deadline (if any) still applies. If timeout_ms is
	// zero, use ctx as-is — the parent deadline is the only bound.
	waitCtx := ctx
	if ms := req.GetTimeoutMs(); ms > 0 {
		var cancel context.CancelFunc
		waitCtx, cancel = context.WithTimeout(ctx, time.Duration(ms)*time.Millisecond)
		defer cancel()
	}

	err := s.store.WaitForOffset(waitCtx, tenantID, target)
	if err != nil {
		// Distinguish ctx cancel / deadline (return DEADLINE_EXCEEDED per
		// task spec) from internal store errors (propagate via the errs
		// mapping; status.Code already returns Unknown for naked errors,
		// which the chokepoint upgrades to a real status if needed).
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return nil, errs.Errorf(codes.DeadlineExceeded, "WaitForOffset: %v", err)
		}
		return nil, errs.Errorf(errs.Code(err), "WaitForOffset: %v", err)
	}

	// Read the latest applied offset (after the wait succeeded) and
	// rebuild a "topic:partition:offset"-shaped string so the response
	// shape matches the client's view of the WAL. Matches Python at
	// grpc_server.py:986 (`current_position = get_applied_offset(...) or
	// ""`).
	cur := s.currentAppliedPosition(ctx, tenantID, req.GetStreamPosition())

	return &pb.WaitForOffsetResponse{
		Reached:         true,
		CurrentPosition: cur,
	}, nil
}

// parseStreamOffset mirrors canonical_store.py:_parse_stream_offset (90):
// split on the last `:` and parse the trailing integer. Inputs that do
// not parse cleanly produce 0 — matching Python's `int(...) except 0`
// behaviour. An empty string is therefore "offset 0", which is
// "already reached" once anything has been applied for the tenant.
func parseStreamOffset(s string) int64 {
	if s == "" {
		return 0
	}
	tail := s
	if i := strings.LastIndex(s, ":"); i >= 0 {
		tail = s[i+1:]
	}
	n, err := strconv.ParseInt(tail, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// currentAppliedPosition queries the per-tenant applied offset and
// rebuilds the "topic:partition:offset" string the client expects. The
// (topic, partition) pair is parsed from the request's stream_position;
// if absent we fall back to ("", 0). On any read error or
// not-yet-applied tenant we return "" — matching grpc_server.py:986
// (`current_position = get_applied_offset(...) or ""`).
func (s *Server) currentAppliedPosition(ctx context.Context, tenantID, requested string) string {
	topic, partition := parseStreamTopicPartition(requested)
	off, err := s.store.GetAppliedOffset(ctx, tenantID, topic, partition)
	if err != nil || off == 0 {
		// 0 here means either no row, or applier genuinely at offset 0.
		// Python collapses the latter to "" via `or ""` truthiness; we
		// mirror that to keep contract tests deterministic.
		return ""
	}
	if topic == "" {
		return fmt.Sprintf("%d", off)
	}
	return fmt.Sprintf("%s:%d:%d", topic, partition, off)
}

// parseStreamTopicPartition splits "topic:partition:offset" into
// (topic, partition). The partition is parsed as int32; non-numeric or
// missing → 0. Mirror of canonical_store.py:_parse_stream_offset shape
// awareness (90-109).
func parseStreamTopicPartition(s string) (string, int32) {
	if s == "" {
		return "", 0
	}
	last := strings.LastIndex(s, ":")
	if last < 0 {
		return "", 0
	}
	head := s[:last]
	mid := strings.LastIndex(head, ":")
	if mid < 0 {
		return "", 0
	}
	topic := head[:mid]
	p, err := strconv.ParseInt(head[mid+1:], 10, 32)
	if err != nil {
		return topic, 0
	}
	return topic, int32(p)
}
