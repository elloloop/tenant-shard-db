// SPDX-License-Identifier: AGPL-3.0-only

package acl

// CoreCapability is the typed-capability enum frozen in
// docs/decisions/acl.md (2026-04-13). Wire-stable: integer values MUST
// match entdb.v1.CoreCapability.
type CoreCapability uint8

const (
	// CoreCapUnspecified is the zero value. Treated as "no requirement"
	// by Registry.RequiredForOp and "no grant" by CheckGrant.
	CoreCapUnspecified CoreCapability = 0
	// CoreCapRead — can see the node.
	CoreCapRead CoreCapability = 1
	// CoreCapComment — can create child Comment nodes. Implies READ.
	CoreCapComment CoreCapability = 2
	// CoreCapEdit — can update node fields. Implies READ, COMMENT.
	CoreCapEdit CoreCapability = 3
	// CoreCapDelete — can delete the node.
	CoreCapDelete CoreCapability = 4
	// CoreCapAdmin — can manage ACL + everything else. Supremum.
	CoreCapAdmin CoreCapability = 5
)

// String returns the wire-stable name for a CoreCapability. Mirrors
// the Python enum value names.
func (c CoreCapability) String() string {
	switch c {
	case CoreCapUnspecified:
		return "CORE_CAP_UNSPECIFIED"
	case CoreCapRead:
		return "CORE_CAP_READ"
	case CoreCapComment:
		return "CORE_CAP_COMMENT"
	case CoreCapEdit:
		return "CORE_CAP_EDIT"
	case CoreCapDelete:
		return "CORE_CAP_DELETE"
	case CoreCapAdmin:
		return "CORE_CAP_ADMIN"
	default:
		return "CORE_CAP_UNKNOWN"
	}
}

// ExtCapID is an opaque per-type extension capability identifier. Its
// meaning is type-scoped — see docs/decisions/acl.md "Layer 2".
type ExtCapID int32

// CoreImplications is the built-in core implication hierarchy. Used by the
// per-type closure builder; consumers go through Registry instead.
var CoreImplications = map[CoreCapability][]CoreCapability{
	CoreCapAdmin: {
		CoreCapRead,
		CoreCapComment,
		CoreCapEdit,
		CoreCapDelete,
	},
	CoreCapEdit: {
		CoreCapRead,
		CoreCapComment,
	},
	CoreCapComment: {
		CoreCapRead,
	},
}

// LegacyToCoreCaps maps a legacy Permission to the typed CoreCapability
// list a grant of that permission implies.
//
// Used by the wire compatibility shim on ShareNode and the
// migrate_permissions_to_capabilities back-fill path.
//
//   - PermDeny → [] (deny grants no positive caps; the deny row itself
//     is recognised by the ACL query path).
//   - PermRead → [READ]
//   - PermComment → [READ, COMMENT]
//   - PermWrite → [READ, COMMENT, EDIT]
//   - PermShare → [ADMIN] (collapses; SHARE is not a CoreCapability)
//   - PermDelete → [READ, COMMENT, EDIT, DELETE]
//   - PermAdmin → [ADMIN]
func LegacyToCoreCaps(p Permission) []CoreCapability {
	switch p {
	case PermDeny:
		return nil
	case PermRead:
		return []CoreCapability{CoreCapRead}
	case PermComment:
		return []CoreCapability{CoreCapRead, CoreCapComment}
	case PermWrite:
		return []CoreCapability{CoreCapRead, CoreCapComment, CoreCapEdit}
	case PermShare:
		return []CoreCapability{CoreCapAdmin}
	case PermDelete:
		return []CoreCapability{
			CoreCapRead, CoreCapComment, CoreCapEdit, CoreCapDelete,
		}
	case PermAdmin:
		return []CoreCapability{CoreCapAdmin}
	default:
		return nil
	}
}
