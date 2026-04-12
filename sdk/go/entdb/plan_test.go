package entdb

import (
	"context"
	"testing"
)

func TestPlan_Create_ReturnsAlias(t *testing.T) {
	mock := &mockTransport{}
	plan := newPlan(mock, "t1", "user:alice", "key-1")

	alias := plan.Create(1, map[string]any{"title": "Task A"})
	if alias == "" {
		t.Fatal("expected non-empty alias")
	}

	ops := plan.Operations()
	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}
	if ops[0].Type != OpCreateNode {
		t.Errorf("expected OpCreateNode, got %v", ops[0].Type)
	}
	if ops[0].Alias != alias {
		t.Errorf("expected alias %q, got %q", alias, ops[0].Alias)
	}
	if ops[0].TypeID != 1 {
		t.Errorf("expected type ID 1, got %d", ops[0].TypeID)
	}
}

func TestPlan_Create_UniqueAliases(t *testing.T) {
	mock := &mockTransport{}
	plan := newPlan(mock, "t1", "user:alice", "key-1")

	alias1 := plan.Create(1, map[string]any{"title": "A"})
	alias2 := plan.Create(1, map[string]any{"title": "B"})
	if alias1 == alias2 {
		t.Errorf("aliases should be unique, got %q twice", alias1)
	}
}

func TestPlan_CreateWithACL(t *testing.T) {
	mock := &mockTransport{}
	plan := newPlan(mock, "t1", "user:alice", "key-1")

	acl := []ACLEntry{{Principal: "user:bob", Permission: "read"}}
	alias := plan.CreateWithACL(1, map[string]any{"title": "Secret"}, acl)
	if alias == "" {
		t.Fatal("expected non-empty alias")
	}

	ops := plan.Operations()
	if len(ops[0].ACL) != 1 {
		t.Fatalf("expected 1 ACL entry, got %d", len(ops[0].ACL))
	}
	if ops[0].ACL[0].Principal != "user:bob" {
		t.Errorf("expected principal user:bob, got %s", ops[0].ACL[0].Principal)
	}
}

func TestPlan_Update(t *testing.T) {
	mock := &mockTransport{}
	plan := newPlan(mock, "t1", "user:alice", "key-1")

	plan.Update("node-1", 1, map[string]any{"status": "done"})

	ops := plan.Operations()
	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}
	if ops[0].Type != OpUpdateNode {
		t.Errorf("expected OpUpdateNode, got %v", ops[0].Type)
	}
	if ops[0].NodeID != "node-1" {
		t.Errorf("expected node ID node-1, got %s", ops[0].NodeID)
	}
	if ops[0].Patch["status"] != "done" {
		t.Errorf("expected patch status=done, got %v", ops[0].Patch["status"])
	}
}

func TestPlan_Delete(t *testing.T) {
	mock := &mockTransport{}
	plan := newPlan(mock, "t1", "user:alice", "key-1")

	plan.Delete("node-1")

	ops := plan.Operations()
	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}
	if ops[0].Type != OpDeleteNode {
		t.Errorf("expected OpDeleteNode, got %v", ops[0].Type)
	}
	if ops[0].NodeID != "node-1" {
		t.Errorf("expected node ID node-1, got %s", ops[0].NodeID)
	}
}

func TestPlan_CreateEdge(t *testing.T) {
	mock := &mockTransport{}
	plan := newPlan(mock, "t1", "user:alice", "key-1")

	plan.CreateEdge(10, "from-id", "to-id")

	ops := plan.Operations()
	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}
	if ops[0].Type != OpCreateEdge {
		t.Errorf("expected OpCreateEdge, got %v", ops[0].Type)
	}
	if ops[0].EdgeTypeID != 10 {
		t.Errorf("expected edge type ID 10, got %d", ops[0].EdgeTypeID)
	}
	if ops[0].FromNodeID != "from-id" {
		t.Errorf("expected from from-id, got %s", ops[0].FromNodeID)
	}
	if ops[0].ToNodeID != "to-id" {
		t.Errorf("expected to to-id, got %s", ops[0].ToNodeID)
	}
}

func TestPlan_DeleteEdge(t *testing.T) {
	mock := &mockTransport{}
	plan := newPlan(mock, "t1", "user:alice", "key-1")

	plan.DeleteEdge(10, "from-id", "to-id")

	ops := plan.Operations()
	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}
	if ops[0].Type != OpDeleteEdge {
		t.Errorf("expected OpDeleteEdge, got %v", ops[0].Type)
	}
}

func TestPlan_MultipleOperations(t *testing.T) {
	mock := &mockTransport{}
	plan := newPlan(mock, "t1", "user:alice", "key-1")

	alias := plan.Create(1, map[string]any{"title": "Task"})
	plan.Update("existing-1", 1, map[string]any{"status": "done"})
	plan.CreateEdge(10, alias, "existing-1")
	plan.Delete("old-node")

	ops := plan.Operations()
	if len(ops) != 4 {
		t.Fatalf("expected 4 operations, got %d", len(ops))
	}
	if ops[0].Type != OpCreateNode {
		t.Errorf("op[0]: expected OpCreateNode, got %v", ops[0].Type)
	}
	if ops[1].Type != OpUpdateNode {
		t.Errorf("op[1]: expected OpUpdateNode, got %v", ops[1].Type)
	}
	if ops[2].Type != OpCreateEdge {
		t.Errorf("op[2]: expected OpCreateEdge, got %v", ops[2].Type)
	}
	if ops[3].Type != OpDeleteNode {
		t.Errorf("op[3]: expected OpDeleteNode, got %v", ops[3].Type)
	}
}

func TestPlan_Commit(t *testing.T) {
	expectedResult := &CommitResult{
		Success:        true,
		CreatedNodeIDs: []string{"node-abc"},
		Applied:        true,
	}
	mock := &mockTransport{commitResp: expectedResult}
	plan := newPlan(mock, "t1", "user:alice", "key-1")

	plan.Create(1, map[string]any{"title": "Task"})

	result, err := plan.Commit(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
	if len(result.CreatedNodeIDs) != 1 || result.CreatedNodeIDs[0] != "node-abc" {
		t.Errorf("unexpected created node IDs: %v", result.CreatedNodeIDs)
	}
	if mock.commitCalls != 1 {
		t.Errorf("expected 1 commit call, got %d", mock.commitCalls)
	}
}

func TestPlan_DoubleCommit_Panics(t *testing.T) {
	mock := &mockTransport{commitResp: &CommitResult{Success: true}}
	plan := newPlan(mock, "t1", "user:alice", "key-1")
	plan.Create(1, map[string]any{"title": "Task"})

	_, _ = plan.Commit(context.Background())

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on double commit")
		}
	}()
	_, _ = plan.Commit(context.Background())
}

func TestPlan_OperationAfterCommit_Panics(t *testing.T) {
	mock := &mockTransport{commitResp: &CommitResult{Success: true}}
	plan := newPlan(mock, "t1", "user:alice", "key-1")
	plan.Create(1, map[string]any{"title": "Task"})
	_, _ = plan.Commit(context.Background())

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on operation after commit")
		}
	}()
	plan.Create(1, map[string]any{"title": "Another"})
}

func TestScope_Chaining(t *testing.T) {
	expected := &Node{NodeID: "n1", TypeID: 1}
	mock := &mockTransport{getNodeResp: expected}
	client := newClientWithTransport("localhost:50051", mock)

	scope := client.Tenant("acme").Actor("user:bob")
	if scope.TenantID() != "acme" {
		t.Errorf("expected tenant acme, got %s", scope.TenantID())
	}
	if scope.ActorID() != "user:bob" {
		t.Errorf("expected actor user:bob, got %s", scope.ActorID())
	}

	node, err := scope.Get(context.Background(), 1, "n1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.NodeID != "n1" {
		t.Errorf("expected node n1, got %s", node.NodeID)
	}
}

func TestScope_Query(t *testing.T) {
	expected := []*Node{{NodeID: "n1"}, {NodeID: "n2"}}
	mock := &mockTransport{queryResp: expected}
	client := newClientWithTransport("localhost:50051", mock)

	scope := client.Tenant("acme").Actor("user:bob")
	nodes, err := scope.Query(context.Background(), 1, map[string]any{"status": "active"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(nodes))
	}
}

func TestScope_Plan(t *testing.T) {
	mock := &mockTransport{}
	client := newClientWithTransport("localhost:50051", mock)

	plan := client.Tenant("acme").Actor("user:bob").Plan()
	if plan.TenantID() != "acme" {
		t.Errorf("expected tenant acme, got %s", plan.TenantID())
	}
	if plan.Actor() != "user:bob" {
		t.Errorf("expected actor user:bob, got %s", plan.Actor())
	}
}

func TestNode_Fields(t *testing.T) {
	node := Node{
		TenantID:   "t1",
		NodeID:     "n1",
		TypeID:     42,
		Payload:    map[string]any{"title": "Hello", "count": 7},
		CreatedAt:  1000,
		UpdatedAt:  2000,
		OwnerActor: "user:alice",
		ACL:        []ACLEntry{{Principal: "user:bob", Permission: "read"}},
	}
	if node.TenantID != "t1" {
		t.Errorf("TenantID = %s, want t1", node.TenantID)
	}
	if node.TypeID != 42 {
		t.Errorf("TypeID = %d, want 42", node.TypeID)
	}
	if node.Payload["title"] != "Hello" {
		t.Errorf("Payload[title] = %v, want Hello", node.Payload["title"])
	}
	if len(node.ACL) != 1 {
		t.Fatalf("ACL len = %d, want 1", len(node.ACL))
	}
	if node.ACL[0].Principal != "user:bob" {
		t.Errorf("ACL[0].Principal = %s, want user:bob", node.ACL[0].Principal)
	}
}

func TestEdge_Fields(t *testing.T) {
	edge := Edge{
		TenantID:   "t1",
		EdgeTypeID: 10,
		FromNodeID: "n1",
		ToNodeID:   "n2",
		Props:      map[string]any{"weight": 0.5},
		CreatedAt:  3000,
	}
	if edge.EdgeTypeID != 10 {
		t.Errorf("EdgeTypeID = %d, want 10", edge.EdgeTypeID)
	}
	if edge.FromNodeID != "n1" {
		t.Errorf("FromNodeID = %s, want n1", edge.FromNodeID)
	}
	if edge.Props["weight"] != 0.5 {
		t.Errorf("Props[weight] = %v, want 0.5", edge.Props["weight"])
	}
}

func TestErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode string
		wantMsg  string
	}{
		{
			name:     "connection error",
			err:      NewConnectionError("refused", "localhost:50051"),
			wantCode: "CONNECTION_ERROR",
			wantMsg:  "entdb CONNECTION_ERROR: refused",
		},
		{
			name:     "validation error",
			err:      NewValidationError("bad field", "email", []string{"required"}),
			wantCode: "VALIDATION_ERROR",
			wantMsg:  "entdb VALIDATION_ERROR: bad field",
		},
		{
			name:     "not found error",
			err:      NewNotFoundError("missing", "node", "n1"),
			wantCode: "NOT_FOUND",
			wantMsg:  "entdb NOT_FOUND: missing",
		},
		{
			name:     "access denied error",
			err:      NewAccessDeniedError("forbidden", "user:bob", "n1", "write"),
			wantCode: "ACCESS_DENIED",
			wantMsg:  "entdb ACCESS_DENIED: forbidden",
		},
		{
			name:     "transaction error",
			err:      NewTransactionError("conflict", "key-1"),
			wantCode: "TRANSACTION_ERROR",
			wantMsg:  "entdb TRANSACTION_ERROR: conflict",
		},
		{
			name:     "schema error",
			err:      NewSchemaError("mismatch", "fp1", "fp2"),
			wantCode: "SCHEMA_ERROR",
			wantMsg:  "entdb SCHEMA_ERROR: mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tt.err.Error()
			if msg != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", msg, tt.wantMsg)
			}
		})
	}
}
