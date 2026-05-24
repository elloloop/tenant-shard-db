// SPDX-License-Identifier: AGPL-3.0-only

package apply

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/elloloop/tenant-shard-db/server/go/internal/acl"
	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

// StoreReaders is the adapter that fulfils every reader interface the
// acl package declares (NodeMetaReader, GrantReader,
// CrossTenantGrantReader, VisibilityReader, GroupMembershipReader),
// backed by store.CanonicalStore + globalstore.GlobalStore.
//
// The applier owns the read-after-write consistency story — the same
// adapter is used by the gRPC read RPCs.
//
// All methods are safe for concurrent goroutine use to the extent the
// underlying stores are.
type StoreReaders struct {
	Canonical *store.CanonicalStore
	Global    *globalstore.GlobalStore
}

// Compile-time assertions that StoreReaders satisfies every acl reader
// contract. Missing methods would break wire-up, so we want the
// build to fail here, not later.
var (
	_ acl.NodeMetaReader         = (*StoreReaders)(nil)
	_ acl.GrantReader            = (*StoreReaders)(nil)
	_ acl.CrossTenantGrantReader = (*StoreReaders)(nil)
	_ acl.VisibilityReader       = (*StoreReaders)(nil)
	_ acl.GroupMembershipReader  = (*StoreReaders)(nil)
)

// NodeMeta implements acl.NodeMetaReader. Returns errs.ErrNotFound when
// the node is missing (matches the acl package's expected sentinel —
// see acl/checker.go NodeMetaReader contract).
func (r *StoreReaders) NodeMeta(ctx context.Context, tenantID, nodeID string) (acl.NodeMeta, error) {
	if r.Canonical == nil {
		return acl.NodeMeta{}, fmt.Errorf("apply: NodeMeta: canonical store not configured")
	}
	n, err := r.Canonical.GetNode(ctx, tenantID, nodeID)
	if err != nil {
		// store.ErrNodeNotFound wraps errs.ErrNotFound; pass through.
		return acl.NodeMeta{}, err
	}
	return acl.NodeMeta{OwnerActor: n.OwnerActor, TypeID: n.TypeID}, nil
}

// GrantsForNode implements acl.GrantReader. Returns the ACL grants
// stored on a single node, decoded from node_access plus the legacy
// acl_blob column (the per-node ACL list). Expired rows are NOT
// filtered here — the acl.Checker drops them.
func (r *StoreReaders) GrantsForNode(ctx context.Context, tenantID, nodeID string) ([]acl.Grant, error) {
	if r.Canonical == nil {
		return nil, fmt.Errorf("apply: GrantsForNode: canonical store not configured")
	}
	db, err := getCanonicalDB(r.Canonical, tenantID)
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, `
		SELECT actor_id, actor_type, permission, granted_by, granted_at,
		       expires_at, type_id, core_caps_json, ext_cap_ids_json
		FROM node_access WHERE node_id = ?`,
		nodeID,
	)
	if err != nil {
		return nil, fmt.Errorf("apply: query node_access: %w", err)
	}
	defer rows.Close()
	var out []acl.Grant
	for rows.Next() {
		var (
			actorID, actorType, permStr, grantedBy string
			grantedAt                              int64
			expires                                sql.NullInt64
			typeID                                 int32
			coreJSON, extJSON                      string
		)
		if err := rows.Scan(&actorID, &actorType, &permStr, &grantedBy, &grantedAt,
			&expires, &typeID, &coreJSON, &extJSON); err != nil {
			return nil, fmt.Errorf("apply: scan node_access: %w", err)
		}
		g := acl.Grant{
			Subject:   parseActor(actorType, actorID),
			NodeID:    nodeID,
			TypeID:    typeID,
			GrantedBy: parseActorString(grantedBy),
			GrantedAt: grantedAt,
		}
		// acl.PermDeny is the zero value of Permission, which makes
		// IsDeny() flag any grant whose permission column is empty /
		// unparseable as an explicit DENY — wrong for typed-cap-only
		// rows (the ShareNode handler accepts an empty permission when
		// CoreCaps is set). Default to PermRead for non-deny rows so
		// the deny-pass in acl.Checker doesn't fire on a well-formed
		// typed-cap grant. An unset permission column never means
		// "deny" — only the literal "deny" string does.
		if perm, ok := acl.ParsePermission(permStr); ok {
			g.Permission = perm
		} else {
			g.Permission = acl.PermRead
		}
		if expires.Valid {
			ms := expires.Int64
			g.ExpiresAt = &ms
		}
		// Decode typed-cap arrays.
		// Fail closed on a corrupt typed-cap array (#582): silently
		// dropping caps would mis-evaluate access. Surface the error so
		// the ACL read fails rather than returning a wrong grant set.
		if coreJSON != "" {
			var raw []int32
			if err := json.Unmarshal([]byte(coreJSON), &raw); err != nil {
				return nil, fmt.Errorf("apply: GrantsForNode: decode core_caps for %s/%s: %w", tenantID, nodeID, err)
			}
			for _, v := range raw {
				g.CoreCaps = append(g.CoreCaps, acl.CoreCapability(v))
			}
		}
		if extJSON != "" {
			var raw []int32
			if err := json.Unmarshal([]byte(extJSON), &raw); err != nil {
				return nil, fmt.Errorf("apply: GrantsForNode: decode ext_caps for %s/%s: %w", tenantID, nodeID, err)
			}
			for _, v := range raw {
				g.ExtCapIDs = append(g.ExtCapIDs, acl.ExtCapID(v))
			}
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("apply: iterate node_access: %w", err)
	}
	return out, nil
}

// CrossTenantGrant implements acl.CrossTenantGrantReader. Looks up the
// shared_index hint for (sourceTenant, nodeID, foreignActor) and
// returns the typed permission; maps any hit to PermRead per
// the acl package spec.
func (r *StoreReaders) CrossTenantGrant(ctx context.Context, sourceTenant, nodeID, foreignActor string) (acl.Permission, bool, error) {
	if r.Global == nil {
		return acl.PermDeny, false, nil
	}
	// foreignActor is "kind:id"; the shared_index keys on the bare
	// user_id (no kind prefix). Strip the "user:" prefix when present.
	userID := foreignActor
	if len(userID) > 5 && userID[:5] == "user:" {
		userID = userID[5:]
	}
	entries, err := r.Global.ListSharedFromNode(ctx, sourceTenant, nodeID)
	if err != nil {
		return acl.PermDeny, false, fmt.Errorf("apply: cross-tenant grant: %w", err)
	}
	for _, e := range entries {
		if e.UserID == userID {
			perm, _ := acl.ParsePermission(e.Permission)
			if perm == acl.PermDeny {
				perm = acl.PermRead
			}
			return perm, true, nil
		}
	}
	return acl.PermDeny, false, nil
}

// VisibleNodeIDs implements acl.VisibilityReader. Pure pass-through to
// store.GetVisibleNodeIDs.
func (r *StoreReaders) VisibleNodeIDs(ctx context.Context, tenantID string, actorIDs []string, nodeIDs []string) (map[string]struct{}, error) {
	if r.Canonical == nil {
		return nil, fmt.Errorf("apply: VisibleNodeIDs: canonical store not configured")
	}
	return r.Canonical.GetVisibleNodeIDs(ctx, tenantID, actorIDs, nodeIDs)
}

// GroupsContaining implements acl.GroupMembershipReader. Reads the
// group_users table and returns the bare group ids that contain
// memberActorID as a direct member.
func (r *StoreReaders) GroupsContaining(ctx context.Context, tenantID, memberActorID string) ([]string, error) {
	if r.Canonical == nil {
		return nil, fmt.Errorf("apply: GroupsContaining: canonical store not configured")
	}
	db, err := getCanonicalDB(r.Canonical, tenantID)
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx,
		`SELECT group_id FROM group_users WHERE member_actor_id = ?`,
		memberActorID,
	)
	if err != nil {
		return nil, fmt.Errorf("apply: query group_users: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var gid string
		if err := rows.Scan(&gid); err != nil {
			return nil, fmt.Errorf("apply: scan group_users: %w", err)
		}
		out = append(out, gid)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("apply: iterate group_users: %w", err)
	}
	return out, nil
}

// getCanonicalDB returns the per-tenant *sql.DB after ensuring the
// tenant is open. Used by acl reads (GrantsForNode / GroupsContaining)
// that don't have a dedicated public helper on store.CanonicalStore.
func getCanonicalDB(s *store.CanonicalStore, tenantID string) (*sql.DB, error) {
	if err := s.OpenTenant(context.Background(), tenantID); err != nil {
		return nil, fmt.Errorf("apply: open tenant for ad-hoc read: %w", err)
	}
	return s.AdminDB(tenantID)
}

// parseActor builds an acl.Actor from a (kind, id) pair. The id may
// be either bare ("alice") or already prefixed ("user:alice") — both
// shapes appear in node_access depending on how the row was authored
// (ShareNode handler normalises to "user:alice", legacy paths and
// group rows use the bare form). We strip a leading "<kind>:" so the
// returned Actor has a clean ID and its String() matches the form
// used elsewhere in the acl package.
func parseActor(kind, id string) acl.Actor {
	id = stripKindPrefix(kind, id)
	switch kind {
	case "group":
		return acl.Group(id)
	case "service":
		return acl.Service(id)
	default:
		return acl.User(id)
	}
}

// stripKindPrefix removes a leading "<kind>:" from id, if present.
// Returns id unchanged when no matching prefix exists.
func stripKindPrefix(kind, id string) string {
	if kind == "" || id == "" {
		return id
	}
	prefix := kind + ":"
	if len(id) > len(prefix) && id[:len(prefix)] == prefix {
		return id[len(prefix):]
	}
	return id
}

// parseActorString builds an acl.Actor from a "kind:id" string. Falls
// back to user: when no prefix is present.
func parseActorString(s string) acl.Actor {
	for i, c := range s {
		if c == ':' {
			return parseActor(s[:i], s[i+1:])
		}
	}
	if s == "" {
		return acl.Actor{}
	}
	return acl.User(s)
}
