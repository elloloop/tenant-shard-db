// GetNodes RPC.
// Spec: docs/go-port/rpcs/GetNodes.md.
//
// Wire contract: proto/entdb/v1/entdb.proto:58 (rpc), :409-422 (request/
// response).
//
// Semantics:
//
//   - Tenant gate first via Server.checkTenant (NOT_FOUND /
//     UNAVAILABLE / FAILED_PRECONDITION on miss).
//   - Trusted-actor rebinding (auth.Authoritative). The wire actor is
//     UNTRUSTED for any auth decision — the trusted identity from the
//     gRPC interceptor wins.
//   - Cross-tenant role check is BATCH-LEVEL: if the actor is neither a
//     tenant member nor a system actor and has no node_access grants,
//     the whole call aborts PERMISSION_DENIED. There is NO per-id
//     PERMISSION_DENIED on the wire.
//   - Per-id missing OR cross-tenant denied are MERGED into
//     missing_ids -- a deliberate information-leak guard so a foreign
//     actor cannot probe whether a node exists vs whether they lack
//     access (spec "Auth").
//   - The proto's `type_id` field is silently ignored (the ids are
//     iterated and the type_id on disk is trusted; preserved for
//     parity, spec "Wire contract" #2).
//   - Duplicate node_ids are NOT deduped; each lookup runs
//     independently. Order of `nodes` follows request order minus
//     missing/denied (gaps closed, indexes are NOT parallel to
//     `node_ids`).
//   - `after_offset` fences the read against the applier (default
//     30s). A fence timeout is swallowed as "everything missing" for
//     v1 parity (spec "Open Questions" #1).
//   - Any internal error (SQLite, panic) is masked as
//     nodes=[], missing_ids=<all input> with status OK. Preserved
//     verbatim: this masks data-loss bugs but is the documented
//     contract.
//
// Hardening:
//
//   - Batch-size cap: the server aborts RESOURCE_EXHAUSTED at
//     len(node_ids) > maxBatchNodeIDs (1000) BEFORE doing any I/O,
//     preventing large payloads from melting the SQLite pool or
//     exceeding the gRPC 4 MiB default response size. Spec "Notes on
//     performance & limits".
//
// No WAL append, no SQLite write, no audit row. Pure read.

package api

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"github.com/elloloop/tenant-shard-db/server/go/internal/metrics"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

const (
	getNodesMethod = "GetNodes"

	// maxBatchNodeIDs is the server-side guardrail for a single
	// GetNodes request. Above this we reject before doing any I/O
	// with codes.RESOURCE_EXHAUSTED.
	maxBatchNodeIDs = 1000

	// getNodesFanout is the bounded concurrency for per-id store
	// lookups. Caps in-flight SQLite reads at a small constant so a
	// 1000-id batch doesn't starve the pool. Only a perf knob.
	getNodesFanout = 32

	// getNodesAfterOffsetDefault is the 30s default fence timeout.
	getNodesAfterOffsetDefault = 30 * time.Second
)

// GetNodes implements entdb.v1.EntDBService/GetNodes. See file header
// for the full contract.
func (s *Server) GetNodes(ctx context.Context, req *pb.GetNodesRequest) (*pb.GetNodesResponse, error) {
	start := time.Now()
	resultStatus := "ok"
	defer func() {
		metrics.RecordGRPCRequest(ctx, getNodesMethod, resultStatus, time.Since(start))
	}()

	tenantID := req.GetContext().GetTenantId()
	if err := s.checkTenant(ctx, tenantID); err != nil {
		resultStatus = "error"
		return nil, err
	}

	// Trusted-actor: ignore req.Context.Actor for any auth decision.
	// The interceptor (when wired) populates ctx with the trusted
	// identity; without it, we fall through to the claimed actor
	// (documented unit-test / no-auth fallback).
	trusted := auth.Authoritative(ctx, auth.ParseActor(req.GetContext().GetActor()))

	// Batch-size cap. Reject BEFORE any I/O so a hostile / buggy
	// caller cannot cause work to start.
	if len(req.GetNodeIds()) > maxBatchNodeIDs {
		resultStatus = "error"
		return nil, errs.Errorf(codes.ResourceExhausted,
			"GetNodes: batch size %d exceeds limit %d",
			len(req.GetNodeIds()), maxBatchNodeIDs)
	}

	// Empty node_ids -> empty response.
	if len(req.GetNodeIds()) == 0 {
		return &pb.GetNodesResponse{
			Nodes:      []*pb.Node{},
			MissingIds: []string{},
		}, nil
	}

	// Open the per-tenant view before reading. The SQLite store is a
	// materialized view of the WAL (ADR-016); "tenant not opened" means
	// the applier has not materialized this tenant in-process yet, not a
	// client error. Lazy-open it (as the single-node GetNode does) so a
	// valid tenant is not silently reported as all-missing, and so the
	// after_offset fence and cross-tenant grant probe below observe a
	// live handle. A genuine open failure surfaces its real typed code.
	if s.store != nil {
		if err := s.store.OpenTenant(ctx, tenantID); err != nil {
			resultStatus = "error"
			return nil, errs.Errorf(errs.Code(err), "GetNodes: open tenant: %v", err)
		}
	}

	// Cross-tenant role check (BATCH-LEVEL). Returns one of: roleLocal
	// (no global), roleMember (member or system actor), roleCrossTenant
	// (non-member with node_access grants), or PERMISSION_DENIED
	// (non-member, no grants).
	role, err := s.checkCrossTenantRead(ctx, tenantID, trusted)
	if err != nil {
		resultStatus = "error"
		return nil, err
	}

	// after_offset fence. A timeout is observed as "all missing" for
	// v1 (spec "Open Questions" #1): a fence failure is intentionally
	// NOT a hard error here.
	if req.GetAfterOffset() != "" && s.store != nil {
		timeout := getNodesAfterOffsetDefault
		if ms := req.GetWaitTimeoutMs(); ms > 0 {
			timeout = time.Duration(ms) * time.Millisecond
		}
		waitCtx, cancel := context.WithTimeout(ctx, timeout)
		target := parseStreamOffset(req.GetAfterOffset())
		_ = s.store.WaitForOffset(waitCtx, tenantID, target)
		cancel()
	}

	// Resolve actor groups once for cross-tenant per-node ACL
	// filtering. Members + local skip this entirely.
	var actorIDs []string
	if role == roleCrossTenant && s.store != nil {
		actorIDs, _ = s.store.ResolveActorGroups(ctx, tenantID, trusted.String())
	}

	if s.store == nil {
		// No per-tenant store wired — everything missing, status OK,
		// metric=error so the swallow doesn't inflate the ok counter.
		resultStatus = "error"
		return &pb.GetNodesResponse{
			Nodes:      []*pb.Node{},
			MissingIds: append([]string{}, req.GetNodeIds()...),
		}, nil
	}

	ids := req.GetNodeIds()
	// Mailbox scope (#568): when target_user is set, each per-id read is
	// confined to that user's USER_MAILBOX nodes — non-mailbox or
	// other-user ids land in missing_ids, the same signal a genuinely
	// absent id produces.
	targetUser := req.GetTargetUser()
	results := make([]*store.Node, len(ids))
	denied := make([]bool, len(ids))
	// firstErr captures the first genuine fault across the fan-out (#573):
	// a GetNode/CanAccess store error or a per-id panic. Such a fault is
	// distinct from a real miss (node absent → n == nil) and must NOT be
	// reported as a missing id — that masks data loss. A real miss leaves
	// firstErr nil and the id flows to missing_ids as before.
	var firstErr error
	var fatalMu sync.Mutex
	recordErr := func(err error) {
		fatalMu.Lock()
		if firstErr == nil {
			firstErr = err
		}
		fatalMu.Unlock()
	}

	sem := make(chan struct{}, getNodesFanout)
	var wg sync.WaitGroup
	for i, id := range ids {
		i, id := i, id
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer func() {
				<-sem
				wg.Done()
				if r := recover(); r != nil {
					recordErr(fmt.Errorf("panic resolving id %q: %v", id, r))
				}
			}()
			var n *store.Node
			var gerr error
			if targetUser != "" {
				n, gerr = s.store.GetMailboxNode(ctx, tenantID, targetUser, id)
			} else {
				n, gerr = s.store.GetNode(ctx, tenantID, id)
			}
			if gerr != nil {
				// NotFound is a genuine miss (the store signals an absent
				// node via ErrNodeNotFound) → missing_ids. Any other error
				// is a real fault (#573) → surface it.
				if errs.Code(gerr) == codes.NotFound {
					return
				}
				recordErr(gerr)
				return
			}
			if n == nil {
				return // genuine miss -> missing_ids
			}
			// Privacy boundary (#568): a plain tenant batch read must not
			// surface a USER_MAILBOX node by id — report it as missing,
			// matching GetNode and the scan-path exclusion.
			if targetUser == "" && n.StorageMode == int32(store.StorageModeUserMailbox) {
				return // mailbox-private -> missing_ids on a tenant read
			}
			if actorIDs != nil {
				ok, aerr := s.store.CanAccess(ctx, tenantID, id, actorIDs)
				if aerr != nil {
					recordErr(aerr)
					return
				}
				if !ok {
					denied[i] = true
					return
				}
			}
			results[i] = n
		}()
	}
	wg.Wait()

	if firstErr != nil {
		// A genuine fault occurred in at least one fan-out worker; partial
		// state cannot be trusted. Surface it (#573) rather than masking it
		// as all-missing + OK. Preserve typed sentinels; sanitize the rest.
		resultStatus = "error"
		if c := errs.Code(firstErr); c != codes.Unknown {
			return nil, errs.Errorf(c, "GetNodes: %v", firstErr)
		}
		return nil, errs.Internal(ctx, "GetNodes: store", firstErr)
	}

	nodes := make([]*pb.Node, 0, len(ids))
	missing := make([]string, 0)
	for i, id := range ids {
		if results[i] == nil || denied[i] {
			missing = append(missing, id)
			continue
		}
		pn, perr := s.storeNodeToProto(0, results[i])
		if perr != nil {
			// A row that will not marshal is corrupt stored state, not an
			// absent node — surface it rather than folding into missing_ids.
			resultStatus = "error"
			return nil, errs.Internal(ctx, "GetNodes: marshal row", perr)
		}
		nodes = append(nodes, pn)
	}

	return &pb.GetNodesResponse{
		Nodes:      nodes,
		MissingIds: missing,
	}, nil
}

// crossTenantRole / checkCrossTenantRead / storeNodeToProto — shared
// with the other read RPCs and declared in helpers.go (consolidated in
// the round-3 dedupe). The shared type is named `readRole`
// there; the constants (roleLocal / roleMember / roleCrossTenant) are
// untouched, so existing callers in this file still compile.
