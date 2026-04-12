package entdb

// Node represents a graph node in EntDB.
type Node struct {
	TenantID   string         `json:"tenant_id"`
	NodeID     string         `json:"node_id"`
	TypeID     int            `json:"type_id"`
	Payload    map[string]any `json:"payload"`
	CreatedAt  int64          `json:"created_at"`
	UpdatedAt  int64          `json:"updated_at"`
	OwnerActor string         `json:"owner_actor"`
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

// ACLEntry represents an access control list entry.
type ACLEntry struct {
	Principal  string `json:"principal"`
	Permission string `json:"permission"`
	ExpiresAt  int64  `json:"expires_at,omitempty"`
}

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

// OperationType enumerates the kinds of operations in a Plan.
type OperationType int

const (
	// OpCreateNode is a node creation operation.
	OpCreateNode OperationType = iota
	// OpUpdateNode is a node update operation.
	OpUpdateNode
	// OpDeleteNode is a node deletion operation.
	OpDeleteNode
	// OpCreateEdge is an edge creation operation.
	OpCreateEdge
	// OpDeleteEdge is an edge deletion operation.
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
