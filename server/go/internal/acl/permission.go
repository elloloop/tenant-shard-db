// SPDX-License-Identifier: AGPL-3.0-only

// Package acl is the Go port of server/python/entdb_server/apply/acl.py
// plus server/python/entdb_server/auth/capability_registry.py.
//
// Two parallel models live here:
//
//   - Permission: the legacy 7-value enum still on the wire
//     (READ/COMMENT/WRITE/SHARE/DELETE/ADMIN/DENY). Source of truth:
//     server/python/entdb_server/apply/acl.py:32-42 and the
//     PERMISSION_HIERARCHY table at acl.py:190-210.
//   - CoreCapability + ExtCapID: the typed capability model frozen in
//     docs/decisions/acl.md (2026-04-13). New writes carry typed caps;
//     legacy strings are mechanically derived for old grants.
//
// Spec: docs/go-port/shared/acl.md.
package acl

// Permission is the legacy coarse enum. Wire-stable: integer values
// MUST not be reordered. Used by ACLEntry.permission strings on the
// wire and by store.node_access.permission rows.
type Permission uint8

const (
	// PermDeny is the explicit-negative override. Matches Python
	// Permission.DENY (acl.py:41). Stored as the literal "deny" in
	// node_access.permission. The zero value is DENY, NOT "unspecified",
	// to mirror the Python set("") -> {} behaviour of an unset grant.
	PermDeny Permission = iota
	// PermRead — implies READ. Python "read".
	PermRead
	// PermComment — implies READ, COMMENT. Python "comment".
	PermComment
	// PermWrite — implies READ, COMMENT, WRITE. Python "write".
	PermWrite
	// PermShare — implies READ, COMMENT, WRITE, SHARE. Python "share".
	PermShare
	// PermDelete — implies READ, COMMENT, WRITE, DELETE (NOT SHARE).
	// Python "delete".
	PermDelete
	// PermAdmin — implies every positive permission. Python "admin".
	PermAdmin
)

// String returns the canonical lowercase form stored in
// node_access.permission. Mirrors Permission.value in the Python enum.
func (p Permission) String() string {
	switch p {
	case PermDeny:
		return "deny"
	case PermRead:
		return "read"
	case PermComment:
		return "comment"
	case PermWrite:
		return "write"
	case PermShare:
		return "share"
	case PermDelete:
		return "delete"
	case PermAdmin:
		return "admin"
	default:
		return "unknown"
	}
}

// ParsePermission parses the wire string form into a Permission. Unknown
// inputs return (PermDeny, false) — callers must check the bool, since
// PermDeny is a real value (not "unspecified").
func ParsePermission(s string) (Permission, bool) {
	switch s {
	case "deny":
		return PermDeny, true
	case "read":
		return PermRead, true
	case "comment":
		return PermComment, true
	case "write":
		return PermWrite, true
	case "share":
		return PermShare, true
	case "delete":
		return PermDelete, true
	case "admin":
		return PermAdmin, true
	default:
		return PermDeny, false
	}
}

// permissionHierarchy mirrors AclManager.PERMISSION_HIERARCHY at
// server/python/entdb_server/apply/acl.py:190-210. A grant of perm P
// satisfies a request for required Q iff Q in permissionHierarchy[P].
//
// PermDeny grants nothing — the table is intentionally empty for it,
// and the AclManager.check_permission two-pass treats DENY specially
// (acl.py:243-264).
var permissionHierarchy = map[Permission]map[Permission]struct{}{
	PermRead: {
		PermRead: {},
	},
	PermComment: {
		PermRead:    {},
		PermComment: {},
	},
	PermWrite: {
		PermRead:    {},
		PermComment: {},
		PermWrite:   {},
	},
	PermShare: {
		PermRead:    {},
		PermComment: {},
		PermWrite:   {},
		PermShare:   {},
	},
	PermDelete: {
		PermRead:    {},
		PermComment: {},
		PermWrite:   {},
		PermDelete:  {},
	},
	PermAdmin: {
		PermRead:    {},
		PermComment: {},
		PermWrite:   {},
		PermShare:   {},
		PermDelete:  {},
		PermAdmin:   {},
	},
}

// Implies reports whether holding p satisfies a request for required.
// PermDeny implies nothing. Mirrors the lookup at acl.py:256-258.
func (p Permission) Implies(required Permission) bool {
	if p == PermDeny {
		return false
	}
	_, ok := permissionHierarchy[p][required]
	return ok
}
