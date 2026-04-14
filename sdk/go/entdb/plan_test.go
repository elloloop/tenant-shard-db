package entdb

import (
	"context"
	"testing"

	"github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/testpb"
	"google.golang.org/protobuf/proto"
)

// newProduct returns a fresh test Product populated with the given
// fields. Returned as a plain proto.Message so plan_test.go doesn't
// depend on the dynamicpb concrete type.
func newProduct(sku, name string, priceCents int64) proto.Message {
	m := testpb.NewProduct()
	testpb.SetProductFields(m, sku, name, priceCents)
	return m
}

func TestPlan_Create_ReturnsAlias(t *testing.T) {
	mock := &mockTransport{}
	plan := newPlan(mock, "t1", "user:alice", "key-1")

	alias := plan.Create(newProduct("WIDGET-1", "Widget", 999))
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
	if ops[0].TypeID != 201 {
		t.Errorf("expected type ID 201, got %d", ops[0].TypeID)
	}
	// Payload must be keyed by field id.
	if ops[0].Data["1"] != "WIDGET-1" {
		t.Errorf("Data[\"1\"] = %v, want WIDGET-1", ops[0].Data["1"])
	}
	if ops[0].Data["2"] != "Widget" {
		t.Errorf("Data[\"2\"] = %v, want Widget", ops[0].Data["2"])
	}
	if ops[0].Data["3"] != int64(999) {
		t.Errorf("Data[\"3\"] = %v, want int64(999)", ops[0].Data["3"])
	}
}

func TestPlan_Create_UniqueAliases(t *testing.T) {
	mock := &mockTransport{}
	plan := newPlan(mock, "t1", "user:alice", "key-1")

	alias1 := plan.Create(newProduct("A", "A", 1))
	alias2 := plan.Create(newProduct("B", "B", 2))
	if alias1 == alias2 {
		t.Errorf("aliases should be unique, got %q twice", alias1)
	}
}

func TestPlan_Create_WithACL(t *testing.T) {
	mock := &mockTransport{}
	plan := newPlan(mock, "t1", "user:alice", "key-1")

	acl := []ACLEntry{{Grantee: UserActor("bob"), Permission: PermissionRead}}
	alias := plan.Create(newProduct("S1", "Secret", 10), WithACL(acl...))
	if alias == "" {
		t.Fatal("expected non-empty alias")
	}

	ops := plan.Operations()
	if len(ops[0].ACL) != 1 {
		t.Fatalf("expected 1 ACL entry, got %d", len(ops[0].ACL))
	}
	if ops[0].ACL[0].Grantee != UserActor("bob") {
		t.Errorf("expected grantee user:bob, got %s", ops[0].ACL[0].Grantee.String())
	}
}

func TestPlan_Create_InMailbox(t *testing.T) {
	mock := &mockTransport{}
	plan := newPlan(mock, "t1", "user:alice", "key-1")

	plan.Create(newProduct("M1", "Mail item", 5), InMailbox("alice"))

	ops := plan.Operations()
	if ops[0].StorageMode != StorageModeUserMailbox {
		t.Errorf("StorageMode = %v, want UserMailbox", ops[0].StorageMode)
	}
	if ops[0].TargetUserID != "alice" {
		t.Errorf("TargetUserID = %q, want alice", ops[0].TargetUserID)
	}
}

func TestPlan_Create_InPublic(t *testing.T) {
	mock := &mockTransport{}
	plan := newPlan(mock, "t1", "user:alice", "key-1")

	plan.Create(newProduct("P1", "Public", 0), InPublic())

	ops := plan.Operations()
	if ops[0].StorageMode != StorageModePublic {
		t.Errorf("StorageMode = %v, want Public", ops[0].StorageMode)
	}
}

func TestPlan_Create_As_ExplicitAlias(t *testing.T) {
	mock := &mockTransport{}
	plan := newPlan(mock, "t1", "user:alice", "key-1")

	alias := plan.Create(newProduct("X", "X", 1), As("my-product"))
	if alias != "my-product" {
		t.Errorf("alias = %q, want my-product", alias)
	}
	if plan.Operations()[0].Alias != "my-product" {
		t.Errorf("op alias = %q, want my-product", plan.Operations()[0].Alias)
	}
}

func TestPlan_Create_NonEntityPanics(t *testing.T) {
	mock := &mockTransport{}
	plan := newPlan(mock, "t1", "user:alice", "key-1")

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for non-entity message")
		}
	}()
	plan.Create(testpb.NewNotAnEntity())
}

func TestPlan_Update(t *testing.T) {
	mock := &mockTransport{}
	plan := newPlan(mock, "t1", "user:alice", "key-1")

	patch := newProduct("", "", 777)
	plan.Update("node-1", patch)

	ops := plan.Operations()
	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}
	if ops[0].Type != OpUpdateNode {
		t.Errorf("expected OpUpdateNode, got %v", ops[0].Type)
	}
	if ops[0].NodeID != "node-1" {
		t.Errorf("NodeID = %q, want node-1", ops[0].NodeID)
	}
	if ops[0].TypeID != 201 {
		t.Errorf("TypeID = %d, want 201", ops[0].TypeID)
	}
	if ops[0].Patch["3"] != int64(777) {
		t.Errorf("Patch[\"3\"] = %v, want int64(777)", ops[0].Patch["3"])
	}
	// Unset fields (sku=\"\", name=\"\") must NOT be in the patch —
	// proto3 Range iterates only set fields, and the empty-string
	// sets are zero values for proto3 scalars.
	if _, ok := ops[0].Patch["1"]; ok {
		t.Errorf("Patch should not contain unset sku field")
	}
	if _, ok := ops[0].Patch["2"]; ok {
		t.Errorf("Patch should not contain unset name field")
	}
}

func TestPlan_Delete(t *testing.T) {
	mock := &mockTransport{}
	plan := newPlan(mock, "t1", "user:alice", "key-1")

	Delete[*testpb.Product](plan, "node-1")

	ops := plan.Operations()
	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}
	if ops[0].Type != OpDeleteNode {
		t.Errorf("expected OpDeleteNode, got %v", ops[0].Type)
	}
	if ops[0].NodeID != "node-1" {
		t.Errorf("NodeID = %q, want node-1", ops[0].NodeID)
	}
	if ops[0].TypeID != 201 {
		t.Errorf("TypeID = %d, want 201", ops[0].TypeID)
	}
}

func TestPlan_CreateEdge(t *testing.T) {
	mock := &mockTransport{}
	plan := newPlan(mock, "t1", "user:alice", "key-1")

	EdgeCreate[*testpb.PurchaseEdge](plan, "from-id", "to-id")

	ops := plan.Operations()
	if len(ops) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(ops))
	}
	if ops[0].Type != OpCreateEdge {
		t.Errorf("expected OpCreateEdge, got %v", ops[0].Type)
	}
	if ops[0].EdgeTypeID != 301 {
		t.Errorf("EdgeTypeID = %d, want 301", ops[0].EdgeTypeID)
	}
	if ops[0].FromNodeID != "from-id" {
		t.Errorf("From = %q, want from-id", ops[0].FromNodeID)
	}
	if ops[0].ToNodeID != "to-id" {
		t.Errorf("To = %q, want to-id", ops[0].ToNodeID)
	}
}

func TestPlan_DeleteEdge(t *testing.T) {
	mock := &mockTransport{}
	plan := newPlan(mock, "t1", "user:alice", "key-1")

	EdgeDelete[*testpb.PurchaseEdge](plan, "from-id", "to-id")

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

	alias := plan.Create(newProduct("A", "A", 1))
	plan.Update("existing-1", newProduct("", "B", 0))
	EdgeCreate[*testpb.PurchaseEdge](plan, alias, "existing-1")
	Delete[*testpb.Product](plan, "old-node")

	ops := plan.Operations()
	if len(ops) != 4 {
		t.Fatalf("expected 4 operations, got %d", len(ops))
	}
	wantTypes := []OperationType{OpCreateNode, OpUpdateNode, OpCreateEdge, OpDeleteNode}
	for i, want := range wantTypes {
		if ops[i].Type != want {
			t.Errorf("op[%d].Type = %v, want %v", i, ops[i].Type, want)
		}
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

	plan.Create(newProduct("X", "X", 1))

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
	plan.Create(newProduct("X", "X", 1))

	_, _ = plan.Commit(context.Background())

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on double commit")
		}
	}()
	_, _ = plan.Commit(context.Background())
}

func TestPlan_OperationAfterCommit_Panics(t *testing.T) {
	mock := &mockTransport{commitResp: &CommitResult{Success: true}}
	plan := newPlan(mock, "t1", "user:alice", "key-1")
	plan.Create(newProduct("X", "X", 1))
	_, _ = plan.Commit(context.Background())

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on operation after commit")
		}
	}()
	plan.Create(newProduct("Y", "Y", 2))
}

func TestScope_Chaining(t *testing.T) {
	mock := &mockTransport{}
	client := newClientWithTransport("localhost:50051", mock)

	scope := client.Tenant("acme").Actor(UserActor("bob"))
	if scope.TenantID() != "acme" {
		t.Errorf("expected tenant acme, got %s", scope.TenantID())
	}
	if scope.ActorID() != UserActor("bob") {
		t.Errorf("expected actor user:bob, got %s", scope.ActorID())
	}
}

func TestScope_Plan(t *testing.T) {
	mock := &mockTransport{}
	client := newClientWithTransport("localhost:50051", mock)

	plan := client.Tenant("acme").Actor(UserActor("bob")).Plan()
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
		OwnerActor: UserActor("alice"),
		ACL:        []ACLEntry{{Grantee: UserActor("bob"), Permission: PermissionRead}},
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

func TestActor_Constructors(t *testing.T) {
	tests := []struct {
		name  string
		actor Actor
		kind  string
		id    string
		str   string
	}{
		{"user", UserActor("bob"), "user", "bob", "user:bob"},
		{"group", GroupActor("admins"), "group", "admins", "group:admins"},
		{"service", ServiceActor("api"), "service", "api", "service:api"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.actor.Kind() != tt.kind {
				t.Errorf("Kind() = %q, want %q", tt.actor.Kind(), tt.kind)
			}
			if tt.actor.ID() != tt.id {
				t.Errorf("ID() = %q, want %q", tt.actor.ID(), tt.id)
			}
			if tt.actor.String() != tt.str {
				t.Errorf("String() = %q, want %q", tt.actor.String(), tt.str)
			}
		})
	}
}

func TestActor_ParseActor(t *testing.T) {
	a, err := ParseActor("user:charlie")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a != UserActor("charlie") {
		t.Errorf("got %v, want user:charlie", a)
	}
	_, err = ParseActor("invalid")
	if err == nil {
		t.Fatal("expected error for invalid actor string")
	}
}

func TestActor_Equality(t *testing.T) {
	a := UserActor("bob")
	b := UserActor("bob")
	c := GroupActor("bob")
	if a != b {
		t.Error("same actors should be equal")
	}
	if a == c {
		t.Error("different kinds should not be equal")
	}
}

func TestPermission_Values(t *testing.T) {
	if string(PermissionRead) != "read" {
		t.Errorf("PermissionRead = %q, want read", PermissionRead)
	}
	if string(PermissionWrite) != "write" {
		t.Errorf("PermissionWrite = %q, want write", PermissionWrite)
	}
	if string(PermissionAdmin) != "admin" {
		t.Errorf("PermissionAdmin = %q, want admin", PermissionAdmin)
	}
}

func TestScope_Share(t *testing.T) {
	mock := &mockTransport{}
	client := newTestClient(t, mock)
	scope := client.Tenant("acme").Actor(UserActor("alice"))
	err := scope.Share(context.Background(), "n1", UserActor("charlie"), PermissionWrite)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
