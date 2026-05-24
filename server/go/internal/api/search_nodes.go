// SearchNodes RPC.
// Spec: docs/go-port/rpcs/SearchNodes.md.
//
// Wire contract: proto/entdb/v1/entdb.proto:148 (rpc), :1124-1136
// (messages).
//
// Semantics:
//
//   - Read-only. No WAL append. Per-tenant FTS5 virtual tables hold the
//     materialised search index; the applier maintains them on each
//     CreateNode/UpdateNode/DeleteNode write. Cross-tenant search is
//     impossible by construction (per-tenant SQLite isolation, CLAUDE.md
//     invariant 4).
//   - Auth: trusted-actor via auth.Authoritative. The wire `actor` is
//     UNTRUSTED — the interceptor-populated Identity wins. No pre-FTS
//     authz: anyone whose tenant gate passes can MATCH against the FTS
//     index. The privacy boundary is the per-row ACL post-filter, not a
//     coarse RPC-level check.
//   - ACL post-filter is applied ONLY when the actor is classified
//     "cross_tenant" (i.e. not a tenant member, not a system identity).
//     In-tenant members see the unfiltered FTS result set. Tightening
//     this requires its own decision doc.
//   - Order of operations: tenant -> validate query -> searchable
//     lookup -> SQL -> ACL trim -> proto convert. Swapping any two
//     breaks at least one contract test.
//
// Error contract:
//
//   - tenant gate fails -> propagated (UNAVAILABLE / FAILED_PRECONDITION / NOT_FOUND).
//   - empty query (after Trim) -> codes.InvalidArgument "query must not be empty".
//   - len(query) > 1000 -> codes.InvalidArgument "query must be under 1000 characters".
//   - type with no searchable -> codes.OK with nodes: [] (no SQL).
//   - FTS5 / SQLite errors -> codes.OK with nodes: [] (swallow-to-empty,
//     see spec lines 63-66). The Go port preserves this for v1; flipping
//     to INTERNAL is a future contract-test update, not this PR.
//
// Payload egress: Node.payload is emitted as a *structpb.Struct whose
// keys are the field-id strings ("1","2"...). Translation to field
// names is the SDK's job (CLAUDE.md invariant 6 + spec line 30).

package api

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"github.com/elloloop/tenant-shard-db/server/go/internal/metrics"
	"github.com/elloloop/tenant-shard-db/server/go/internal/payload"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

const grpcMethodSearchNodes = "SearchNodes"

// maxQueryLen caps the query length before issuing SQL.
const maxQueryLen = 1000

// SearchNodes implements entdb.v1.EntDBService/SearchNodes.
func (s *Server) SearchNodes(
	ctx context.Context, req *pb.SearchNodesRequest,
) (*pb.SearchNodesResponse, error) {
	start := time.Now()
	outcome := "ok"
	defer func() {
		metrics.RecordGRPCRequest(grpcMethodSearchNodes, outcome, time.Since(start))
	}()

	// 1. Tenant gate (UNAVAILABLE / FAILED_PRECONDITION / NOT_FOUND /
	//    InvalidArgument for empty tenant_id).
	tenantID := req.GetTenantId()
	if err := s.checkTenant(ctx, tenantID); err != nil {
		outcome = "error"
		return nil, err
	}

	// 2. Validate query before touching storage. INVALID_ARGUMENT is
	//    pinned by tests/python/integration/test_grpc_contract.py:627-632.
	q := strings.TrimSpace(req.GetQuery())
	if q == "" {
		outcome = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "query must not be empty")
	}
	if len(q) > maxQueryLen {
		outcome = "error"
		return nil, errs.Errorf(codes.InvalidArgument, "query must be under 1000 characters")
	}

	// 3. Schema lookup. An empty searchable set short-circuits to empty
	//    response WITHOUT issuing SQL. This is a load-bearing contract
	//    (test_grpc_contract.py:633-641).
	typeID := req.GetTypeId()
	var searchableFIDs []uint32
	if s.registry != nil {
		searchableFIDs = s.registry.SearchableFieldIDs(typeID)
	}
	if len(searchableFIDs) == 0 {
		return &pb.SearchNodesResponse{Nodes: []*pb.Node{}}, nil
	}

	// 4. Run the FTS5 JOIN. Lazy-CREATE happens inside store.SearchNodes.
	//    Any error from the FTS layer (malformed FTS5 syntax, SQLite I/O,
	//    etc.) is swallowed and reported as an empty result with codes.OK.
	//    See spec lines 63-66.
	if s.store == nil {
		// No store wired — empty result, codes.OK.
		return &pb.SearchNodesResponse{Nodes: []*pb.Node{}}, nil
	}

	// Open the per-tenant view before searching. The SQLite store is a
	// materialized view of the WAL (ADR-016); "tenant not opened" means
	// the applier has not materialized this tenant in-process yet, not a
	// client error. Lazy-open it (as GetNode does) so a valid tenant is
	// not silently reported as zero hits. A genuine open failure (region
	// pin / crypto-shred -> FailedPrecondition; IO -> Internal) surfaces
	// its real typed code; only post-open FTS faults fall through to the
	// best-effort swallow below.
	if err := s.store.OpenTenant(ctx, tenantID); err != nil {
		outcome = "error"
		return nil, errs.Errorf(errs.Code(err), "SearchNodes: open tenant: %v", err)
	}

	limit := int(req.GetLimit())
	if limit == 0 {
		limit = 50
	}
	// SEC-4 (#135): cap oversized page requests before the FTS JOIN
	// materialises rows.
	limit = clampPageSize(limit)
	offset := int(req.GetOffset())

	rows, ferr := s.store.SearchNodes(ctx, tenantID, typeID, q, searchableFIDs, limit, offset)
	if ferr != nil {
		outcome = "error"
		// A malformed FTS5 MATCH query is a CLIENT error: surface it as
		// InvalidArgument so the caller learns the query was bad, rather
		// than the old empty+OK that masqueraded as "no matches" (#573).
		if isFTSQueryError(ferr) {
			return nil, errs.Errorf(codes.InvalidArgument, "SearchNodes: malformed query: %v", ferr)
		}
		// Otherwise this is a genuine post-open fault (IO, corruption,
		// scan) — surface it. Preserve typed sentinels; sanitize the rest.
		if c := errs.Code(ferr); c != codes.Unknown {
			return nil, errs.Errorf(c, "SearchNodes: %v", ferr)
		}
		return nil, errs.Internal(ctx, "SearchNodes: store", ferr)
	}

	// 5. ACL post-filter — only when the actor is classified
	//    "cross_tenant". In-tenant members and system actors see the
	//    unfiltered set (see spec line 42).
	trusted := auth.Authoritative(ctx, auth.ParseActor(req.GetActor()))
	if s.isCrossTenantReader(ctx, tenantID, trusted) {
		rows = filterNodesByActor(rows, trusted)
	}

	// 6. Convert to proto. Payload Struct is field-id-keyed verbatim.
	out := make([]*pb.Node, 0, len(rows))
	for _, n := range rows {
		pn, perr := nodeRowToProto(n)
		if perr != nil {
			// A row that will not marshal is a corrupt stored payload, not
			// an absent result — surfacing it (#573) beats silently dropping
			// the row from the result set.
			outcome = "error"
			return nil, errs.Internal(ctx, "SearchNodes: marshal row", perr)
		}
		out = append(out, pn)
	}
	return &pb.SearchNodesResponse{Nodes: out}, nil
}

// isCrossTenantReader returns true when trusted is NOT a member of
// tenantID and is NOT a system/admin identity. A cross-tenant actor
// without any node_access grants will have the ACL post-filter drop
// everything they cannot see, producing an empty result set.
//
// When global_store is not configured, returns false (no filter).
func (s *Server) isCrossTenantReader(ctx context.Context, tenantID string, trusted auth.Actor) bool {
	if s.global == nil {
		return false
	}
	if trusted.IsSystem() || trusted.IsAdmin() {
		return false
	}
	if trusted.IsZero() {
		// No identity at all — treat as cross-tenant so the ACL filter
		// drops everything not explicitly shared, producing an empty
		// response via the outer swallow.
		return true
	}
	member, err := s.global.IsMember(ctx, tenantID, trusted.ID())
	if err != nil {
		// Membership probe failure: be conservative — treat as
		// cross-tenant. The downstream ACL filter is purely additive
		// (drops rows), so this is a safe default.
		return true
	}
	return !member
}

// filterNodesByActor is the per-row "can_access" approximation used by
// the cross-tenant branch. A row is kept when:
//
//   - the row's owner_actor matches the trusted actor's wire form, OR
//   - any ACL entry on the row grants the trusted actor (matching the
//     legacy `principal` field or the newer `grantee`).
//
// This is the minimal can_access shape; the full group-expansion +
// capability-typed grant evaluation lives in internal/acl and will be
// wired in a later change (see spec "Open questions / risks: ACL trim
// cost"). An actor with no matching grants sees zero rows.
func filterNodesByActor(rows []*store.Node, trusted auth.Actor) []*store.Node {
	if len(rows) == 0 {
		return rows
	}
	actorWire := trusted.String()
	out := rows[:0]
	for _, n := range rows {
		if n == nil {
			continue
		}
		if actorWire != "" && n.OwnerActor == actorWire {
			out = append(out, n)
			continue
		}
		if aclMatches(n.ACLJSON, actorWire) {
			out = append(out, n)
		}
	}
	return out
}

// aclMatches reports whether actorWire is granted on the node by the
// ACL JSON blob. Empty actor never matches (unlike the empty-grantee
// pun in some legacy rows).
func aclMatches(aclJSON, actorWire string) bool {
	if aclJSON == "" || actorWire == "" {
		return false
	}
	var entries []store.ACLEntry
	if err := json.Unmarshal([]byte(aclJSON), &entries); err != nil {
		return false
	}
	for _, e := range entries {
		if e.Principal == actorWire {
			return true
		}
	}
	return false
}

// nodeRowToProto converts a store.Node row into a wire pb.Node. The
// payload Struct keys are the field-id strings exactly as stored —
// no name translation happens server-side (CLAUDE.md invariant 6).
//
// JSON unmarshalling errors propagate up so the caller can drop the
// row from the response (see SearchNodes, "swallow" outer behavior).
func nodeRowToProto(n *store.Node) (*pb.Node, error) {
	if n == nil {
		return nil, nil
	}
	out := &pb.Node{
		TenantId:   n.TenantID,
		NodeId:     n.NodeID,
		TypeId:     n.TypeID,
		CreatedAt:  n.CreatedAt,
		UpdatedAt:  n.UpdatedAt,
		OwnerActor: n.OwnerActor,
	}

	idKeyed, err := decodeIDKeyedPayload(n.PayloadJSON)
	if err != nil {
		return nil, err
	}
	if len(idKeyed) > 0 {
		// Schema-less (no registry here): canonical decode keeps int64 in
		// typed_payload; Struct payload stays float64-lossy by design.
		st, serr := payload.PayloadToStruct(nil, "", idKeyed)
		if serr != nil {
			return nil, serr
		}
		out.Payload = st
		typed, terr := payload.PayloadToTyped(nil, "", idKeyed)
		if terr != nil {
			return nil, terr
		}
		out.TypedPayload = typed
	}

	if n.ACLJSON != "" {
		var entries []store.ACLEntry
		if err := json.Unmarshal([]byte(n.ACLJSON), &entries); err == nil {
			for _, e := range entries {
				out.Acl = append(out.Acl, &pb.AclEntry{
					Principal:  e.Principal,
					Permission: e.Permission,
				})
			}
		}
	}

	return out, nil
}

// isFTSQueryError reports whether a store.SearchNodes error came from an
// invalid FTS5 MATCH expression (a client-query problem) rather than a
// genuine storage fault. FTS5 reports query-syntax problems with stable
// message signatures; a real IO/corruption error does not match these,
// so it falls through to the Internal path (#573). String inspection is
// the only seam — modernc/SQLite does not return a distinct error code
// for FTS5 syntax errors.
func isFTSQueryError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, sig := range []string{
		"fts5: syntax error",
		"fts5: ",
		"unterminated string",
		"malformed match",
		"unknown special query",
		"no such column", // column-filtered MATCH against an unknown column
	} {
		if strings.Contains(msg, sig) {
			return true
		}
	}
	return false
}
