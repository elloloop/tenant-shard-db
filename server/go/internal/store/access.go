package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// systemActor is the bootstrap/replay identity used by the applier.
// It bypasses every ACL check. Never appears on the wire.
const systemActor = "__system__"

// aclMaxDepth bounds recursive group-membership / acl_inherit
// expansion.
const aclMaxDepth = 10

// ResolveActorGroups expands an actor into the flat list
// [actor, group_1, group_2, ...] by recursively walking the
// group_users table.
//
// Cycle-safe: the SQL CTE bounds depth at aclMaxDepth.
//
// Used by read handlers in two places:
//
//  1. GetNodes / QueryNodes: the cross-tenant role gate calls this once
//     per request when the caller is non-member, then feeds the
//     expansion into HasNodeAccess + CanAccess.
//  2. ACL post-filter on visibility queries (GetVisibleNodeIDs,
//     ListSharedWithMe).
//
// The actor argument is the canonical "kind:id" form (e.g.
// "user:alice"). The returned slice always contains actor as its
// first element so callers can treat the empty-groups case as
// "self-only" without a special case.
func (s *CanonicalStore) ResolveActorGroups(ctx context.Context, tenantID, actor string) ([]string, error) {
	if actor == "" {
		return nil, nil
	}
	db, err := s.readDB(tenantID)
	if err != nil {
		return nil, err
	}
	q := fmt.Sprintf(`
		WITH RECURSIVE membership(gid, depth) AS (
			SELECT group_id, 0 FROM group_users
			WHERE member_actor_id = ?
			UNION ALL
			SELECT gu.group_id, m.depth + 1
			FROM group_users gu
			JOIN membership m ON gu.member_actor_id = m.gid
			WHERE m.depth < %d
		)
		SELECT DISTINCT gid FROM membership`, aclMaxDepth,
	)
	rows, err := db.QueryContext(ctx, q, actor)
	if err != nil {
		return nil, fmt.Errorf("store: ResolveActorGroups: %w", err)
	}
	defer rows.Close()
	out := []string{actor}
	for rows.Next() {
		var gid string
		if err := rows.Scan(&gid); err != nil {
			return nil, fmt.Errorf("store: ResolveActorGroups scan: %w", err)
		}
		out = append(out, gid)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: ResolveActorGroups rows: %w", err)
	}
	return out, nil
}

// HasNodeAccess reports whether at least one of actorIDs has a
// non-deny, non-expired node_access grant in the tenant.
//
// Used by the cross-tenant role gate in GetNodes / QueryNodes to
// decide whether a non-member caller has any reason to be reading
// from this tenant at all (vs. an outright PERMISSION_DENIED).
func (s *CanonicalStore) HasNodeAccess(ctx context.Context, tenantID string, actorIDs []string) (bool, error) {
	if len(actorIDs) == 0 {
		return false, nil
	}
	db, err := s.readDB(tenantID)
	if err != nil {
		return false, err
	}
	ph := strings.Repeat("?,", len(actorIDs))
	ph = ph[:len(ph)-1]
	q := fmt.Sprintf(`
		SELECT 1 FROM node_access
		WHERE actor_id IN (%s)
		  AND permission != 'deny'
		  AND (expires_at IS NULL OR expires_at > ?)
		LIMIT 1`, ph,
	)
	args := make([]any, 0, len(actorIDs)+1)
	for _, a := range actorIDs {
		args = append(args, a)
	}
	args = append(args, s.now())
	var one int
	row := db.QueryRowContext(ctx, q, args...)
	switch err := row.Scan(&one); err {
	case nil:
		return true, nil
	default:
		// sql.ErrNoRows -> false, no error. Other errors propagate.
		if isNoRows(err) {
			return false, nil
		}
		return false, fmt.Errorf("store: HasNodeAccess: %w", err)
	}
}

// CanAccess reports whether any of actorIDs can read nodeID per the
// owner / visibility / node_access / acl_inherit rules.
//
// Order:
//  1. The "__system__" actor short-circuits to true.
//  2. An explicit deny on the target node by any actor in actorIDs
//     wins -- returns false even if a sibling allow exists.
//  3. Otherwise walk the acl_inherit chain (depth-bounded) and at
//     each ancestor accept on owner-match, visibility-match (incl.
//     "tenant:*" wildcard), or non-deny non-expired node_access.
//
// Returns false if no path matches.
func (s *CanonicalStore) CanAccess(ctx context.Context, tenantID, nodeID string, actorIDs []string) (bool, error) {
	if len(actorIDs) == 0 {
		return false, nil
	}
	for _, a := range actorIDs {
		if a == systemActor {
			return true, nil
		}
	}
	db, err := s.readDB(tenantID)
	if err != nil {
		return false, err
	}

	actorPh := strings.Repeat("?,", len(actorIDs))
	actorPh = actorPh[:len(actorPh)-1]

	// Step 1: explicit deny check on the target node.
	denyQ := fmt.Sprintf(`
		SELECT 1 FROM node_access
		WHERE node_id = ? AND actor_id IN (%s)
		  AND permission = 'deny'
		LIMIT 1`, actorPh,
	)
	denyArgs := make([]any, 0, 1+len(actorIDs))
	denyArgs = append(denyArgs, nodeID)
	for _, a := range actorIDs {
		denyArgs = append(denyArgs, a)
	}
	var one int
	row := db.QueryRowContext(ctx, denyQ, denyArgs...)
	switch err := row.Scan(&one); err {
	case nil:
		return false, nil // explicit deny
	default:
		if !isNoRows(err) {
			return false, fmt.Errorf("store: CanAccess deny probe: %w", err)
		}
	}

	// Step 2: ancestry walk + owner / visibility / node_access checks.
	visIDs := make([]string, 0, len(actorIDs)+1)
	visIDs = append(visIDs, actorIDs...)
	visIDs = append(visIDs, "tenant:*")
	visPh := strings.Repeat("?,", len(visIDs))
	visPh = visPh[:len(visPh)-1]

	walkQ := fmt.Sprintf(`
		WITH RECURSIVE ancestry(nid, depth) AS (
			SELECT ?, 0
			UNION ALL
			SELECT ai.inherit_from, a.depth + 1
			FROM acl_inherit ai
			JOIN ancestry a ON ai.node_id = a.nid
			WHERE a.depth < %d
		)
		SELECT 1 FROM ancestry anc
		JOIN nodes n ON n.node_id = anc.nid AND n.tenant_id = ?
		WHERE (
			n.owner_actor IN (%s)
			OR EXISTS (
				SELECT 1 FROM node_visibility nv
				WHERE nv.tenant_id = ? AND nv.node_id = anc.nid
				  AND nv.principal IN (%s)
			)
			OR EXISTS (
				SELECT 1 FROM node_access na
				WHERE na.node_id = anc.nid
				  AND na.actor_id IN (%s)
				  AND na.permission != 'deny'
				  AND (na.expires_at IS NULL OR na.expires_at > ?)
			)
		)
		LIMIT 1`,
		aclMaxDepth, actorPh, visPh, actorPh,
	)
	walkArgs := make([]any, 0, 4+3*len(actorIDs)+len(visIDs))
	walkArgs = append(walkArgs, nodeID, tenantID)
	for _, a := range actorIDs {
		walkArgs = append(walkArgs, a)
	}
	walkArgs = append(walkArgs, tenantID)
	for _, v := range visIDs {
		walkArgs = append(walkArgs, v)
	}
	for _, a := range actorIDs {
		walkArgs = append(walkArgs, a)
	}
	walkArgs = append(walkArgs, s.now())

	row = db.QueryRowContext(ctx, walkQ, walkArgs...)
	switch err := row.Scan(&one); err {
	case nil:
		return true, nil
	default:
		if isNoRows(err) {
			return false, nil
		}
		return false, fmt.Errorf("store: CanAccess walk: %w", err)
	}
}

// isNoRows wraps errors.Is(err, sql.ErrNoRows) so callers don't need
// to import database/sql.
func isNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
