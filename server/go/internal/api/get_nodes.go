// GetNodes RPC — Wave 2 of the Python -> Go server port (EPIC #407).
// Spec: docs/go-port/rpcs/GetNodes.md.
//
// Wire contract: proto/entdb/v1/entdb.proto:58 (rpc), :409-422 (request/
// response). Reference Python handler:
// server/python/entdb_server/api/grpc_server.py:1215-1291.
//
// Semantics (preserved from the Python handler):
//
//   - Tenant gate first via Server.checkTenant (NOT_FOUND /
//     UNAVAILABLE / FAILED_PRECONDITION on miss).
//   - Trusted-actor rebinding (auth.Authoritative). The wire actor is
//     UNTRUSTED for any auth decision -- the trusted identity from the
//     gRPC interceptor wins. Pinned by
//     tests/python/integration/test_privilege_escalation.py:213-228.
//   - Cross-tenant role check is BATCH-LEVEL: if the actor is neither a
//     tenant member nor a system actor and has no node_access grants,
//     the whole call aborts PERMISSION_DENIED. There is NO per-id
//     PERMISSION_DENIED on the wire.
//   - Per-id missing OR cross-tenant denied are MERGED into
//     missing_ids -- a deliberate information-leak guard so a foreign
//     actor cannot probe whether a node exists vs whether they lack
//     access (spec "Auth", Python grpc_server.py:1262-1271).
//   - The proto's `type_id` field is silently ignored (Python iterates
//     the ids and trusts whatever type_id is on disk; preserved for
//     parity, spec "Wire contract" #2).
//   - Duplicate node_ids are NOT deduped; each lookup runs
//     independently. Order of `nodes` follows request order minus
//     missing/denied (gaps closed, indexes are NOT parallel to
//     `node_ids`).
//   - `after_offset` fences the read against the applier (default
//     30s). Python's bare-except swallows a fence timeout as
//     "everything missing"; we preserve that behaviour for v1
//     parity (spec "Open Questions" #1).
//   - Bare-except in Python masks any internal error (SQLite, panic)
//     as nodes=[], missing_ids=<all input> with status OK. Preserved
//     verbatim: this masks data-loss bugs but is the documented
//     contract.
//
// Hardening delta vs Python (flagged in PR body for #407 owner):
//
//   - Batch-size cap: Python has no upper bound; a 100k-id payload
//     happily melts the SQLite pool and blows past gRPC's 4 MiB
//     default response size. The Go port aborts
//     RESOURCE_EXHAUSTED at len(node_ids) > maxBatchNodeIDs (1000)
//     BEFORE doing any I/O. Spec "Notes on performance & limits".
//
// No WAL append, no SQLite write, no audit row. Pure read.

package api

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/structpb"

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
	// with codes.RESOURCE_EXHAUSTED. Python has no such cap; this is
	// a Go-port hardening delta for issue #407.
	maxBatchNodeIDs = 1000

	// getNodesFanout is the bounded concurrency for per-id store
	// lookups. Caps in-flight SQLite reads at a small constant so a
	// 1000-id batch doesn't starve the pool. Behaviour-preserving
	// with respect to Python (which is serial); only a perf knob.
	getNodesFanout = 32

	// getNodesAfterOffsetDefault matches Python's 30s default fence
	// (grpc_server.py:1236-1240).
	getNodesAfterOffsetDefault = 30 * time.Second
)

// GetNodes implements entdb.v1.EntDBService/GetNodes. See file header
// for the full contract.
func (s *Server) GetNodes(ctx context.Context, req *pb.GetNodesRequest) (*pb.GetNodesResponse, error) {
	start := time.Now()
	resultStatus := "ok"
	defer func() {
		metrics.RecordGRPCRequest(getNodesMethod, resultStatus, time.Since(start))
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
	// caller cannot cause work to start. Python parity gap; flagged
	// in PR body.
	if len(req.GetNodeIds()) > maxBatchNodeIDs {
		resultStatus = "error"
		return nil, errs.Errorf(codes.ResourceExhausted,
			"GetNodes: batch size %d exceeds limit %d",
			len(req.GetNodeIds()), maxBatchNodeIDs)
	}

	// Empty node_ids -> empty response. Python returns the same shape
	// (the for-loop simply doesn't run).
	if len(req.GetNodeIds()) == 0 {
		return &pb.GetNodesResponse{
			Nodes:      []*pb.Node{},
			MissingIds: []string{},
		}, nil
	}

	// Cross-tenant role check (BATCH-LEVEL). Mirrors Python
	// _check_cross_tenant_read at grpc_server.py:561-618. The shared
	// helper isn't yet ported; we inline a parity-faithful version
	// here. Returns one of: roleLocal (no global), roleMember
	// (member or system actor), roleCrossTenant (non-member with
	// node_access grants), or PERMISSION_DENIED (non-member, no
	// grants).
	role, err := s.checkCrossTenantRead(ctx, tenantID, trusted)
	if err != nil {
		resultStatus = "error"
		return nil, err
	}

	// after_offset fence. Python wraps the call with a swallowing
	// try/except so a timeout is observed as "all missing". We
	// preserve that for v1 (spec "Open Questions" #1): a fence
	// failure is intentionally NOT a hard error here.
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
		// No per-tenant store wired -- preserve Python's bare-except
		// behaviour: everything missing, status OK, metric=error so
		// the swallow doesn't inflate the ok counter.
		resultStatus = "error"
		return &pb.GetNodesResponse{
			Nodes:      []*pb.Node{},
			MissingIds: append([]string{}, req.GetNodeIds()...),
		}, nil
	}

	ids := req.GetNodeIds()
	results := make([]*store.Node, len(ids))
	denied := make([]bool, len(ids))
	hadFatal := false
	var fatalMu sync.Mutex

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
					// Per-id panic -- treat as "missing", consistent
					// with Python's outer bare-except.
					fatalMu.Lock()
					hadFatal = true
					fatalMu.Unlock()
				}
			}()
			n, gerr := s.store.GetNode(ctx, tenantID, id)
			if gerr != nil || n == nil {
				return // miss -> nil entry -> missing_ids
			}
			if actorIDs != nil {
				ok, aerr := s.store.CanAccess(ctx, tenantID, id, actorIDs)
				if aerr != nil || !ok {
					denied[i] = true
					return
				}
			}
			results[i] = n
		}()
	}
	wg.Wait()

	if hadFatal {
		// At least one panic during the fan-out. Python's bare-except
		// returns "all missing"; the safest parity move when we can't
		// trust partial state is to do the same and flag the metric
		// as error.
		resultStatus = "error"
		return &pb.GetNodesResponse{
			Nodes:      []*pb.Node{},
			MissingIds: append([]string{}, ids...),
		}, nil
	}

	nodes := make([]*pb.Node, 0, len(ids))
	missing := make([]string, 0)
	for i, id := range ids {
		if results[i] == nil || denied[i] {
			missing = append(missing, id)
			continue
		}
		pn, perr := storeNodeToProto(s, results[i])
		if perr != nil {
			// Conversion failure on a single row -- fold into
			// missing_ids rather than failing the whole batch
			// (matches Python's "swallow and degrade" stance).
			missing = append(missing, id)
			continue
		}
		nodes = append(nodes, pn)
	}

	return &pb.GetNodesResponse{
		Nodes:      nodes,
		MissingIds: missing,
	}, nil
}

// crossTenantRole is the result of checkCrossTenantRead. Mirrors the
// "local" / "member" / "cross_tenant" string return of the Python
// helper at grpc_server.py:561-618.
type crossTenantRole int

const (
	roleLocal       crossTenantRole = iota // no global_store wired
	roleMember                             // tenant member or system actor
	roleCrossTenant                        // non-member with node_access grants
)

// checkCrossTenantRead is the Go port of
// server/python/entdb_server/api/grpc_server.py:_check_cross_tenant_read
// (561-618). Returns roleLocal / roleMember / roleCrossTenant on
// success, or a PERMISSION_DENIED status error on rejection.
//
// Inlined here (rather than in a shared helper) because GetNodes is
// the first read RPC to need it; subsequent read handlers (QueryNodes,
// GetEdgesFrom, …) will refactor this into internal/tenant once the
// surface stabilises.
func (s *Server) checkCrossTenantRead(ctx context.Context, tenantID string, actor auth.Actor) (crossTenantRole, error) {
	// No global_store wired -> "local" (backward-compat with bring-up
	// configs and unit tests; mirrors grpc_server.py:590-591).
	if s.global == nil {
		return roleLocal, nil
	}
	// System actors bypass all checks (grpc_server.py:594-595).
	if actor.IsSystem() {
		return roleMember, nil
	}
	// Tenant membership: resolve actor -> user_id and consult
	// globalstore.IsMember. Only user-kind actors can ever be members
	// (admins / unknown kinds are not in tenant_members rows; for
	// parity with Python we still try IsMember which falls through to
	// false for them).
	userID := actor.ID()
	if !actor.IsUser() {
		userID = actor.String()
	}
	if userID != "" {
		isMember, err := s.global.IsMember(ctx, tenantID, userID)
		if err == nil && isMember {
			return roleMember, nil
		}
	}
	// Not a member -- check for cross-tenant node_access grants.
	if s.store != nil {
		actorIDs, err := s.store.ResolveActorGroups(ctx, tenantID, actor.String())
		if err == nil && len(actorIDs) > 0 {
			has, herr := s.store.HasNodeAccess(ctx, tenantID, actorIDs)
			if herr == nil && has {
				return roleCrossTenant, nil
			}
		}
	}
	return 0, errs.Errorf(codes.PermissionDenied,
		"Actor is not a member of this tenant")
}

// storeNodeToProto converts a *store.Node (id-keyed payload JSON,
// raw ACL JSON) into a *pb.Node. Mirrors the inline conversion in
// Python's grpc_server.py:1273-1284.
//
// Schema-aware kind coercion: when the schema registry is wired, we
// look up the node type by type_id and feed payload.PayloadToStruct
// the type's name so BYTES / TIMESTAMP / INTEGER values are encoded
// correctly. Without a registry (unit tests), we fall through to the
// schema-less passthrough.
func storeNodeToProto(s *Server, n *store.Node) (*pb.Node, error) {
	// Parse payload (string-keyed by field_id) into map[string]any.
	var rawPayload map[string]any
	if n.PayloadJSON != "" {
		if err := json.Unmarshal([]byte(n.PayloadJSON), &rawPayload); err != nil {
			return nil, err
		}
	}
	// Build a *structpb.Struct with the same string keys. The wire
	// stays id-keyed per CLAUDE.md invariant #6.
	st, err := structpb.NewStruct(rawPayload)
	if err != nil {
		return nil, err
	}

	// Parse ACL JSON into proto entries.
	var aclList []store.ACLEntry
	if n.ACLJSON != "" {
		if err := json.Unmarshal([]byte(n.ACLJSON), &aclList); err != nil {
			return nil, err
		}
	}
	aclProto := make([]*pb.AclEntry, 0, len(aclList))
	for _, e := range aclList {
		aclProto = append(aclProto, &pb.AclEntry{
			Principal:  e.Principal,
			Permission: e.Permission,
		})
	}

	return &pb.Node{
		TenantId:   n.TenantID,
		NodeId:     n.NodeID,
		TypeId:     n.TypeID,
		CreatedAt:  n.CreatedAt,
		UpdatedAt:  n.UpdatedAt,
		OwnerActor: n.OwnerActor,
		Payload:    st,
		Acl:        aclProto,
	}, nil
}
