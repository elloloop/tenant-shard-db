// SPDX-License-Identifier: AGPL-3.0-only

// Tests for TransferUserContent. Pin set is documented in
// docs/go-port/rpcs/TransferUserContent.md "Contract tests pinning
// behavior". The Go-side concerns this file enforces:
//
//  1. Admin happy path, small batch (single WAL event).
//  2. Admin large-batch path: owned-set straddles the chunk size,
//     producing multiple WAL events of `admin_transfer_content`.
//  3. Recipient sees all transferred nodes after the applier consumes
//     the WAL events (visibility refresh wiring).
//  4. Non-admin caller -> PERMISSION_DENIED + zero WAL events appended
//     (privilege-escalation regression).

package api_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	"github.com/elloloop/tenant-shard-db/server/go/internal/apply"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

// transferFixture wires globalstore + canonical store + in-memory WAL
// + applier so a test can call srv.TransferUserContent and observe
// both the WAL trace and the post-apply SQLite state.
type transferFixture struct {
	srv       *api.Server
	gs        *globalstore.GlobalStore
	cs        *store.CanonicalStore
	w         *wal.InMemory
	applier   *apply.Applier
	stopApply context.CancelFunc
	doneApply chan error
	tenantID  string
}

func newTransferFixture(t *testing.T) *transferFixture {
	t.Helper()
	ctx := context.Background()

	gs := newGlobalStore(t)
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", ""); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	// Seed canonical members. alice = owner (admin power), carol = plain
	// member (used as the negative-auth caller).
	if err := gs.AddTenantMember(ctx, "acme", "alice", "owner"); err != nil {
		t.Fatalf("AddTenantMember alice: %v", err)
	}
	if err := gs.AddTenantMember(ctx, "acme", "carol", "member"); err != nil {
		t.Fatalf("AddTenantMember carol: %v", err)
	}

	cs, err := store.New(store.Options{
		RootDir: t.TempDir(),
		WALMode: true,
	})
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	if err := cs.OpenTenant(ctx, "acme"); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}

	w := wal.NewInMemory(1)

	srv := api.New(
		api.WithGlobalStore(gs),
		api.WithStore(cs),
		api.WithWALProducer(w),
	)

	return &transferFixture{
		srv:      srv,
		gs:       gs,
		cs:       cs,
		w:        w,
		tenantID: "acme",
	}
}

// startApplier launches the apply.Applier in a background goroutine so
// the test can observe post-apply SQLite state. Returns a stop func
// the test must invoke before assertions on terminal state.
func (f *transferFixture) startApplier(t *testing.T) {
	t.Helper()
	a, err := apply.New(apply.Options{
		Store:    f.cs,
		Consumer: f.w,
		Topic:    "entdb-wal",
		GroupID:  "test-applier",
	})
	if err != nil {
		t.Fatalf("apply.New: %v", err)
	}
	f.applier = a
	ctx, cancel := context.WithCancel(context.Background())
	f.stopApply = cancel
	f.doneApply = make(chan error, 1)
	go func() { f.doneApply <- a.Run(ctx) }()
}

func (f *transferFixture) stopApplier(t *testing.T) {
	t.Helper()
	if f.stopApply == nil {
		return
	}
	f.stopApply()
	select {
	case <-f.doneApply:
	case <-time.After(2 * time.Second):
		t.Fatalf("applier did not stop")
	}
}

// seedNodes inserts `count` nodes owned by `owner` in tenant "acme".
func (f *transferFixture) seedNodes(t *testing.T, owner string, count int) []string {
	t.Helper()
	ctx := context.Background()
	ids := make([]string, 0, count)
	for i := 0; i < count; i++ {
		id := uniqueNodeID(i)
		ids = append(ids, id)
		if _, err := f.cs.CreateNodeRaw(ctx, f.tenantID, store.NodeInput{
			NodeID:     id,
			TypeID:     1,
			Payload:    map[string]any{"1": "x"},
			OwnerActor: owner,
		}); err != nil {
			t.Fatalf("CreateNodeRaw[%d]: %v", i, err)
		}
	}
	return ids
}

func uniqueNodeID(i int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz"
	if i < len(alphabet) {
		return "n-" + string(alphabet[i])
	}
	// 4-byte rolling base for >26 nodes.
	a := alphabet[i%len(alphabet)]
	b := alphabet[(i/len(alphabet))%len(alphabet)]
	c := alphabet[(i/(len(alphabet)*len(alphabet)))%len(alphabet)]
	return "n-" + string([]byte{a, b, c})
}

// drainWAL pulls every record currently in the in-memory WAL topic and
// returns the decoded events (in offset order). Reads via PollBatch
// with a fresh group id so the calls don't mutate the applier's
// committed offset.
func (f *transferFixture) drainWAL(t *testing.T) []wal.Event {
	t.Helper()
	records, err := f.w.PollBatch(
		context.Background(), "entdb-wal", "test-drain", 1000, 50*time.Millisecond,
	)
	if err != nil {
		t.Fatalf("PollBatch: %v", err)
	}
	out := make([]wal.Event, 0, len(records))
	for _, r := range records {
		ev, err := wal.DecodeEvent(r.Value)
		if err != nil {
			t.Fatalf("DecodeEvent: %v", err)
		}
		out = append(out, ev)
	}
	return out
}

// TestTransferUserContent_AdminHappySmallBatch: the canonical
// happy-path. An owner (alice) transfers a handful of nodes to a new
// user (bob). The handler must emit exactly one WAL event with op
// "admin_transfer_content" and return success=true with the pre-apply
// count.
func TestTransferUserContent_AdminHappySmallBatch(t *testing.T) {
	t.Parallel()
	f := newTransferFixture(t)
	f.seedNodes(t, "user:dave", 5)

	resp, err := f.srv.TransferUserContent(
		context.Background(), &pb.TransferUserContentRequest{
			Actor:    "user:alice",
			TenantId: f.tenantID,
			FromUser: "user:dave",
			ToUser:   "user:bob",
		},
	)
	if err != nil {
		t.Fatalf("TransferUserContent: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("Success: false; resp=%+v", resp)
	}
	if got, want := resp.GetTransferred(), int32(5); got != want {
		t.Fatalf("Transferred = %d, want %d", got, want)
	}

	// WAL trace: exactly one event, exactly one op, op == admin_transfer_content.
	events := f.drainWAL(t)
	if len(events) != 1 {
		t.Fatalf("WAL events: got %d, want 1", len(events))
	}
	ev := events[0]
	if ev.TenantID != f.tenantID {
		t.Fatalf("event tenant = %q, want %q", ev.TenantID, f.tenantID)
	}
	if ev.Actor != "user:alice" {
		t.Fatalf("event actor = %q, want %q", ev.Actor, "user:alice")
	}
	if !strings.HasPrefix(ev.IdempotencyKey, "admin-transfer-") {
		t.Fatalf("idempotency_key = %q, want prefix %q", ev.IdempotencyKey, "admin-transfer-")
	}
	if len(ev.Ops) != 1 {
		t.Fatalf("ops: got %d, want 1", len(ev.Ops))
	}
	if got, _ := ev.Ops[0]["op"].(string); got != "admin_transfer_content" {
		t.Fatalf("op = %q, want admin_transfer_content", got)
	}
	if got, _ := ev.Ops[0]["from_user"].(string); got != "user:dave" {
		t.Fatalf("from_user = %q, want user:dave", got)
	}
	if got, _ := ev.Ops[0]["to_user"].(string); got != "user:bob" {
		t.Fatalf("to_user = %q, want user:bob", got)
	}

	// Membership upsert is visible in the global store.
	members, err := f.gs.GetTenantMembers(context.Background(), f.tenantID)
	if err != nil {
		t.Fatalf("GetTenantMembers: %v", err)
	}
	hasBob := false
	for _, m := range members {
		if m.UserID == "user:bob" {
			hasBob = true
		}
	}
	if !hasBob {
		t.Fatalf("expected user:bob to be a tenant member after transfer")
	}
}

// TestTransferUserContent_AdminLargeBatchChunked: when the owner's
// node count exceeds transferUserContentChunkSize (1000) the handler
// must emit multiple WAL events. Pinning the chunk-count contract
// guards the spec "Open question" §2 mitigation.
func TestTransferUserContent_AdminLargeBatchChunked(t *testing.T) {
	t.Parallel()
	f := newTransferFixture(t)
	// 1500 owned nodes -> 2 chunks (1000 + 500) under chunk size 1000.
	f.seedNodes(t, "user:dave", 1500)

	resp, err := f.srv.TransferUserContent(
		context.Background(), &pb.TransferUserContentRequest{
			Actor:    "user:alice",
			TenantId: f.tenantID,
			FromUser: "user:dave",
			ToUser:   "user:bob",
		},
	)
	if err != nil {
		t.Fatalf("TransferUserContent: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("Success: false; resp=%+v", resp)
	}
	if got, want := resp.GetTransferred(), int32(1500); got != want {
		t.Fatalf("Transferred = %d, want %d", got, want)
	}

	events := f.drainWAL(t)
	if len(events) < 2 {
		t.Fatalf("WAL events: got %d, want >= 2 (chunked)", len(events))
	}
	// Total node_ids across chunks must equal 1500.
	total := 0
	for _, ev := range events {
		if got, _ := ev.Ops[0]["op"].(string); got != "admin_transfer_content" {
			t.Fatalf("op = %q, want admin_transfer_content", got)
		}
		ids, _ := ev.Ops[0]["node_ids"].([]any)
		total += len(ids)
	}
	if total != 1500 {
		t.Fatalf("sum(node_ids) across chunks = %d, want 1500", total)
	}
}

// TestTransferUserContent_RecipientSeesNodesAfterApply: drives the
// full WAL-first flow. A populated owner gets transferred; once the
// applier consumes the events, querying SQLite by (tenant, owner=bob)
// returns every original node.
func TestTransferUserContent_RecipientSeesNodesAfterApply(t *testing.T) {
	t.Parallel()
	f := newTransferFixture(t)
	originalIDs := f.seedNodes(t, "user:dave", 7)

	f.startApplier(t)
	t.Cleanup(func() { f.stopApplier(t) })

	resp, err := f.srv.TransferUserContent(
		context.Background(), &pb.TransferUserContentRequest{
			Actor:    "user:alice",
			TenantId: f.tenantID,
			FromUser: "user:dave",
			ToUser:   "user:bob",
		},
	)
	if err != nil {
		t.Fatalf("TransferUserContent: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("Success: false; resp=%+v", resp)
	}

	// Wait for the applier to converge: poll CountOwnedNodes(bob) until
	// it equals len(originalIDs). The Applier handler runs the bulk
	// UPDATE inside a single SQLite write, so the count flips
	// atomically once per consumed event.
	deadline := time.Now().Add(3 * time.Second)
	for {
		n, err := f.cs.CountOwnedNodes(context.Background(), f.tenantID, "user:bob")
		if err != nil {
			t.Fatalf("CountOwnedNodes(bob): %v", err)
		}
		if int(n) == len(originalIDs) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("applier did not converge: bob owns %d, want %d",
				n, len(originalIDs))
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Dave must own zero nodes post-apply.
	if n, err := f.cs.CountOwnedNodes(
		context.Background(), f.tenantID, "user:dave",
	); err != nil || n != 0 {
		t.Fatalf("CountOwnedNodes(dave) = %d, err=%v; want 0/nil", n, err)
	}

	// Sanity: every original node still exists, owned by bob.
	for _, id := range originalIDs {
		got, err := f.cs.GetNode(context.Background(), f.tenantID, id)
		if err != nil {
			t.Fatalf("GetNode %q: %v", id, err)
		}
		if got.OwnerActor != "user:bob" {
			t.Fatalf("node %q owner = %q, want user:bob", id, got.OwnerActor)
		}
	}
}

// TestTransferUserContent_NonAdminPermissionDenied: a plain tenant
// member (carol) attempting the transfer must be rejected with
// PERMISSION_DENIED, AND no WAL event must be appended (the
// privilege-escalation regression pinned by Python at
// test_privilege_escalation.py:344-365).
func TestTransferUserContent_NonAdminPermissionDenied(t *testing.T) {
	t.Parallel()
	f := newTransferFixture(t)
	f.seedNodes(t, "user:dave", 3)

	_, err := f.srv.TransferUserContent(
		context.Background(), &pb.TransferUserContentRequest{
			Actor:    "user:carol",
			TenantId: f.tenantID,
			FromUser: "user:dave",
			ToUser:   "user:bob",
		},
	)
	if err == nil {
		t.Fatalf("expected PERMISSION_DENIED, got nil")
	}
	if got := errs.Code(err); got != codes.PermissionDenied {
		t.Fatalf("code = %v, want PermissionDenied (err=%v)", got, err)
	}

	// No WAL events: the auth check must fail BEFORE the append.
	events := f.drainWAL(t)
	if len(events) != 0 {
		t.Fatalf("WAL events on denied call: got %d, want 0; first=%s",
			len(events), mustJSON(t, events[0]))
	}

	// Membership unchanged: bob must NOT have been silently upserted by
	// the rejected call (auth runs before the global-store write).
	members, err := f.gs.GetTenantMembers(context.Background(), f.tenantID)
	if err != nil {
		t.Fatalf("GetTenantMembers: %v", err)
	}
	for _, m := range members {
		if m.UserID == "user:bob" {
			t.Fatalf("user:bob upserted on denied call: %+v", m)
		}
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return string(b)
}
