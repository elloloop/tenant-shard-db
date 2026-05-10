package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// ShareNodeInput is the input shape of ShareNode. Mirrors
// canonical_store.py:_sync_share_node (2962). Capability fields are
// optional; if CoreCaps is nil and Permission is set, the applier (W1.10)
// is responsible for back-filling — this package stores whatever it is
// given.
type ShareNodeInput struct {
	NodeID     string
	ActorID    string
	ActorType  string // "user" / "group", default "user"
	Permission string // legacy permission string ("read" / "write" / "admin")
	GrantedBy  string
	ExpiresAt  int64 // 0 == never
	TypeID     int32 // node type id (for typed-capability lookups)
	CoreCaps   []int32
	ExtCapIDs  []int32
}

// ShareNode upserts an ACL grant for (node_id, actor_id). Mirrors
// canonical_store.py:_sync_share_node (2962). INSERT OR REPLACE
// semantics — re-sharing overwrites the prior grant.
func (s *CanonicalStore) ShareNode(ctx context.Context, tenantID string, in ShareNodeInput) error {
	if in.NodeID == "" || in.ActorID == "" {
		return fmt.Errorf("store: ShareNode: node_id and actor_id required")
	}
	actorType := in.ActorType
	if actorType == "" {
		actorType = "user"
	}
	now := s.now()
	coreCapsJSON, err := json.Marshal(intsOrEmpty(in.CoreCaps))
	if err != nil {
		return fmt.Errorf("store: marshal core_caps: %w", err)
	}
	extCapsJSON, err := json.Marshal(intsOrEmpty(in.ExtCapIDs))
	if err != nil {
		return fmt.Errorf("store: marshal ext_caps: %w", err)
	}
	var expires sql.NullInt64
	if in.ExpiresAt > 0 {
		expires = sql.NullInt64{Int64: in.ExpiresAt, Valid: true}
	}
	return s.withWrite(ctx, tenantID, func(conn *sql.Conn) error {
		_, err := conn.ExecContext(ctx, `
			INSERT OR REPLACE INTO node_access
			    (node_id, actor_id, actor_type, permission, granted_by,
			     granted_at, expires_at, type_id, core_caps_json, ext_cap_ids_json)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			in.NodeID, in.ActorID, actorType, in.Permission, in.GrantedBy,
			now, expires, in.TypeID, string(coreCapsJSON), string(extCapsJSON),
		)
		if err != nil {
			return fmt.Errorf("store: insert node_access: %w", err)
		}
		return nil
	})
}

// RevokeAccess removes the (node_id, actor_id) ACL grant. Returns
// nil even if no grant existed (idempotent revoke). Mirrors
// canonical_store.py:_sync_revoke_access (3240).
func (s *CanonicalStore) RevokeAccess(ctx context.Context, tenantID, nodeID, actorID string) (bool, error) {
	var existed bool
	err := s.withWrite(ctx, tenantID, func(conn *sql.Conn) error {
		res, err := conn.ExecContext(ctx,
			`DELETE FROM node_access WHERE node_id = ? AND actor_id = ?`,
			nodeID, actorID,
		)
		if err != nil {
			return fmt.Errorf("store: revoke access: %w", err)
		}
		n, _ := res.RowsAffected()
		existed = n > 0
		return nil
	})
	return existed, err
}

// DelegateAccess is conceptually identical to ShareNode (insert into
// node_access) but with the delegating actor recorded as granted_by.
// Mirrors canonical_store.py:_sync_delegate_access (3774). The semantic
// distinction (an actor can only delegate caps they themselves have)
// is enforced in W1.9 acl, not here.
func (s *CanonicalStore) DelegateAccess(ctx context.Context, tenantID string, in ShareNodeInput) error {
	return s.ShareNode(ctx, tenantID, in)
}

// TransferOwnership reassigns owner_actor on a node and refreshes the
// node_visibility index. Mirrors canonical_store.py:_sync_transfer_ownership
// (3631). Returns ErrNodeNotFound if the node does not exist.
func (s *CanonicalStore) TransferOwnership(ctx context.Context, tenantID, nodeID, newOwner string) error {
	if newOwner == "" {
		return fmt.Errorf("store: TransferOwnership: new_owner required")
	}
	return s.withWrite(ctx, tenantID, func(conn *sql.Conn) error {
		// Read existing ACL so visibility refresh keeps explicit shares.
		row := conn.QueryRowContext(ctx,
			`SELECT acl_blob FROM nodes WHERE tenant_id = ? AND node_id = ?`,
			tenantID, nodeID,
		)
		var aclJSON string
		if err := row.Scan(&aclJSON); err != nil {
			if err == sql.ErrNoRows {
				return fmt.Errorf("%w: %s/%s", ErrNodeNotFound, tenantID, nodeID)
			}
			return fmt.Errorf("store: read acl: %w", err)
		}
		var acl []ACLEntry
		if aclJSON != "" {
			if err := json.Unmarshal([]byte(aclJSON), &acl); err != nil {
				// Best-effort: visibility refresh proceeds with empty ACL.
				acl = nil
			}
		}
		res, err := conn.ExecContext(ctx,
			`UPDATE nodes SET owner_actor = ? WHERE tenant_id = ? AND node_id = ?`,
			newOwner, tenantID, nodeID,
		)
		if err != nil {
			return fmt.Errorf("store: update owner: %w", err)
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return fmt.Errorf("%w: %s/%s", ErrNodeNotFound, tenantID, nodeID)
		}
		return updateVisibilityWithConn(ctx, conn, tenantID, nodeID, newOwner, acl)
	})
}

// RevokeUserAccess deletes every node_access grant + group_users
// membership for a given user_id, plus their visibility rows. Mirrors
// canonical_store.py:_sync_revoke_user_access (3871). Returns the count
// of (revoked grants, revoked group memberships).
func (s *CanonicalStore) RevokeUserAccess(ctx context.Context, tenantID, userID string) (revokedGrants, revokedGroups int64, err error) {
	err = s.withWrite(ctx, tenantID, func(conn *sql.Conn) error {
		gres, err := conn.ExecContext(ctx,
			`DELETE FROM node_access WHERE actor_id = ?`, userID,
		)
		if err != nil {
			return fmt.Errorf("store: revoke grants: %w", err)
		}
		revokedGrants, _ = gres.RowsAffected()
		mres, err := conn.ExecContext(ctx,
			`DELETE FROM group_users WHERE member_actor_id = ?`, userID,
		)
		if err != nil {
			return fmt.Errorf("store: revoke groups: %w", err)
		}
		revokedGroups, _ = mres.RowsAffected()
		if _, err := conn.ExecContext(ctx,
			`DELETE FROM node_visibility WHERE tenant_id = ? AND principal = ?`,
			tenantID, userID,
		); err != nil {
			return fmt.Errorf("store: revoke visibility: %w", err)
		}
		return nil
	})
	return
}

// AddGroupMember inserts (or replaces) a (group_id, member_actor_id)
// row. Mirrors canonical_store.py:_sync_add_group_member (3497).
func (s *CanonicalStore) AddGroupMember(ctx context.Context, tenantID, groupID, memberActorID, role string) error {
	if groupID == "" || memberActorID == "" {
		return fmt.Errorf("store: AddGroupMember: group_id and member_actor_id required")
	}
	if role == "" {
		role = "member"
	}
	now := s.now()
	return s.withWrite(ctx, tenantID, func(conn *sql.Conn) error {
		_, err := conn.ExecContext(ctx, `
			INSERT OR REPLACE INTO group_users
			    (group_id, member_actor_id, role, joined_at)
			VALUES (?, ?, ?, ?)`,
			groupID, memberActorID, role, now,
		)
		if err != nil {
			return fmt.Errorf("store: add group member: %w", err)
		}
		return nil
	})
}

// RemoveGroupMember deletes the (group_id, member_actor_id) row.
// Returns true if the row existed. Mirrors canonical_store.py:
// _sync_remove_group_member (3533).
func (s *CanonicalStore) RemoveGroupMember(ctx context.Context, tenantID, groupID, memberActorID string) (bool, error) {
	var existed bool
	err := s.withWrite(ctx, tenantID, func(conn *sql.Conn) error {
		res, err := conn.ExecContext(ctx,
			`DELETE FROM group_users WHERE group_id = ? AND member_actor_id = ?`,
			groupID, memberActorID,
		)
		if err != nil {
			return fmt.Errorf("store: remove group member: %w", err)
		}
		n, _ := res.RowsAffected()
		existed = n > 0
		return nil
	})
	return existed, err
}

func intsOrEmpty(s []int32) []int32 {
	if s == nil {
		return []int32{}
	}
	return s
}
