// SPDX-License-Identifier: MIT
package entdb

import (
	"context"
	"errors"
	"testing"
)

// TestInsertIfNotExists_HappyPath pins that a successful single-op
// commit returns the created id with existed empty. Issue #599.
func TestInsertIfNotExists_HappyPath(t *testing.T) {
	mock := &mockTransport{
		commitResp: &CommitResult{
			Success:        true,
			Receipt:        &Receipt{StreamPosition: "wal:0:1"},
			CreatedNodeIDs: []string{"new-id-1"},
		},
	}
	client := newTestClient(t, mock)
	scope := client.Tenant("acme").Actor(UserActor("alice"))
	created, existed, err := scope.InsertIfNotExists(
		context.Background(), "key-1", newProduct("sku-1", "Widget", 100),
	)
	if err != nil {
		t.Fatalf("InsertIfNotExists: %v", err)
	}
	if created != "new-id-1" {
		t.Errorf("created = %q, want new-id-1", created)
	}
	if existed != "" {
		t.Errorf("existed = %q, want \"\" on a successful insert", existed)
	}
	// WaitApplied must have been forced on so the applier's outcome
	// surfaces synchronously (otherwise the loser of a unique race
	// would get a phantom success — issue #606).
	if !mock.lastWaitAppliedSet || !mock.lastWaitApplied {
		t.Errorf("InsertIfNotExists should force WithWaitApplied(true); set=%v applied=%v",
			mock.lastWaitAppliedSet, mock.lastWaitApplied)
	}
}

// TestInsertIfNotExists_SingleFieldConflictResolves pins the
// post-conflict GetNodeByKey path: a *UniqueConstraintError surfaces
// the (type_id, field_id, value), the helper follows up with
// GetNodeByKey and returns the existing id. Issue #599.
func TestInsertIfNotExists_SingleFieldConflictResolves(t *testing.T) {
	uce := &UniqueConstraintError{
		EntDBError: EntDBError{Message: "Unique constraint violation"},
		TenantID:   "acme",
		TypeID:     201,
		FieldID:    1,
		Value:      "sku-1",
	}
	mock := &mockTransport{
		commitErr:        uce,
		getNodeByKeyResp: &Node{NodeID: "existing-id-7", TypeID: 201},
	}
	client := newTestClient(t, mock)
	scope := client.Tenant("acme").Actor(UserActor("alice"))
	created, existed, err := scope.InsertIfNotExists(
		context.Background(), "key-2", newProduct("sku-1", "Widget", 100),
	)
	if err != nil {
		t.Fatalf("InsertIfNotExists: %v", err)
	}
	if created != "" {
		t.Errorf("created = %q, want \"\" on a conflict", created)
	}
	if existed != "existing-id-7" {
		t.Errorf("existed = %q, want existing-id-7", existed)
	}
	if mock.getNodeByKeyCalls != 1 {
		t.Errorf("GetNodeByKey calls = %d, want 1", mock.getNodeByKeyCalls)
	}
	if mock.lastGetByKeyTypeID != 201 || mock.lastGetByKeyFieldID != 1 || mock.lastGetByKeyValue != "sku-1" {
		t.Errorf("GetNodeByKey args = (%d, %d, %v); want (201, 1, sku-1) from the UCE",
			mock.lastGetByKeyTypeID, mock.lastGetByKeyFieldID, mock.lastGetByKeyValue)
	}
}

// TestInsertIfNotExists_CompositeConflictReturnsError pins the v2.1.0
// boundary: composite-unique violations do NOT trigger a follow-up
// lookup (no GetByCompositeKey RPC). The raw *UniqueConstraintError is
// surfaced so callers can do their own composite query. Tracked for
// v2.2 (server-side NodeConflictPolicy_SKIP).
func TestInsertIfNotExists_CompositeConflictReturnsError(t *testing.T) {
	uce := &UniqueConstraintError{
		EntDBError:     EntDBError{Message: "Composite unique constraint violation"},
		TenantID:       "acme",
		TypeID:         201,
		ConstraintName: "(1,2)",
		FieldIDs:       []int32{1, 2},
		Values:         []any{"google", "uid-1"},
	}
	mock := &mockTransport{commitErr: uce}
	client := newTestClient(t, mock)
	scope := client.Tenant("acme").Actor(UserActor("alice"))
	created, existed, err := scope.InsertIfNotExists(
		context.Background(), "key-3", newProduct("sku-1", "Widget", 100),
	)
	if created != "" || existed != "" {
		t.Errorf("expected both ids empty on composite conflict; got created=%q existed=%q", created, existed)
	}
	var got *UniqueConstraintError
	if !errors.As(err, &got) {
		t.Fatalf("expected *UniqueConstraintError, got %T: %v", err, err)
	}
	if !got.IsComposite() {
		t.Error("expected IsComposite() = true")
	}
	if mock.getNodeByKeyCalls != 0 {
		t.Errorf("GetNodeByKey should NOT have been called on a composite conflict; got %d calls", mock.getNodeByKeyCalls)
	}
}

// TestInsertIfNotExists_PropagatesUnrelatedError pins that errors
// other than *UniqueConstraintError bubble up unchanged (no lookup,
// no swallowing).
func TestInsertIfNotExists_PropagatesUnrelatedError(t *testing.T) {
	mock := &mockTransport{commitErr: errors.New("transport: deadline exceeded")}
	client := newTestClient(t, mock)
	scope := client.Tenant("acme").Actor(UserActor("alice"))
	_, _, err := scope.InsertIfNotExists(
		context.Background(), "key-4", newProduct("sku-1", "Widget", 100),
	)
	if err == nil || err.Error() != "transport: deadline exceeded" {
		t.Errorf("err = %v; want unwrapped transport error", err)
	}
	if mock.getNodeByKeyCalls != 0 {
		t.Errorf("GetNodeByKey should not run on non-UCE errors; got %d calls", mock.getNodeByKeyCalls)
	}
}

// TestInsertIfNotExists_NilMsg pins the precondition.
func TestInsertIfNotExists_NilMsg(t *testing.T) {
	mock := &mockTransport{}
	client := newTestClient(t, mock)
	scope := client.Tenant("acme").Actor(UserActor("alice"))
	_, _, err := scope.InsertIfNotExists(context.Background(), "key-5", nil)
	if err == nil {
		t.Fatal("expected error on nil msg")
	}
	if mock.commitCalls != 0 {
		t.Errorf("nil-msg precondition should short-circuit; got %d commit calls", mock.commitCalls)
	}
}
