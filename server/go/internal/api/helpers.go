// SPDX-License-Identifier: AGPL-3.0-only

// Shared helpers for the api package. These were previously duplicated
// across the per-RPC handler files (e.g. get_user.go, list_users.go,
// add_tenant_member.go, change_member_role.go) by parallel PRs that
// landed on main without seeing each other. Consolidating them here
// keeps `go vet ./internal/api/...` clean and ensures every caller
// sees identical semantics.
//
// Conventions:
//   - Helpers that are pure Server methods (need s.global, s.sharding,
//     etc.) live as methods on *Server.
//   - Helpers that are pure wire-shape transforms live as free
//     functions and are nil-safe.

package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/elloloop/tenant-shard-db/server/go/internal/acl"
	"github.com/elloloop/tenant-shard-db/server/go/internal/apply"
	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
	"github.com/elloloop/tenant-shard-db/server/go/internal/payload"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

// userToProto maps a globalstore.User row to its proto wire form.
// NULL columns land as Go zero values via globalstore.User's
// plain-string fields, which marshal to the proto's empty-string
// defaults. A nil row yields a zero-value UserInfo so callers never
// panic on missing data (preserves the list_users.go pre-merge
// behaviour).
func userToProto(u *globalstore.User) *pb.UserInfo {
	if u == nil {
		return &pb.UserInfo{}
	}
	return &pb.UserInfo{
		UserId:    u.UserID,
		Email:     u.Email,
		Name:      u.Name,
		Status:    u.Status,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

// lookupMemberRole returns the role string for (tenantID, userID) or
// "" if no membership row exists. Empty userID short-circuits to ""
// so callers don't have to guard. O(N) in the tenant's member count
// to stay in lock-step with the Python _get_member_role implementation
// (grpc_server.py:2290-2296); a dedicated MemberRole helper in
// globalstore is tracked as a follow-up in the port spec.
//
// Errors from the globalstore are returned raw — call sites decide
// how to wrap them (e.g. as codes.Internal). This matches the
// add_tenant_member.go pre-merge contract; change_member_role.go's
// call site wraps explicitly.
func (s *Server) lookupMemberRole(ctx context.Context, tenantID, userID string) (string, error) {
	if userID == "" {
		return "", nil
	}
	members, err := s.global.GetTenantMembers(ctx, tenantID)
	if err != nil {
		return "", err
	}
	for _, m := range members {
		if m.UserID == userID {
			return m.Role, nil
		}
	}
	return "", nil
}

// readRole captures the outcome of the cross-tenant read-membership
// check. Mirrors the Python `_check_cross_tenant_read` sentinels.
//
// Consolidated from per-handler duplicates (get_node.go's readRole and
// get_nodes.go's crossTenantRole — same semantics, different names).
type readRole int

const (
	// roleLocal is the back-compat path when no global_store is wired.
	// Skips all membership checks; matches Python's
	// `if self.global_store is None: return "local"` (`:570-571`).
	roleLocal readRole = iota
	// roleMember means the caller is either a tenant member or a
	// system / admin actor. Same as Python `"member"` (`:582-589`).
	roleMember
	// roleCrossTenant means the caller is not a member but has at
	// least one node_access row in the tenant. The handler must then
	// re-check the specific node_id post-lookup. Same as Python
	// `"cross_tenant"` (`:597-617`).
	roleCrossTenant
)

// checkCrossTenantRead returns the caller's read role within tenantID.
//
// Resolution order:
//
//  1. global_store == nil => roleLocal (back-compat path).
//  2. system / admin actor => roleMember (bypass).
//  3. zero actor => PERMISSION_DENIED (defensive).
//  4. user IsMember => roleMember.
//  5. any non-deny, non-expired node_access grant (group-aware via
//     ResolveActorGroups) => roleCrossTenant.
//  6. otherwise => PERMISSION_DENIED.
//
// Consolidated from get_node.go (admin/zero handling) and get_nodes.go
// (group-aware grant resolution). Picks the union: broader scope on
// the grant check, defensive on the actor side.
func (s *Server) checkCrossTenantRead(ctx context.Context, tenantID string, a auth.Actor) (readRole, error) {
	if s.global == nil {
		return roleLocal, nil
	}
	if a.IsSystem() || a.IsAdmin() {
		return roleMember, nil
	}
	if a.IsZero() {
		return 0, errs.Errorf(codes.PermissionDenied,
			"actor lacks access to tenant %q", tenantID)
	}

	// Membership check. global_store stores user_ids by their bare
	// SDK identifier (e.g. "alice"), so user-kind actors consult
	// IsMember with the bare id. Non-user kinds (already filtered by
	// the admin/system branch above) fall through to the grant check.
	if a.IsUser() {
		ok, err := s.global.IsMember(ctx, tenantID, a.ID())
		if err != nil {
			return 0, fmt.Errorf("checkCrossTenantRead: IsMember: %w", err)
		}
		if ok {
			return roleMember, nil
		}
	}

	// Cross-tenant grant probe. Group-aware: ResolveActorGroups expands
	// the actor into the set of actor_ids (self + groups) that could
	// hold a node_access row; HasNodeAccess filters by non-deny / non-
	// expired in the per-tenant SQLite.
	if s.store != nil {
		actorIDs, err := s.store.ResolveActorGroups(ctx, tenantID, a.String())
		if err == nil && len(actorIDs) > 0 {
			has, herr := s.store.HasNodeAccess(ctx, tenantID, actorIDs)
			if herr == nil && has {
				return roleCrossTenant, nil
			}
		}
	}
	return 0, errs.Errorf(codes.PermissionDenied,
		"actor lacks access to tenant %q", tenantID)
}

// edgeToProto translates a store.Edge into its wire shape. PropsJSON
// is stored as a JSON-encoded map keyed by field IDs (CLAUDE.md
// invariant 6); we round-trip through map[string]any -> structpb so
// the wire `props` mirrors Python's `_dict_to_struct` output.
//
// Defensive shape: an empty / invalid props_json lowers to an empty
// Struct (never nil) so SDK consumers can rely on `edge.props.fields`
// being addressable without a nil check. Matches Python's
// `_dict_to_struct` zero-value behaviour at grpc_server.py:166.
func edgeToProto(e *store.Edge) *pb.Edge {
	return &pb.Edge{
		TenantId:   e.TenantID,
		EdgeTypeId: e.EdgeTypeID,
		FromNodeId: e.FromNodeID,
		ToNodeId:   e.ToNodeID,
		CreatedAt:  e.CreatedAt,
		Props:      edgePropsToStruct(e.PropsJSON),
	}
}

// edgePropsToStruct parses a stored props_json column into a typed
// Struct. Empty / invalid input lowers to an empty Struct, never nil:
// the Python wire shape always emits the field so SDK consumers can
// rely on `edge.props.fields` being addressable without a nil check.
func edgePropsToStruct(propsJSON string) *structpb.Struct {
	if propsJSON == "" {
		s, _ := structpb.NewStruct(map[string]any{})
		return s
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(propsJSON), &m); err != nil || m == nil {
		s, _ := structpb.NewStruct(map[string]any{})
		return s
	}
	s, err := structpb.NewStruct(m)
	if err != nil {
		s, _ = structpb.NewStruct(map[string]any{})
		return s
	}
	return s
}

// storeNodeToProto translates a store.Node into the wire pb.Node.
// Payload stays id-keyed on the wire per CLAUDE.md invariant #6; the
// SDK names the fields client-side. ACL JSON is unmarshalled to
// repeated AclEntry.
//
// typeName is consumed only by payload.PayloadToStruct for kind-aware
// coercion (BYTES -> base64, TIMESTAMP/INTEGER -> float64). Callers
// that have already resolved the type (QueryNodes) pass it directly;
// callers that operate on heterogeneous node sets (GetNodes,
// GetConnectedNodes) pass "" and accept the schema-less passthrough.
//
// Consolidated from three duplicates (get_nodes.go,
// get_connected_nodes.go, query_nodes.go). Picks the method form +
// payload.PayloadToStruct path so the schema-aware QueryNodes call
// site is preserved.
func (s *Server) storeNodeToProto(typeName string, n *store.Node) (*pb.Node, error) {
	idPayload, err := decodeIDKeyedPayload(n.PayloadJSON)
	if err != nil {
		return nil, errs.InternalNoCtx("parse node payload", err)
	}
	pStruct, err := payload.PayloadToStruct(s.registry, typeName, idPayload)
	if err != nil {
		return nil, err
	}
	aclEntries, err := decodeACLEntries(n.ACLJSON)
	if err != nil {
		return nil, errs.InternalNoCtx("parse node acl", err)
	}
	return &pb.Node{
		TenantId:   n.TenantID,
		NodeId:     n.NodeID,
		TypeId:     n.TypeID,
		CreatedAt:  n.CreatedAt,
		UpdatedAt:  n.UpdatedAt,
		OwnerActor: n.OwnerActor,
		Payload:    pStruct,
		Acl:        aclEntries,
	}, nil
}

// decodeIDKeyedPayload parses a raw JSON object whose keys are
// stringified field_ids ({"1":, ...}) into the uint32-keyed map the
// payload package consumes. Empty / null JSON yields an empty map.
// Non-digit keys are dropped silently (CLAUDE.md invariant #6 — the
// applier-side guard prevents new ones; we can't fix legacy rows on
// read).
func decodeIDKeyedPayload(raw string) (map[uint32]any, error) {
	out := map[uint32]any{}
	if raw == "" || raw == "null" {
		return out, nil
	}
	tmp := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &tmp); err != nil {
		return nil, err
	}
	for k, v := range tmp {
		id, ok := parsePayloadFieldID(k)
		if !ok {
			continue
		}
		out[id] = v
	}
	return out, nil
}

// decodeACLEntries parses the on-disk acl_blob
// ([{principal, permission}]) into the wire pb.AclEntry slice. Empty /
// null yields nil. Grantee mirrors Principal (the proto field was
// added for clarity; the SDK reads either).
func decodeACLEntries(raw string) ([]*pb.AclEntry, error) {
	if raw == "" || raw == "null" {
		return nil, nil
	}
	var list []store.ACLEntry
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		return nil, err
	}
	out := make([]*pb.AclEntry, 0, len(list))
	for _, e := range list {
		out = append(out, &pb.AclEntry{
			Principal:  e.Principal,
			Permission: e.Permission,
			Grantee:    e.Principal,
		})
	}
	return out, nil
}

// parsePayloadFieldID parses a digit-only string into a uint32 in the
// FieldDef bound (1-65535). Mirrors payload.parseFieldID — duplicated
// here because that helper is unexported.
func parsePayloadFieldID(s string) (uint32, bool) {
	if s == "" {
		return 0, false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
	}
	n, err := strconv.ParseUint(s, 10, 32)
	if err != nil || n == 0 || n > 65535 {
		return 0, false
	}
	return uint32(n), true
}

// authActorToACLActor bridges the auth.Actor (caller identity admitted
// by the interceptor) and the acl.Actor (ACL grant subject). user /
// system translate directly; admin maps to system because admins
// bypass ACL on the read path (matches Python's "admin can read
// everything" handler-level convention — expressing admin as system:
// here picks up the same bypass in acl.Filter.FilterReadable).
//
// Consolidated from query_nodes.go and get_connected_nodes.go. Picks
// the defensive form: IsZero short-circuits, admin -> system to
// preserve the read-path bypass.
func authActorToACLActor(a auth.Actor) acl.Actor {
	if a.IsZero() {
		return acl.Actor{}
	}
	switch a.Kind() {
	case auth.KindUser:
		return acl.User(a.ID())
	case auth.KindSystem:
		return acl.System(a.ID())
	case auth.KindAdmin:
		return acl.System(a.ID())
	default:
		// Service / unknown — fall through to user: form so the
		// filter has a non-system, non-empty actor.
		return acl.User(a.ID())
	}
}

// storeVisibilityAdapter satisfies acl.VisibilityReader by forwarding
// to CanonicalStore.GetVisibleNodeIDs. The store method is named
// GetVisibleNodeIDs (renamed from the Python _get_visible_nodes); the
// acl interface uses the unprefixed name. Both signatures are
// otherwise identical.
type storeVisibilityAdapter struct {
	s *store.CanonicalStore
}

// VisibleNodeIDs satisfies acl.VisibilityReader.
func (a storeVisibilityAdapter) VisibleNodeIDs(ctx context.Context, tenantID string, actorIDs []string, nodeIDs []string) (map[string]struct{}, error) {
	return a.s.GetVisibleNodeIDs(ctx, tenantID, actorIDs, nodeIDs)
}

// aclCheck runs an ACL check via acl.Checker.Check. Handlers that
// need ACL pre-checks (ShareNode, RevokeAccess, TransferOwnership, …)
// call this so the (Registry, Resolver, NodeMetaReader, GrantReader)
// wiring lives in one place.
//
// The Checker is constructed per call against s.store + s.global. The
// construction itself is cheap (NewRegistry / NewResolver allocate a
// couple of maps, the readers are pointer wrappers over the stores);
// no SQL is issued until Check actually walks grants.
//
// When WithEnforcer wires a process-wide Enforcer, future iterations
// can reuse its precomputed Registry/Resolver — for now the Enforcer
// is a marker that ACL wiring is present; the per-call construction
// reads the same store handles either way.
//
// Returns the Check result (nil on allow, errs.ErrPermission /
// errs.ErrNotFound otherwise). When s.store is unwired the function
// returns a sentinel error so handlers can short-circuit with their
// own contract-shaped response.
func (s *Server) aclCheck(ctx context.Context, req acl.CheckRequest) error {
	if s.store == nil {
		return errs.Errorf(codes.Unimplemented, "acl: canonical store not configured")
	}
	readers := &apply.StoreReaders{Canonical: s.store, Global: s.global}
	checker, err := acl.NewChecker(acl.CheckerOptions{
		Registry: acl.NewRegistry(),
		Resolver: acl.NewResolver(readers),
		Nodes:    readers,
		Grants:   readers,
	})
	if err != nil {
		return err
	}
	return checker.Check(ctx, req)
}

// newIdempotencyKey returns a 128-bit hex-encoded random idempotency
// key for a WAL Append. The producer's dedupe identity is
// (topic, key, idempotency_key); a fresh key per RPC means every
// distinct WAL-writing invocation lands a distinct record even when
// the natural (topic, key) pair repeats. Mirrors the
// uuid.uuid4().hex pattern Python uses at grpc_server.py:797 and
// elsewhere — only uniqueness is contractually pinned.
//
// Consolidated from three duplicates
// (remove_group_member.go, set_legal_hold.go, transfer_ownership.go).
// Picks the (string, error) signature; set_legal_hold's call site
// (previously string-only with a time-based fallback) now propagates
// the error along its best-effort WAL-append branch.
func newIdempotencyKey() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("idempotency key: %w", err)
	}
	return hex.EncodeToString(buf[:]), nil
}

const adminGlobalWaitAppliedTimeout = 30 * time.Second

type adminApplyFailure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// appendGlobalAdminOp appends one global-scope admin op and waits for
// the applier to materialize it into globalstore. Admin RPCs default to
// wait-applied semantics because their pre-WAL contract returned only
// after the global SQLite write committed.
func (s *Server) appendGlobalAdminOp(ctx context.Context, actor string, op map[string]any) (wal.StreamPos, string, error) {
	if s.producer == nil {
		return wal.StreamPos{}, "", errs.Errorf(codes.Unimplemented, "WAL producer not configured")
	}
	if s.store == nil {
		return wal.StreamPos{}, "", errs.Errorf(codes.Unimplemented, "Canonical store not configured")
	}
	idem, err := newIdempotencyKey()
	if err != nil {
		return wal.StreamPos{}, "", errs.Internal(ctx, "generate idempotency key", err)
	}
	ev := wal.Event{
		TenantID:       wal.GlobalTenantID,
		Scope:          wal.ScopeGlobal,
		Actor:          actor,
		IdempotencyKey: idem,
		TsMs:           time.Now().UnixMilli(),
		Ops:            []map[string]any{op},
	}
	value, err := ev.Encode()
	if err != nil {
		return wal.StreamPos{}, "", errs.Internal(ctx, "encode global admin event", err)
	}
	headers := map[string][]byte{
		wal.HeaderIdempotencyKey: []byte(idem),
	}
	pos, err := s.producer.Append(ctx, s.walTopic(), wal.GlobalTenantID, value, headers)
	if err != nil {
		return wal.StreamPos{}, "", errs.Internal(ctx, "append global admin event", err)
	}

	if err := s.waitForAdminApplied(ctx, wal.GlobalTenantID, pos.Offset, idem, "global admin event"); err != nil {
		return pos, idem, err
	}
	return pos, idem, nil
}

func (s *Server) waitForAdminApplied(ctx context.Context, tenantID string, offset int64, idem, label string) error {
	waitCtx, cancel := context.WithTimeout(ctx, adminGlobalWaitAppliedTimeout)
	defer cancel()
	if err := s.store.WaitForOffset(waitCtx, tenantID, offset); err != nil {
		return errs.Errorf(codes.DeadlineExceeded, "wait %s: %v", label, err)
	}
	rec := waitForIdempotencyRecord(ctx, s.store, tenantID, idem, adminGlobalWaitAppliedTimeout)
	if !rec.Present {
		return errs.Errorf(codes.DeadlineExceeded, "wait %s: idempotency record not visible", label)
	}
	if rec.Status == store.IdempotencyStatusFailedPrecondition {
		return adminApplyFailureError(label, rec.FailureJSON)
	}
	return nil
}

func adminApplyFailureError(label, failureJSON string) error {
	failure := adminApplyFailure{
		Code:    "FAILED_PRECONDITION",
		Message: "deterministic apply failure",
	}
	if failureJSON != "" {
		_ = json.Unmarshal([]byte(failureJSON), &failure)
	}
	if failure.Message == "" {
		failure.Message = "deterministic apply failure"
	}
	code := codes.FailedPrecondition
	if failure.Code == "ALREADY_EXISTS" {
		code = codes.AlreadyExists
	}
	return errs.Errorf(code, "%s: %s", label, failure.Message)
}
