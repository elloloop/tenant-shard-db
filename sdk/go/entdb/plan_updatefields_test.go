// SPDX-License-Identifier: MIT
package entdb

import (
	"testing"

	"github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/testpb"
)

// TestPlan_UpdateFields_IncludesZeroValue pins the #574 fix: UpdateFields
// forces named fields into the patch even at their proto3 zero value, so a
// field can be updated TO 0 / "" / false — which plain Update cannot
// express (it sends only set, non-default fields).
func TestPlan_UpdateFields_IncludesZeroValue(t *testing.T) {
	client := newClientWithTransport("localhost:50051", &mockTransport{})
	plan := client.Tenant("acme").Actor(UserActor("alice")).Plan()

	p := testpb.NewProductMsg()
	p.SetFields("", "", 0) // price_cents (field 3) at its zero value

	plan.UpdateFields("node-1", p, "price_cents")
	if len(plan.operations) != 1 {
		t.Fatalf("ops = %d, want 1", len(plan.operations))
	}
	op := plan.operations[0]
	if op.Type != OpUpdateNode || op.NodeID != "node-1" {
		t.Fatalf("op = %+v", op)
	}
	v, ok := op.Patch["3"]
	if !ok {
		t.Fatal("UpdateFields must include the named zero-valued field (price_cents=3)")
	}
	if v != int64(0) {
		t.Errorf("patch[3] = %v (%T), want int64(0)", v, v)
	}

	// Contrast: plain Update omits the zero field entirely (proto3 default).
	plan2 := client.Tenant("acme").Actor(UserActor("alice")).Plan()
	plan2.Update("node-1", p)
	if _, ok := plan2.operations[0].Patch["3"]; ok {
		t.Error("plain Update should omit the zero-valued price_cents")
	}
}

func TestPlan_UpdateFields_UnknownFieldPanics(t *testing.T) {
	client := newClientWithTransport("localhost:50051", &mockTransport{})
	plan := client.Tenant("acme").Actor(UserActor("alice")).Plan()
	p := testpb.NewProductMsg()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for unknown field name")
		}
	}()
	plan.UpdateFields("node-1", p, "no_such_field")
}

func TestPlan_UpdateFields_NoFieldsPanics(t *testing.T) {
	client := newClientWithTransport("localhost:50051", &mockTransport{})
	plan := client.Tenant("acme").Actor(UserActor("alice")).Plan()
	p := testpb.NewProductMsg()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when no fields are named")
		}
	}()
	plan.UpdateFields("node-1", p)
}
