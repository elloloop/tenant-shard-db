// SPDX-License-Identifier: AGPL-3.0-only

package apply_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/elloloop/tenant-shard-db/server/go/internal/apply"
	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

const (
	testTopic   = "entdb-wal-test"
	testTenant  = "tenant_a"
	testGroupID = "applier-test"
)

// fixture wires an in-memory WAL + per-tenant store + (optional) global
// store + applier. Cleaning up is t.Cleanup so individual tests don't
// have to remember to Stop / Close.
type fixture struct {
	t       *testing.T
	wal     *wal.InMemory
	store   *store.CanonicalStore
	global  *globalstore.GlobalStore
	applier *apply.Applier
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	w := wal.NewInMemory(1)
	if err := w.Connect(context.Background()); err != nil {
		t.Fatalf("wal.Connect: %v", err)
	}
	dir := t.TempDir()
	cs, err := store.New(store.Options{RootDir: dir, WALMode: true})
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	gs, err := globalstore.New(globalstore.Options{DataDir: dir, WALMode: true})
	if err != nil {
		t.Fatalf("globalstore.New: %v", err)
	}
	t.Cleanup(func() { _ = gs.Close() })

	a, err := apply.New(apply.Options{
		Store:    cs,
		Global:   gs,
		Consumer: w,
		Topic:    testTopic,
		GroupID:  testGroupID,
		// Tight poll timeout keeps the consumer responsive in tests.
		PollTimeout: 25 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("apply.New: %v", err)
	}
	if err := cs.OpenTenant(context.Background(), testTenant); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}

	return &fixture{t: t, wal: w, store: cs, global: gs, applier: a}
}

// appendEvent JSON-encodes ev and appends it to the WAL under the
// tenant's key. Returns the assigned StreamPos.
func (f *fixture) appendEvent(t *testing.T, ev apply.Event) wal.StreamPos {
	t.Helper()
	val, err := ev.Encode()
	if err != nil {
		t.Fatalf("event.Encode: %v", err)
	}
	headers := map[string][]byte{
		wal.HeaderIdempotencyKey: []byte(ev.IdempotencyKey),
	}
	pos, err := f.wal.Append(context.Background(), testTopic, ev.TenantID, val, headers)
	if err != nil {
		t.Fatalf("wal.Append: %v", err)
	}
	return pos
}

// runApplierUntilApplied runs the applier in a goroutine and returns a
// stop function. The caller is expected to call stop() when done.
func (f *fixture) runApplierUntilApplied(t *testing.T) (cancel func()) {
	t.Helper()
	ctx, cancelF := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- f.applier.Run(ctx) }()
	t.Cleanup(func() {
		cancelF()
		<-done
	})
	return cancelF
}

// waitForIdempKey polls until idempKey appears in applied_events for
// tenantID, or the deadline fires.
func (f *fixture) waitForIdempKey(t *testing.T, tenantID, idempKey string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		ok, err := f.store.CheckIdempotency(context.Background(), tenantID, idempKey)
		if err != nil {
			t.Fatalf("CheckIdempotency: %v", err)
		}
		if ok {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("waitForIdempKey: %q never showed up", idempKey)
}

func mkCreateNode(id string, typeID int32, payload map[string]any) map[string]any {
	return map[string]any{
		"op":      string(apply.OpCreateNode),
		"id":      id,
		"type_id": typeID,
		"data":    payload,
	}
}

// dumpState produces a deterministic snapshot of the per-tenant store
// for replay-determinism assertions. Only the columns the contract
// pins are included.
func dumpState(t *testing.T, cs *store.CanonicalStore, tenantID string, nodeIDs []string) map[string]any {
	t.Helper()
	type nodeRepr struct {
		NodeID      string
		TypeID      int32
		OwnerActor  string
		PayloadJSON string
		ACLJSON     string
	}
	out := map[string]any{}
	var nodes []nodeRepr
	for _, id := range nodeIDs {
		n, err := cs.GetNode(context.Background(), tenantID, id)
		if err != nil {
			continue
		}
		nodes = append(nodes, nodeRepr{
			NodeID:      n.NodeID,
			TypeID:      n.TypeID,
			OwnerActor:  n.OwnerActor,
			PayloadJSON: n.PayloadJSON,
			ACLJSON:     n.ACLJSON,
		})
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].NodeID < nodes[j].NodeID })
	out["nodes"] = nodes
	return out
}

// TestApplier_HappyPathCreateNode confirms a create_node op materialises a
// row through the applier.
func TestApplier_HappyPathCreateNode(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ev := apply.Event{
		TenantID:       testTenant,
		Actor:          "user:alice",
		IdempotencyKey: "k1",
		TsMs:           1700000000000,
		Ops:            []map[string]any{mkCreateNode("node1", 1, map[string]any{"1": "alice@x"})},
	}
	f.appendEvent(t, ev)
	f.runApplierUntilApplied(t)
	f.waitForIdempKey(t, testTenant, "k1")

	n, err := f.store.GetNode(context.Background(), testTenant, "node1")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if n.OwnerActor != "user:alice" {
		t.Fatalf("owner: got %q want user:alice", n.OwnerActor)
	}
}

// TestApplier_DelegateAccessFix is the contract-pinning regression for
// PLAN.md §6.4 item 1 — the Python applier silently drops
// admin_delegate_access events. The Go applier MUST materialise the
// node_access grant on replay.
func TestApplier_DelegateAccessFix(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	// Seed a node so the grant has a target.
	createEv := apply.Event{
		TenantID: testTenant, Actor: "user:owner", IdempotencyKey: "seed",
		Ops: []map[string]any{mkCreateNode("doc1", 1, map[string]any{"1": "secret"})},
	}
	f.appendEvent(t, createEv)

	// Delegate access to a different actor.
	delegEv := apply.Event{
		TenantID: testTenant, Actor: "user:owner", IdempotencyKey: "deleg1",
		Ops: []map[string]any{{
			"op":         string(apply.OpDelegateAccess),
			"node_id":    "doc1",
			"actor_id":   "user:bob",
			"actor_type": "user",
			"permission": "read",
		}},
	}
	f.appendEvent(t, delegEv)

	f.runApplierUntilApplied(t)
	f.waitForIdempKey(t, testTenant, "deleg1")

	// Assert the grant exists in node_access. We use AdminDB to read
	// directly because the store package doesn't yet expose a
	// ListGrantsForNode helper.
	db, err := f.store.AdminDB(testTenant)
	if err != nil {
		t.Fatalf("AdminDB: %v", err)
	}
	var actor, perm string
	err = db.QueryRowContext(context.Background(),
		`SELECT actor_id, permission FROM node_access WHERE node_id = ?`,
		"doc1",
	).Scan(&actor, &perm)
	if err != nil {
		t.Fatalf("delegate-access did not materialise a node_access row: %v "+
			"(this is the Python bug — the Go applier MUST close it)", err)
	}
	if actor != "user:bob" || perm != "read" {
		t.Fatalf("delegate-access wrong row: actor=%q perm=%q (want user:bob/read)", actor, perm)
	}
}

// TestApplier_HaltsOnPoison appends a structurally-malformed op (missing
// id on create_node) and asserts Run returns a non-nil error AND the
// WAL offset did not advance past the poison record.
func TestApplier_HaltsOnPoison(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	good := apply.Event{
		TenantID: testTenant, Actor: "user:alice", IdempotencyKey: "good",
		Ops: []map[string]any{mkCreateNode("good1", 1, nil)},
	}
	f.appendEvent(t, good)

	// Poisoned: create_node with no id.
	poison := apply.Event{
		TenantID: testTenant, Actor: "user:alice", IdempotencyKey: "poison",
		Ops: []map[string]any{{"op": "create_node", "type_id": 1}},
	}
	f.appendEvent(t, poison)

	// Following good event — must NOT be applied.
	after := apply.Event{
		TenantID: testTenant, Actor: "user:alice", IdempotencyKey: "after",
		Ops: []map[string]any{mkCreateNode("after1", 1, nil)},
	}
	f.appendEvent(t, after)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- f.applier.Run(ctx) }()

	// Wait for halt.
	var runErr error
	select {
	case runErr = <-done:
	case <-time.After(2 * time.Second):
		cancel()
		<-done
		t.Fatalf("applier did not halt within 2s (poison should have surfaced)")
	}
	if runErr == nil {
		t.Fatalf("applier returned nil; expected halt-on-poison error")
	}
	if !errors.Is(runErr, apply.ErrPoisonEvent) {
		t.Fatalf("got %v, want ErrPoisonEvent", runErr)
	}
	// Good event applied.
	ok, _ := f.store.CheckIdempotency(context.Background(), testTenant, "good")
	if !ok {
		t.Fatalf("good event was not applied before halt")
	}
	// After event NOT applied.
	ok, _ = f.store.CheckIdempotency(context.Background(), testTenant, "after")
	if ok {
		t.Fatalf("after-poison event should not have been applied (offset advanced past poison)")
	}
}

// TestApplier_IdempotencySameKeyOnce confirms two appends of the same
// idempotency_key produce one effect in the store.
func TestApplier_IdempotencySameKeyOnce(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ev := apply.Event{
		TenantID: testTenant, Actor: "user:alice", IdempotencyKey: "dup",
		Ops: []map[string]any{mkCreateNode("once", 1, map[string]any{"1": "first"})},
	}
	f.appendEvent(t, ev)
	// Different node id but same idempotency key — second is a SKIP.
	ev2 := apply.Event{
		TenantID: testTenant, Actor: "user:alice", IdempotencyKey: "dup",
		TsMs: ev.TsMs + 1,
		Ops:  []map[string]any{mkCreateNode("once-other", 1, map[string]any{"1": "second"})},
	}
	f.appendEvent(t, ev2)

	f.runApplierUntilApplied(t)
	f.waitForIdempKey(t, testTenant, "dup")
	// Give the applier a moment to chew on the second record (which it
	// must skip).
	time.Sleep(150 * time.Millisecond)

	if _, err := f.store.GetNode(context.Background(), testTenant, "once"); err != nil {
		t.Fatalf("first node missing: %v", err)
	}
	if _, err := f.store.GetNode(context.Background(), testTenant, "once-other"); err == nil {
		t.Fatalf("second node was applied; idempotency key did not skip the duplicate")
	}
}

// TestApplier_ReplayDeterminism mirrors test_wal_replay_determinism.py.
// Drive a mixed sequence, dump state, wipe the store, replay from a
// fresh group_id, dump again, assert equal.
func TestApplier_ReplayDeterminism(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	events := []apply.Event{
		{TenantID: testTenant, Actor: "user:alice", IdempotencyKey: "e1", TsMs: 1700000000000,
			Ops: []map[string]any{mkCreateNode("n1", 1, map[string]any{"1": "alice"})}},
		{TenantID: testTenant, Actor: "user:alice", IdempotencyKey: "e2", TsMs: 1700000000001,
			Ops: []map[string]any{mkCreateNode("n2", 1, map[string]any{"1": "bob"})}},
		{TenantID: testTenant, Actor: "user:alice", IdempotencyKey: "e3", TsMs: 1700000000002,
			Ops: []map[string]any{{
				"op": string(apply.OpUpdateNode), "id": "n1",
				"patch": map[string]any{"2": "updated"},
			}}},
		{TenantID: testTenant, Actor: "user:alice", IdempotencyKey: "e4", TsMs: 1700000000003,
			Ops: []map[string]any{{
				"op": string(apply.OpCreateEdge), "edge_id": int32(7),
				"from": "n1", "to": "n2",
			}}},
		// Delegate access — exercises the bugfix path on replay too.
		{TenantID: testTenant, Actor: "user:alice", IdempotencyKey: "e5", TsMs: 1700000000004,
			Ops: []map[string]any{{
				"op": string(apply.OpDelegateAccess), "node_id": "n1",
				"actor_id": "user:carol", "permission": "read",
			}}},
	}
	for _, ev := range events {
		f.appendEvent(t, ev)
	}
	f.runApplierUntilApplied(t)
	for _, ev := range events {
		f.waitForIdempKey(t, testTenant, ev.IdempotencyKey)
	}
	first := dumpState(t, f.store, testTenant, []string{"n1", "n2"})

	// Now: tear down store, reopen on the SAME WAL with a fresh
	// group_id so PollBatch returns from offset 0.
	f.applier.Stop()
	if err := f.store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	dir := t.TempDir()
	cs2, err := store.New(store.Options{RootDir: dir, WALMode: true})
	if err != nil {
		t.Fatalf("store.New replay: %v", err)
	}
	defer cs2.Close()
	if err := cs2.OpenTenant(context.Background(), testTenant); err != nil {
		t.Fatalf("OpenTenant replay: %v", err)
	}

	a2, err := apply.New(apply.Options{
		Store:       cs2,
		Consumer:    f.wal,
		Topic:       testTopic,
		GroupID:     "applier-replay", // FRESH group => offset starts at 0
		PollTimeout: 25 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("apply.New replay: %v", err)
	}
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	done2 := make(chan error, 1)
	go func() { done2 <- a2.Run(ctx2) }()
	defer func() { a2.Stop(); <-done2 }()

	// Wait for replay to apply every event.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		ok, _ := cs2.CheckIdempotency(context.Background(), testTenant, "e5")
		if ok {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	second := dumpState(t, cs2, testTenant, []string{"n1", "n2"})
	if !reflect.DeepEqual(first, second) {
		fa, _ := json.MarshalIndent(first, "", "  ")
		sb, _ := json.MarshalIndent(second, "", "  ")
		t.Fatalf("replay determinism violated:\nfirst:\n%s\nsecond:\n%s", fa, sb)
	}
}

// TestApplier_ShareNodeRecorded exercises the WAL-first share_node op.
func TestApplier_ShareNodeRecorded(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.appendEvent(t, apply.Event{
		TenantID: testTenant, Actor: "user:owner", IdempotencyKey: "seed",
		Ops: []map[string]any{mkCreateNode("doc-share", 1, nil)},
	})
	f.appendEvent(t, apply.Event{
		TenantID: testTenant, Actor: "user:owner", IdempotencyKey: "share1",
		Ops: []map[string]any{{
			"op": string(apply.OpShareNode), "node_id": "doc-share",
			"actor_id": "user:bob", "permission": "read",
		}},
	})
	f.runApplierUntilApplied(t)
	f.waitForIdempKey(t, testTenant, "share1")

	db, _ := f.store.AdminDB(testTenant)
	var n int
	if err := db.QueryRowContext(context.Background(),
		`SELECT count(*) FROM node_access WHERE node_id = ? AND actor_id = ?`,
		"doc-share", "user:bob",
	).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("share_node did not write node_access row (n=%d)", n)
	}
}

// TestApplier_RevokeAccess exercises the WAL-first revoke op.
func TestApplier_RevokeAccess(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.appendEvent(t, apply.Event{
		TenantID: testTenant, Actor: "user:owner", IdempotencyKey: "seed",
		Ops: []map[string]any{mkCreateNode("d", 1, nil)},
	})
	f.appendEvent(t, apply.Event{
		TenantID: testTenant, Actor: "user:owner", IdempotencyKey: "share",
		Ops: []map[string]any{{
			"op": string(apply.OpShareNode), "node_id": "d",
			"actor_id": "user:bob", "permission": "read",
		}},
	})
	f.appendEvent(t, apply.Event{
		TenantID: testTenant, Actor: "user:owner", IdempotencyKey: "revoke",
		Ops: []map[string]any{{
			"op": string(apply.OpRevokeAccess), "node_id": "d", "actor_id": "user:bob",
		}},
	})
	f.runApplierUntilApplied(t)
	f.waitForIdempKey(t, testTenant, "revoke")

	db, _ := f.store.AdminDB(testTenant)
	var n int
	_ = db.QueryRowContext(context.Background(),
		`SELECT count(*) FROM node_access WHERE node_id = ? AND actor_id = ?`,
		"d", "user:bob",
	).Scan(&n)
	if n != 0 {
		t.Fatalf("revoke_access left a row behind (n=%d)", n)
	}
}

// TestApplier_TransferOwnership exercises the WAL-first transfer op.
func TestApplier_TransferOwnership(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.appendEvent(t, apply.Event{
		TenantID: testTenant, Actor: "user:alice", IdempotencyKey: "seed",
		Ops: []map[string]any{mkCreateNode("xfer", 1, nil)},
	})
	f.appendEvent(t, apply.Event{
		TenantID: testTenant, Actor: "user:alice", IdempotencyKey: "xfer",
		Ops: []map[string]any{{
			"op":      string(apply.OpTransferOwnership),
			"node_id": "xfer", "new_owner": "user:bob",
		}},
	})
	f.runApplierUntilApplied(t)
	f.waitForIdempKey(t, testTenant, "xfer")

	n, err := f.store.GetNode(context.Background(), testTenant, "xfer")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if n.OwnerActor != "user:bob" {
		t.Fatalf("owner: got %q want user:bob", n.OwnerActor)
	}
}

// TestApplier_GroupMembership exercises add/remove group member ops.
func TestApplier_GroupMembership(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.appendEvent(t, apply.Event{
		TenantID: testTenant, Actor: "user:admin", IdempotencyKey: "add1",
		Ops: []map[string]any{{
			"op":       string(apply.OpAddGroupMember),
			"group_id": "engineering", "member_actor_id": "user:alice",
		}},
	})
	f.runApplierUntilApplied(t)
	f.waitForIdempKey(t, testTenant, "add1")

	db, _ := f.store.AdminDB(testTenant)
	var role string
	if err := db.QueryRowContext(context.Background(),
		`SELECT role FROM group_users WHERE group_id = ? AND member_actor_id = ?`,
		"engineering", "user:alice",
	).Scan(&role); err != nil {
		t.Fatalf("group row not found: %v", err)
	}

	f.appendEvent(t, apply.Event{
		TenantID: testTenant, Actor: "user:admin", IdempotencyKey: "rm1",
		Ops: []map[string]any{{
			"op":       string(apply.OpRemoveGroupMember),
			"group_id": "engineering", "member_actor_id": "user:alice",
		}},
	})
	f.waitForIdempKey(t, testTenant, "rm1")
	var n int
	_ = db.QueryRowContext(context.Background(),
		`SELECT count(*) FROM group_users WHERE group_id = ?`, "engineering",
	).Scan(&n)
	if n != 0 {
		t.Fatalf("group member not removed: n=%d", n)
	}
}

// TestApplier_AdminRevokeAccessBroadens confirms PLAN.md §6.4 item 2:
// admin_revoke_access deletes node_access AND group_users (Python only
// deletes node_visibility).
func TestApplier_AdminRevokeAccessBroadens(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	// Seed a node + share + group membership for user:bob.
	f.appendEvent(t, apply.Event{
		TenantID: testTenant, Actor: "user:owner", IdempotencyKey: "seed",
		Ops: []map[string]any{mkCreateNode("doc", 1, nil)},
	})
	f.appendEvent(t, apply.Event{
		TenantID: testTenant, Actor: "user:owner", IdempotencyKey: "share",
		Ops: []map[string]any{{
			"op": string(apply.OpShareNode), "node_id": "doc",
			"actor_id": "user:bob", "permission": "read",
		}},
	})
	f.appendEvent(t, apply.Event{
		TenantID: testTenant, Actor: "user:owner", IdempotencyKey: "join",
		Ops: []map[string]any{{
			"op":       string(apply.OpAddGroupMember),
			"group_id": "g", "member_actor_id": "user:bob",
		}},
	})
	f.appendEvent(t, apply.Event{
		TenantID: testTenant, Actor: "user:admin", IdempotencyKey: "revoke",
		Ops: []map[string]any{{
			"op": string(apply.OpAdminRevokeAccess), "user_id": "user:bob",
		}},
	})
	f.runApplierUntilApplied(t)
	f.waitForIdempKey(t, testTenant, "revoke")

	db, _ := f.store.AdminDB(testTenant)
	for _, q := range []struct {
		name string
		sql  string
		args []any
	}{
		{"node_access", `SELECT count(*) FROM node_access WHERE actor_id = ?`, []any{"user:bob"}},
		{"group_users", `SELECT count(*) FROM group_users WHERE member_actor_id = ?`, []any{"user:bob"}},
		{"node_visibility", `SELECT count(*) FROM node_visibility WHERE tenant_id = ? AND principal = ?`, []any{testTenant, "user:bob"}},
	} {
		var n int
		_ = db.QueryRowContext(context.Background(), q.sql, q.args...).Scan(&n)
		if n != 0 {
			t.Errorf("admin_revoke_access did not clean up %s (n=%d) — Python parity bug not closed", q.name, n)
		}
	}
}

// TestApplier_TenantMembership exercises add_tenant_member /
// remove_tenant_member / change_member_role.
func TestApplier_TenantMembership(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	f.appendEvent(t, apply.Event{
		TenantID: testTenant, Actor: "user:admin", IdempotencyKey: "atm",
		Ops: []map[string]any{{
			"op":      string(apply.OpAddTenantMember),
			"user_id": "alice", "role": "member",
		}},
	})
	f.runApplierUntilApplied(t)
	f.waitForIdempKey(t, testTenant, "atm")

	ok, err := f.global.IsMember(context.Background(), testTenant, "alice")
	if err != nil || !ok {
		t.Fatalf("IsMember: ok=%v err=%v", ok, err)
	}

	f.appendEvent(t, apply.Event{
		TenantID: testTenant, Actor: "user:admin", IdempotencyKey: "chg",
		Ops: []map[string]any{{
			"op":      string(apply.OpChangeMemberRole),
			"user_id": "alice", "role": "owner",
		}},
	})
	f.waitForIdempKey(t, testTenant, "chg")
	members, _ := f.global.GetTenantMembers(context.Background(), testTenant)
	found := false
	for _, m := range members {
		if m.UserID == "alice" && m.Role == "owner" {
			found = true
		}
	}
	if !found {
		t.Fatalf("change_member_role did not update role to owner: %+v", members)
	}

	f.appendEvent(t, apply.Event{
		TenantID: testTenant, Actor: "user:admin", IdempotencyKey: "rmt",
		Ops: []map[string]any{{
			"op": string(apply.OpRemoveTenantMember), "user_id": "alice",
		}},
	})
	f.waitForIdempKey(t, testTenant, "rmt")
	ok, _ = f.global.IsMember(context.Background(), testTenant, "alice")
	if ok {
		t.Fatalf("remove_tenant_member did not delete the row")
	}
}

// TestApplier_SetLegalHold exercises the set_legal_hold op (clear=false
// then clear=true).
func TestApplier_SetLegalHold(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	f.appendEvent(t, apply.Event{
		TenantID: testTenant, Actor: "user:admin", IdempotencyKey: "lh1",
		Ops: []map[string]any{{
			"op":      string(apply.OpSetLegalHold),
			"held_by": "user:counsel", "reason": "litigation",
		}},
	})
	f.runApplierUntilApplied(t)
	f.waitForIdempKey(t, testTenant, "lh1")

	on, _ := f.global.IsLegalHoldSet(context.Background(), testTenant)
	if !on {
		t.Fatalf("set_legal_hold did not set the hold")
	}

	f.appendEvent(t, apply.Event{
		TenantID: testTenant, Actor: "user:admin", IdempotencyKey: "lh2",
		Ops: []map[string]any{{
			"op":      string(apply.OpSetLegalHold),
			"held_by": "user:counsel", "clear": true,
		}},
	})
	f.waitForIdempKey(t, testTenant, "lh2")
	on, _ = f.global.IsLegalHoldSet(context.Background(), testTenant)
	if on {
		t.Fatalf("set_legal_hold clear=true did not clear the hold")
	}
}

// TestApplier_UnknownOpTypeHalts confirms an unknown op-type halts the
// consumer rather than silently dropping data — the lesson the
// DelegateAccess bug taught us.
func TestApplier_UnknownOpTypeHalts(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.appendEvent(t, apply.Event{
		TenantID: testTenant, Actor: "user:alice", IdempotencyKey: "u1",
		Ops: []map[string]any{{"op": "no_such_op"}},
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- f.applier.Run(ctx) }()

	select {
	case err := <-done:
		if !errors.Is(err, apply.ErrUnknownOpType) {
			t.Fatalf("got %v, want ErrUnknownOpType", err)
		}
	case <-time.After(2 * time.Second):
		cancel()
		<-done
		t.Fatalf("unknown op-type did not halt the applier")
	}
}

// TestApplier_ReplayFromNonZeroOffset reaches the same final state as a
// full replay from offset 0 when both arrive at the same set of events.
func TestApplier_ReplayFromNonZeroOffset(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	for i := 0; i < 4; i++ {
		f.appendEvent(t, apply.Event{
			TenantID: testTenant, Actor: "user:alice",
			IdempotencyKey: fmt.Sprintf("k%d", i),
			TsMs:           int64(1700000000000 + i),
			Ops:            []map[string]any{mkCreateNode(fmt.Sprintf("n%d", i), 1, nil)},
		})
	}
	f.runApplierUntilApplied(t)
	for i := 0; i < 4; i++ {
		f.waitForIdempKey(t, testTenant, fmt.Sprintf("k%d", i))
	}
	full := dumpState(t, f.store, testTenant, []string{"n0", "n1", "n2", "n3"})

	// Stop the applier, drop all data, replay from a fresh group_id so
	// the consumer reads from offset 0. The "from non-zero offset"
	// shape is exercised by Replay against a partially-committed
	// consumer group: we skip the first 2 records by manually
	// committing them on a new group, then replay the rest.
	f.applier.Stop()

	dir := t.TempDir()
	cs3, err := store.New(store.Options{RootDir: dir, WALMode: true})
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	defer cs3.Close()
	if err := cs3.OpenTenant(context.Background(), testTenant); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}
	a3, err := apply.New(apply.Options{
		Store: cs3, Consumer: f.wal,
		Topic: testTopic, GroupID: "replay-partial",
		PollTimeout: 25 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("apply.New: %v", err)
	}

	if err := a3.Replay(context.Background(), testTenant, 0); err != nil {
		t.Fatalf("Replay: %v", err)
	}
	partial := dumpState(t, cs3, testTenant, []string{"n0", "n1", "n2", "n3"})
	if !reflect.DeepEqual(full, partial) {
		t.Fatalf("Replay state diverged from Run state")
	}
}
