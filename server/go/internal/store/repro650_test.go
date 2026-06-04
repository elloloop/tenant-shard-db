// SPDX-License-Identifier: AGPL-3.0-only

package store_test

import (
	"context"
	"strings"
	"testing"

	"github.com/elloloop/tenant-shard-db/server/go/internal/schema"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

// TestRepro650_EnumStringStoreAndQuery reproduces issue #650 at the store
// layer: an indexed STRING field carrying an enum_values constraint must
// round-trip (be present on read-back) and be matchable by an equality query,
// exactly like the sibling indexed bool field.
func TestRepro650_EnumStringStoreAndQuery(t *testing.T) {
	reg := schema.NewRegistry()
	if err := reg.RegisterNode(&schema.NodeTypeDef{
		TypeID: 11,
		Fields: []schema.FieldDef{
			{FieldID: 8, Kind: schema.KindBoolean, Indexed: true},
			{FieldID: 17, Kind: schema.KindString, Indexed: true,
				EnumValues: []string{"queued", "sending", "sent", "failed", "bounced"}},
		},
	}); err != nil {
		t.Fatalf("RegisterNode: %v", err)
	}
	if _, err := reg.Freeze(); err != nil {
		t.Fatalf("Freeze: %v", err)
	}

	cs := newStoreWithRegistry(t, reg)
	ctx := context.Background()
	const tenantID = "acme"
	if err := cs.OpenTenant(ctx, tenantID); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}
	if _, err := cs.CreateNodeRaw(ctx, tenantID, store.NodeInput{
		NodeID:     "m1",
		TypeID:     11,
		OwnerActor: "user:svc",
		Payload:    map[string]any{"8": false, "17": "queued"},
	}); err != nil {
		t.Fatalf("CreateNodeRaw: %v", err)
	}

	// (a) read-back by id: the enum-string field must be present.
	n, err := cs.GetNode(ctx, tenantID, "m1")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	t.Logf("payload_json = %s", n.PayloadJSON)
	if !strings.Contains(n.PayloadJSON, `"17":"queued"`) {
		t.Fatalf("ISSUE #650 REPRODUCED (store): delivery_status (field 17) absent on read-back; payload=%s", n.PayloadJSON)
	}

	// (b) equality query on the enum-string field must match.
	nodes, err := cs.QueryNodes(ctx, store.QueryNodesArgs{
		TenantID:        tenantID,
		TypeID:          11,
		EqualityFilters: map[uint32]any{17: "queued"},
	})
	if err != nil {
		t.Fatalf("QueryNodes: %v", err)
	}
	if len(nodes) != 1 || nodes[0].NodeID != "m1" {
		t.Fatalf("ISSUE #650 REPRODUCED (store): equality query on field 17 returned %d rows, want 1", len(nodes))
	}
}
