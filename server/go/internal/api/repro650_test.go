// SPDX-License-Identifier: AGPL-3.0-only

// Full-path reproduction for issue #650: an indexed STRING field carrying an
// enum_values constraint (modelling `delivery_status`) reportedly disappears on
// read-back and an equality query on it returns 0 rows, while a sibling indexed
// bool field (`is_read`) works.
//
// This drives the whole server path the SDK exercises — ExecuteAtomic (legacy
// Struct `data` ingress) -> WAL -> applier -> store -> GetNode handler and
// QueryNodes handler (the field-filter encoding) — to locate where, if
// anywhere, the value is dropped on the CURRENT tree.

package api_test

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	"github.com/elloloop/tenant-shard-db/server/go/internal/apply"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/schema"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

const (
	r650Topic   = "entdb-wal-r650"
	r650Tenant  = "tenant-r650"
	r650Group   = "r650-applier"
	r650Actor   = "user:svc"
	r650Type    = 11
	r650FieldIs = 8  // is_read (bool, indexed) — the field that "works"
	r650FieldDs = 17 // delivery_status (string, indexed, enum_values) — the field reportedly dropped
)

// newR650Fixture mirrors newXAFixture but registers the EmailMessage-like type:
// field 8 = indexed bool, field 17 = indexed string with an enum_values
// constraint.
func newR650Fixture(t *testing.T) *xaFixture {
	t.Helper()
	w := wal.NewInMemory(1)
	if err := w.Connect(context.Background()); err != nil {
		t.Fatalf("wal.Connect: %v", err)
	}
	cs, err := store.New(store.Options{RootDir: t.TempDir(), WALMode: true})
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	if err := cs.OpenTenant(context.Background(), r650Tenant); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}

	reg := schema.NewRegistry()
	if err := reg.RegisterNode(&schema.NodeTypeDef{
		TypeID: r650Type,
		Fields: []schema.FieldDef{
			{FieldID: r650FieldIs, Kind: schema.KindBoolean, Indexed: true},
			{FieldID: r650FieldDs, Kind: schema.KindString, Indexed: true,
				EnumValues: []string{"queued", "sending", "sent", "failed", "bounced"}},
		},
	}); err != nil {
		t.Fatalf("RegisterNode: %v", err)
	}
	if _, err := reg.Freeze(); err != nil {
		t.Fatalf("Freeze: %v", err)
	}

	srv := api.New(
		api.WithStore(cs),
		api.WithWALProducer(w),
		api.WithWALTopic(r650Topic),
		api.WithSchemaRegistry(reg),
	)
	a, err := apply.New(apply.Options{
		Store:       cs,
		Consumer:    w,
		Topic:       r650Topic,
		GroupID:     r650Group,
		PollTimeout: 25 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("apply.New: %v", err)
	}
	return &xaFixture{t: t, wal: w, store: cs, registry: reg, srv: srv, applier: a}
}

func TestRepro650_EnumStringFullPath(t *testing.T) {
	f := newR650Fixture(t)
	f.runApplier(t)
	ctx := context.Background()

	const idem = "r650-create"
	const nodeID = "msg-1"
	resp, err := f.srv.ExecuteAtomic(ctx, &pb.ExecuteAtomicRequest{
		Context:        &pb.RequestContext{TenantId: r650Tenant, Actor: r650Actor},
		IdempotencyKey: idem,
		Operations: []*pb.Operation{{
			Op: &pb.Operation_CreateNode{CreateNode: &pb.CreateNodeOp{
				TypeId: r650Type,
				Id:     nodeID,
				Data: newStruct(t, map[string]any{
					"8":  false,
					"17": "queued",
				}),
			}},
		}},
	})
	if err != nil {
		t.Fatalf("ExecuteAtomic: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("Success=false (error=%q code=%q)", resp.GetError(), resp.GetErrorCode())
	}
	// Wait for the applier to materialize the create on r650Tenant.
	var n *store.Node
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, gerr := f.store.GetNode(ctx, r650Tenant, nodeID)
		if gerr == nil && got != nil {
			n = got
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if n == nil {
		t.Fatalf("applier never materialized node %q", nodeID)
	}
	t.Logf("payload_json = %s", n.PayloadJSON)
	if !strings.Contains(n.PayloadJSON, `"17":"queued"`) {
		t.Fatalf("ISSUE #650 REPRODUCED (ingress/store): delivery_status absent on disk; payload=%s", n.PayloadJSON)
	}

	// (b) GetNode handler (the SDK read path): field 17 must surface.
	gnResp, err := f.srv.GetNode(ctx, &pb.GetNodeRequest{
		Context: &pb.RequestContext{TenantId: r650Tenant, Actor: r650Actor},
		TypeId:  r650Type,
		NodeId:  nodeID,
	})
	if err != nil {
		t.Fatalf("GetNode handler: %v", err)
	}
	gotDS := readField(gnResp.GetNode(), r650FieldDs)
	if gotDS != "queued" {
		t.Fatalf("ISSUE #650 REPRODUCED (read handler): delivery_status read back as %v, want \"queued\"; node=%+v",
			gotDS, gnResp.GetNode())
	}

	// (c) QueryNodes handler equality filter on the enum-string field.
	qResp, err := f.srv.QueryNodes(ctx, &pb.QueryNodesRequest{
		Context: &pb.RequestContext{TenantId: r650Tenant, Actor: r650Actor},
		TypeId:  r650Type,
		Filters: []*pb.FieldFilter{{
			Field: "17",
			Op:    pb.FilterOp_EQ,
			Value: newValue(t, "queued"),
		}},
	})
	if err != nil {
		t.Fatalf("QueryNodes handler: %v", err)
	}
	if len(qResp.GetNodes()) != 1 || qResp.GetNodes()[0].GetNodeId() != nodeID {
		t.Fatalf("ISSUE #650 REPRODUCED (query): equality on delivery_status returned %d rows, want 1",
			len(qResp.GetNodes()))
	}
}

// readField extracts a field value from a returned Node, tolerating either the
// typed payload (ADR-028) or the legacy Struct payload.
func readField(n *pb.Node, fieldID uint32) any {
	if n == nil {
		return nil
	}
	if tv := n.GetTypedPayload(); tv != nil {
		if ev, ok := tv[fieldID]; ok {
			return ev.GetStringValue()
		}
	}
	if d := n.GetPayload(); d != nil {
		if v, ok := d.GetFields()[strconv.Itoa(int(fieldID))]; ok {
			return v.GetStringValue()
		}
	}
	return nil
}
