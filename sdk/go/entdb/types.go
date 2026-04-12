package entdb

import "fmt"

// ── Actor ───────────────────────────────────────────────────────────

// Actor is a typed principal identifier that replaces raw "user:bob" strings.
// Use the constructor functions UserActor, GroupActor, ServiceActor.
type Actor struct {
	kind string
	id   string
}

// UserActor creates an Actor for a user principal.
func UserActor(id string) Actor { return Actor{kind: "user", id: id} }

// GroupActor creates an Actor for a group principal.
func GroupActor(id string) Actor { return Actor{kind: "group", id: id} }

// ServiceActor creates an Actor for a service principal.
func ServiceActor(id string) Actor { return Actor{kind: "service", id: id} }

// ParseActor parses a "kind:id" string into an Actor.
func ParseActor(s string) (Actor, error) {
	for i, c := range s {
		if c == ':' {
			return Actor{kind: s[:i], id: s[i+1:]}, nil
		}
	}
	return Actor{}, fmt.Errorf("entdb: invalid actor %q (expected kind:id)", s)
}

// Kind returns the actor kind (user, group, service).
func (a Actor) Kind() string { return a.kind }

// ID returns the actor identifier.
func (a Actor) ID() string { return a.id }

// String returns the "kind:id" wire format.
func (a Actor) String() string { return a.kind + ":" + a.id }

// ── Permission ──────────────────────────────────────────────────────

// Permission represents an ACL permission level.
type Permission string

const (
	PermissionRead  Permission = "read"
	PermissionWrite Permission = "write"
	PermissionAdmin Permission = "admin"
)

// ── ACL ─────────────────────────────────────────────────────────────

// ACLEntry represents an access control list entry.
type ACLEntry struct {
	Grantee    Actor      `json:"grantee"`
	Permission Permission `json:"permission"`
	ExpiresAt  int64      `json:"expires_at,omitempty"`
}

// ── Node / Edge ─────────────────────────────────────────────────────

// Node represents a graph node in EntDB.
type Node struct {
	TenantID   string         `json:"tenant_id"`
	NodeID     string         `json:"node_id"`
	TypeID     int            `json:"type_id"`
	Payload    map[string]any `json:"payload"`
	CreatedAt  int64          `json:"created_at"`
	UpdatedAt  int64          `json:"updated_at"`
	OwnerActor Actor          `json:"owner_actor"`
	ACL        []ACLEntry     `json:"acl,omitempty"`
}

// Edge represents a graph edge in EntDB.
type Edge struct {
	TenantID   string         `json:"tenant_id"`
	EdgeTypeID int            `json:"edge_type_id"`
	FromNodeID string         `json:"from_node_id"`
	ToNodeID   string         `json:"to_node_id"`
	Props      map[string]any `json:"props,omitempty"`
	CreatedAt  int64          `json:"created_at"`
}

// ── Transaction ─────────────────────────────────────────────────────

// Receipt tracks a committed transaction.
type Receipt struct {
	TenantID       string `json:"tenant_id"`
	IdempotencyKey string `json:"idempotency_key"`
	StreamPosition string `json:"stream_position,omitempty"`
}

// CommitResult is the result of committing a Plan.
type CommitResult struct {
	Success        bool     `json:"success"`
	Receipt        *Receipt `json:"receipt,omitempty"`
	CreatedNodeIDs []string `json:"created_node_ids,omitempty"`
	Applied        bool     `json:"applied"`
	Error          string   `json:"error,omitempty"`
}

// ── Operations ──────────────────────────────────────────────────────

// OperationType enumerates the kinds of operations in a Plan.
type OperationType int

const (
	OpCreateNode OperationType = iota
	OpUpdateNode
	OpDeleteNode
	OpCreateEdge
	OpDeleteEdge
)

// Operation represents a single mutation in a Plan.
type Operation struct {
	Type       OperationType  `json:"type"`
	TypeID     int            `json:"type_id,omitempty"`
	NodeID     string         `json:"node_id,omitempty"`
	Alias      string         `json:"alias,omitempty"`
	Data       map[string]any `json:"data,omitempty"`
	Patch      map[string]any `json:"patch,omitempty"`
	EdgeTypeID int            `json:"edge_type_id,omitempty"`
	FromNodeID string         `json:"from_node_id,omitempty"`
	ToNodeID   string         `json:"to_node_id,omitempty"`
	ACL        []ACLEntry     `json:"acl,omitempty"`
}
