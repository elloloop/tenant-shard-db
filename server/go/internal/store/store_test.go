// Unit tests for the store package. modernc.org/sqlite is CGO-free so
// every t.TempDir() works on every CI runner. Each test opens a fresh
// store + tenant; tests are self-contained and parallel-safe.

package store_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	entcrypto "github.com/elloloop/tenant-shard-db/server/go/internal/crypto"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

// newStore returns a tmpdir-backed store with WAL on. Cleans up on test
// completion.
func newStore(t *testing.T) *store.CanonicalStore {
	t.Helper()
	dir := t.TempDir()
	cs, err := store.New(store.Options{
		RootDir: dir,
		WALMode: true,
	})
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func TestOpenTenantIsIdempotent(t *testing.T) {
	cs := newStore(t)
	ctx := context.Background()
	if err := cs.OpenTenant(ctx, "tenant1"); err != nil {
		t.Fatalf("OpenTenant 1: %v", err)
	}
	// Second open is a no-op.
	if err := cs.OpenTenant(ctx, "tenant1"); err != nil {
		t.Fatalf("OpenTenant 2: %v", err)
	}
}

func TestSchemaInitIdempotent(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 3; i++ {
		cs, err := store.New(store.Options{RootDir: dir, WALMode: true})
		if err != nil {
			t.Fatalf("store.New iter=%d: %v", i, err)
		}
		if err := cs.OpenTenant(context.Background(), "t1"); err != nil {
			t.Fatalf("OpenTenant iter=%d: %v", i, err)
		}
		if err := cs.Close(); err != nil {
			t.Fatalf("Close iter=%d: %v", i, err)
		}
	}
	// Sanity: file is on disk where we expect it.
	if _, err := filepath.Abs(filepath.Join(dir, "tenant_t1.db")); err != nil {
		t.Fatalf("path: %v", err)
	}
}

func TestEncryptedTenantDatabaseUnreadableWithoutKey(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	manager, err := entcrypto.NewKeyManager(testMaster(0x31), nil)
	if err != nil {
		t.Fatalf("NewKeyManager: %v", err)
	}
	cs, err := store.New(store.Options{
		RootDir:            dir,
		WALMode:            true,
		KeyManager:         manager,
		EncryptionRequired: true,
	})
	if err != nil {
		t.Fatalf("store.New encrypted: %v", err)
	}
	if err := cs.OpenTenant(ctx, "tenant1"); err != nil {
		t.Fatalf("OpenTenant encrypted: %v", err)
	}
	if _, err := cs.CreateNodeRaw(ctx, "tenant1", store.NodeInput{
		NodeID:     "encrypted-node",
		TypeID:     7,
		OwnerActor: "user:alice",
		Payload:    map[string]any{"1": "secret"},
	}); err != nil {
		t.Fatalf("CreateNodeRaw encrypted: %v", err)
	}
	path, err := cs.TenantDBPath("tenant1")
	if err != nil {
		t.Fatalf("TenantDBPath: %v", err)
	}
	if err := cs.Close(); err != nil {
		t.Fatalf("Close encrypted store: %v", err)
	}

	encrypted, hasDB, err := entcrypto.SQLiteFileEncryptionStatus(path)
	if err != nil {
		t.Fatalf("SQLiteFileEncryptionStatus: %v", err)
	}
	if !hasDB || !encrypted {
		t.Fatalf("tenant db encryption status encrypted=%v hasDB=%v, want encrypted database", encrypted, hasDB)
	}

	raw, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("open raw sqlite: %v", err)
	}
	defer raw.Close()
	var n int
	if err := raw.QueryRowContext(ctx, `SELECT count(*) FROM nodes`).Scan(&n); err == nil {
		t.Fatalf("raw sqlite read succeeded with count=%d; want encrypted file to be unreadable", n)
	}

	reopened, err := store.New(store.Options{
		RootDir:            dir,
		WALMode:            true,
		KeyManager:         manager,
		EncryptionRequired: true,
	})
	if err != nil {
		t.Fatalf("store.New encrypted reopen: %v", err)
	}
	defer reopened.Close()
	if err := reopened.OpenTenant(ctx, "tenant1"); err != nil {
		t.Fatalf("OpenTenant encrypted reopen: %v", err)
	}
	if _, err := reopened.GetNode(ctx, "tenant1", "encrypted-node"); err != nil {
		t.Fatalf("GetNode encrypted reopen: %v", err)
	}
}

func TestEncryptionRequiredNeedsKeyManager(t *testing.T) {
	_, err := store.New(store.Options{
		RootDir:            t.TempDir(),
		WALMode:            true,
		EncryptionRequired: true,
	})
	if err == nil {
		t.Fatal("store.New encryption required without key manager err = nil, want error")
	}
	if !strings.Contains(err.Error(), "encryption required") {
		t.Fatalf("store.New error = %q, want encryption required", err)
	}
}

func TestEncryptedStoreRefusesPlaintextTenantFile(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	plain, err := store.New(store.Options{RootDir: dir, WALMode: true})
	if err != nil {
		t.Fatalf("store.New plaintext: %v", err)
	}
	if err := plain.OpenTenant(ctx, "tenant1"); err != nil {
		t.Fatalf("OpenTenant plaintext: %v", err)
	}
	if err := plain.Close(); err != nil {
		t.Fatalf("Close plaintext: %v", err)
	}

	manager, err := entcrypto.NewKeyManager(testMaster(0x32), nil)
	if err != nil {
		t.Fatalf("NewKeyManager: %v", err)
	}
	encrypted, err := store.New(store.Options{
		RootDir:            dir,
		WALMode:            true,
		KeyManager:         manager,
		EncryptionRequired: true,
	})
	if err != nil {
		t.Fatalf("store.New encrypted: %v", err)
	}
	defer encrypted.Close()
	err = encrypted.OpenTenant(ctx, "tenant1")
	if err == nil {
		t.Fatal("OpenTenant encrypted over plaintext err = nil, want error")
	}
	if !strings.Contains(err.Error(), "refusing to open unencrypted tenant DB") {
		t.Fatalf("OpenTenant error = %q, want unencrypted refusal", err)
	}
}

func TestInvalidTenantID(t *testing.T) {
	cs := newStore(t)
	ctx := context.Background()
	for _, bad := range []string{
		"", "tenant/1", "tenant.1", "tenant 1", "../etc",
	} {
		if err := cs.OpenTenant(ctx, bad); err == nil {
			t.Fatalf("OpenTenant(%q): expected error, got nil", bad)
		} else if !errors.Is(err, errs.ErrInvalidArgument) {
			t.Fatalf("OpenTenant(%q): got %v, want errs.ErrInvalidArgument", bad, err)
		}
	}
}

func testMaster(fill byte) []byte {
	key := make([]byte, entcrypto.KeyLength)
	for i := range key {
		key[i] = fill
	}
	return key
}

func TestNodeRoundTrip(t *testing.T) {
	cs := newStore(t)
	ctx := context.Background()
	if err := cs.OpenTenant(ctx, "tenant1"); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}

	n, err := cs.CreateNodeRaw(ctx, "tenant1", store.NodeInput{
		NodeID:     "node-1",
		TypeID:     7,
		Payload:    map[string]any{"1": "alice", "2": int64(42)},
		OwnerActor: "user:alice",
	})
	if err != nil {
		t.Fatalf("CreateNodeRaw: %v", err)
	}
	if n.NodeID != "node-1" || n.TypeID != 7 || n.OwnerActor != "user:alice" {
		t.Fatalf("CreateNodeRaw returned %+v", n)
	}

	got, err := cs.GetNode(ctx, "tenant1", "node-1")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if got.NodeID != "node-1" || got.OwnerActor != "user:alice" {
		t.Fatalf("GetNode returned %+v", got)
	}

	// NotFound.
	_, err = cs.GetNode(ctx, "tenant1", "missing")
	if !errors.Is(err, store.ErrNodeNotFound) {
		t.Fatalf("GetNode missing: got %v, want ErrNodeNotFound", err)
	}
	if !errors.Is(err, errs.ErrNotFound) {
		t.Fatalf("GetNode missing: not wrapped as errs.ErrNotFound: %v", err)
	}
}

func TestUpdateNodeMerge(t *testing.T) {
	cs := newStore(t)
	ctx := context.Background()
	_ = cs.OpenTenant(ctx, "t1")
	_, _ = cs.CreateNodeRaw(ctx, "t1", store.NodeInput{
		NodeID: "n1", TypeID: 1, OwnerActor: "u",
		Payload: map[string]any{"1": "a", "2": "b"},
	})
	got, err := cs.UpdateNode(ctx, "t1", "n1", map[string]any{"2": "B", "3": "C"})
	if err != nil {
		t.Fatalf("UpdateNode: %v", err)
	}
	// Payload should have all three keys after merge.
	if got.PayloadJSON == "" {
		t.Fatalf("UpdateNode returned empty payload")
	}

	// UpdateNode missing -> ErrNodeNotFound.
	_, err = cs.UpdateNode(ctx, "t1", "missing", map[string]any{"x": 1})
	if !errors.Is(err, store.ErrNodeNotFound) {
		t.Fatalf("UpdateNode missing: got %v, want ErrNodeNotFound", err)
	}
}

func TestDeleteNode(t *testing.T) {
	cs := newStore(t)
	ctx := context.Background()
	_ = cs.OpenTenant(ctx, "t1")
	_, _ = cs.CreateNodeRaw(ctx, "t1", store.NodeInput{
		NodeID: "n1", TypeID: 1, OwnerActor: "u",
	})
	if err := cs.DeleteNode(ctx, "t1", "n1"); err != nil {
		t.Fatalf("DeleteNode: %v", err)
	}
	if err := cs.DeleteNode(ctx, "t1", "n1"); !errors.Is(err, store.ErrNodeNotFound) {
		t.Fatalf("DeleteNode missing: got %v, want ErrNodeNotFound", err)
	}
}

func TestGetNodesPartialMissing(t *testing.T) {
	cs := newStore(t)
	ctx := context.Background()
	_ = cs.OpenTenant(ctx, "t1")
	_, _ = cs.CreateNodeRaw(ctx, "t1", store.NodeInput{
		NodeID: "a", TypeID: 1, OwnerActor: "u",
	})
	_, _ = cs.CreateNodeRaw(ctx, "t1", store.NodeInput{
		NodeID: "b", TypeID: 1, OwnerActor: "u",
	})
	nodes, missing, err := cs.GetNodes(ctx, "t1", []string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("GetNodes: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("nodes len = %d, want 2", len(nodes))
	}
	if len(missing) != 1 || missing[0] != "c" {
		t.Fatalf("missing = %v, want [c]", missing)
	}
}

func TestQueryNodesEqualityFilter(t *testing.T) {
	cs := newStore(t)
	ctx := context.Background()
	_ = cs.OpenTenant(ctx, "t1")
	for i, owner := range []string{"a", "b", "a"} {
		_, _ = cs.CreateNodeRaw(ctx, "t1", store.NodeInput{
			NodeID:     string(rune('n' + i)),
			TypeID:     1,
			OwnerActor: "u",
			Payload:    map[string]any{"1": owner},
		})
	}
	got, err := cs.QueryNodes(ctx, store.QueryNodesArgs{
		TenantID:        "t1",
		TypeID:          1,
		EqualityFilters: map[uint32]any{1: "a"},
		Limit:           10,
	})
	if err != nil {
		t.Fatalf("QueryNodes: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("QueryNodes len = %d, want 2", len(got))
	}
}

func TestQueryNodesPagination(t *testing.T) {
	cs := newStore(t)
	ctx := context.Background()
	_ = cs.OpenTenant(ctx, "t1")
	now := int64(1000)
	for i := 0; i < 5; i++ {
		_, _ = cs.CreateNodeRaw(ctx, "t1", store.NodeInput{
			NodeID:     string(rune('a' + i)),
			TypeID:     1,
			OwnerActor: "u",
			CreatedAt:  now + int64(i),
		})
	}
	first, err := cs.QueryNodes(ctx, store.QueryNodesArgs{
		TenantID: "t1", TypeID: 1, Limit: 2, Descending: true,
	})
	if err != nil {
		t.Fatalf("QueryNodes p1: %v", err)
	}
	second, err := cs.QueryNodes(ctx, store.QueryNodesArgs{
		TenantID: "t1", TypeID: 1, Limit: 2, Offset: 2, Descending: true,
	})
	if err != nil {
		t.Fatalf("QueryNodes p2: %v", err)
	}
	if len(first) != 2 || len(second) != 2 {
		t.Fatalf("pagination lens = %d, %d", len(first), len(second))
	}
	if first[0].NodeID == second[0].NodeID {
		t.Fatalf("pages overlap: %s == %s", first[0].NodeID, second[0].NodeID)
	}
}

func TestEdgeRoundTrip(t *testing.T) {
	cs := newStore(t)
	ctx := context.Background()
	_ = cs.OpenTenant(ctx, "t1")
	_, err := cs.CreateEdge(ctx, "t1", store.EdgeInput{
		EdgeTypeID: 1, FromNodeID: "a", ToNodeID: "b",
	})
	if err != nil {
		t.Fatalf("CreateEdge: %v", err)
	}

	out, err := cs.GetEdgesFrom(ctx, "t1", "a", nil, 0)
	if err != nil {
		t.Fatalf("GetEdgesFrom: %v", err)
	}
	if len(out) != 1 || out[0].ToNodeID != "b" {
		t.Fatalf("GetEdgesFrom = %+v", out)
	}

	in, err := cs.GetEdgesTo(ctx, "t1", "b", nil, 0)
	if err != nil {
		t.Fatalf("GetEdgesTo: %v", err)
	}
	if len(in) != 1 || in[0].FromNodeID != "a" {
		t.Fatalf("GetEdgesTo = %+v", in)
	}

	if err := cs.DeleteEdge(ctx, "t1", 1, "a", "b"); err != nil {
		t.Fatalf("DeleteEdge: %v", err)
	}
	if err := cs.DeleteEdge(ctx, "t1", 1, "a", "b"); !errors.Is(err, store.ErrEdgeNotFound) {
		t.Fatalf("DeleteEdge missing: got %v, want ErrEdgeNotFound", err)
	}
}

func TestVisibilityIndex(t *testing.T) {
	cs := newStore(t)
	ctx := context.Background()
	_ = cs.OpenTenant(ctx, "t1")
	_, _ = cs.CreateNodeRaw(ctx, "t1", store.NodeInput{
		NodeID: "n1", TypeID: 1, OwnerActor: "user:alice",
		ACL: []store.ACLEntry{
			{Principal: "user:bob", Permission: "read"},
		},
	})
	_, _ = cs.CreateNodeRaw(ctx, "t1", store.NodeInput{
		NodeID: "n2", TypeID: 1, OwnerActor: "user:carol",
	})
	visible, err := cs.GetVisibleNodeIDs(ctx, "t1",
		[]string{"user:bob"}, []string{"n1", "n2"})
	if err != nil {
		t.Fatalf("GetVisibleNodeIDs: %v", err)
	}
	if _, ok := visible["n1"]; !ok {
		t.Fatalf("expected n1 to be visible to bob via ACL")
	}
	if _, ok := visible["n2"]; ok {
		t.Fatalf("did not expect n2 to be visible to bob")
	}
}

func TestIdempotencyDedup(t *testing.T) {
	cs := newStore(t)
	ctx := context.Background()
	_ = cs.OpenTenant(ctx, "t1")

	exists, err := cs.CheckIdempotency(ctx, "t1", "req-1")
	if err != nil || exists {
		t.Fatalf("CheckIdempotency 1: exists=%v err=%v", exists, err)
	}
	if err := cs.RecordIdempotency(ctx, "t1", "req-1", "topic:0:5"); err != nil {
		t.Fatalf("RecordIdempotency: %v", err)
	}
	exists, err = cs.CheckIdempotency(ctx, "t1", "req-1")
	if err != nil || !exists {
		t.Fatalf("CheckIdempotency 2: exists=%v err=%v", exists, err)
	}
	// Second insert -> ErrIdempotencyViolation.
	err = cs.RecordIdempotency(ctx, "t1", "req-1", "topic:0:6")
	if !errors.Is(err, store.ErrIdempotencyViolation) {
		t.Fatalf("RecordIdempotency duplicate: got %v, want ErrIdempotencyViolation", err)
	}
}

func TestWaitForOffsetBlocksUntilUpdate(t *testing.T) {
	cs := newStore(t)
	ctx := context.Background()
	_ = cs.OpenTenant(ctx, "t1")

	var wg sync.WaitGroup
	wg.Add(1)
	var waitErr error
	go func() {
		defer wg.Done()
		ctx2, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		waitErr = cs.WaitForOffset(ctx2, "t1", 5)
	}()

	// Give the waiter a moment to park on the cond.
	time.Sleep(20 * time.Millisecond)
	if err := cs.UpdateAppliedOffset(ctx, "t1", "topic", 0, 5); err != nil {
		t.Fatalf("UpdateAppliedOffset: %v", err)
	}
	wg.Wait()
	if waitErr != nil {
		t.Fatalf("WaitForOffset: %v", waitErr)
	}
}

func TestWaitForOffsetContextCancel(t *testing.T) {
	cs := newStore(t)
	ctx := context.Background()
	_ = cs.OpenTenant(ctx, "t1")

	ctx2, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	err := cs.WaitForOffset(ctx2, "t1", 99)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("WaitForOffset: got %v, want DeadlineExceeded", err)
	}
}

func TestUpdateAppliedOffsetPersists(t *testing.T) {
	cs := newStore(t)
	ctx := context.Background()
	_ = cs.OpenTenant(ctx, "t1")

	if err := cs.UpdateAppliedOffset(ctx, "t1", "topic", 0, 10); err != nil {
		t.Fatalf("Update 1: %v", err)
	}
	got, err := cs.GetAppliedOffset(ctx, "t1", "topic", 0)
	if err != nil || got != 10 {
		t.Fatalf("GetAppliedOffset: got=%d err=%v want=10", got, err)
	}
	// Going backward must be a no-op (the WHERE clause guards it).
	if err := cs.UpdateAppliedOffset(ctx, "t1", "topic", 0, 5); err != nil {
		t.Fatalf("Update backward: %v", err)
	}
	got, _ = cs.GetAppliedOffset(ctx, "t1", "topic", 0)
	if got != 10 {
		t.Fatalf("GetAppliedOffset after backward update: got=%d, want 10", got)
	}
}

func TestEnsureUniqueIndexIdempotent(t *testing.T) {
	cs := newStore(t)
	ctx := context.Background()
	_ = cs.OpenTenant(ctx, "t1")

	for i := 0; i < 3; i++ {
		if err := cs.EnsureUniqueIndex(ctx, "t1", 7, []uint32{1, 2}); err != nil {
			t.Fatalf("EnsureUniqueIndex iter=%d: %v", i, err)
		}
	}
	if err := cs.EnsureQueryIndex(ctx, "t1", 7, []uint32{3}); err != nil {
		t.Fatalf("EnsureQueryIndex: %v", err)
	}
	if err := cs.EnsureCompositeUniqueIndex(ctx, "t1", 7, []store.CompositeUnique{
		{Name: "by_email", FieldIDs: []uint32{1, 2}},
	}); err != nil {
		t.Fatalf("EnsureCompositeUniqueIndex: %v", err)
	}
}

func TestFTSRoundTrip(t *testing.T) {
	cs := newStore(t)
	ctx := context.Background()
	_ = cs.OpenTenant(ctx, "t1")
	const typeID int32 = 9
	fields := []uint32{1}

	if err := cs.EnsureFTSIndex(ctx, "t1", typeID, fields); err != nil {
		t.Fatalf("EnsureFTSIndex: %v", err)
	}
	_, _ = cs.CreateNodeRaw(ctx, "t1", store.NodeInput{
		NodeID: "n1", TypeID: typeID, OwnerActor: "u",
		Payload: map[string]any{"1": "the quick brown fox jumps over the lazy dog"},
	})
	if err := cs.FTSInsert(ctx, "t1", typeID, "n1",
		map[string]any{"1": "the quick brown fox jumps over the lazy dog"},
		fields,
	); err != nil {
		t.Fatalf("FTSInsert: %v", err)
	}
	hits, err := cs.SearchNodes(ctx, "t1", typeID, "fox", fields, 10, 0)
	if err != nil {
		t.Fatalf("SearchNodes: %v", err)
	}
	if len(hits) != 1 || hits[0].NodeID != "n1" {
		t.Fatalf("SearchNodes returned %+v", hits)
	}

	// Stem matching: "running" matches "run" via porter stemmer.
	_, _ = cs.CreateNodeRaw(ctx, "t1", store.NodeInput{
		NodeID: "n2", TypeID: typeID, OwnerActor: "u",
		Payload: map[string]any{"1": "running fast"},
	})
	if err := cs.FTSInsert(ctx, "t1", typeID, "n2",
		map[string]any{"1": "running fast"}, fields); err != nil {
		t.Fatalf("FTSInsert n2: %v", err)
	}
	stemHits, err := cs.SearchNodes(ctx, "t1", typeID, "run", fields, 10, 0)
	if err != nil {
		t.Fatalf("SearchNodes stem: %v", err)
	}
	if len(stemHits) == 0 {
		t.Fatalf("expected stem-match hit for 'run' against 'running'")
	}
}

func TestACLGrantRoundTrip(t *testing.T) {
	cs := newStore(t)
	ctx := context.Background()
	_ = cs.OpenTenant(ctx, "t1")
	_, _ = cs.CreateNodeRaw(ctx, "t1", store.NodeInput{
		NodeID: "n1", TypeID: 1, OwnerActor: "user:alice",
	})
	if err := cs.ShareNode(ctx, "t1", store.ShareNodeInput{
		NodeID: "n1", ActorID: "user:bob", Permission: "read",
		GrantedBy: "user:alice",
	}); err != nil {
		t.Fatalf("ShareNode: %v", err)
	}
	existed, err := cs.RevokeAccess(ctx, "t1", "n1", "user:bob")
	if err != nil || !existed {
		t.Fatalf("RevokeAccess: existed=%v err=%v", existed, err)
	}
	existed, _ = cs.RevokeAccess(ctx, "t1", "n1", "user:bob")
	if existed {
		t.Fatalf("RevokeAccess second call: expected false")
	}
}

func TestTransferOwnership(t *testing.T) {
	cs := newStore(t)
	ctx := context.Background()
	_ = cs.OpenTenant(ctx, "t1")
	_, _ = cs.CreateNodeRaw(ctx, "t1", store.NodeInput{
		NodeID: "n1", TypeID: 1, OwnerActor: "user:alice",
	})
	if err := cs.TransferOwnership(ctx, "t1", "n1", "user:bob"); err != nil {
		t.Fatalf("TransferOwnership: %v", err)
	}
	got, _ := cs.GetNode(ctx, "t1", "n1")
	if got.OwnerActor != "user:bob" {
		t.Fatalf("owner = %q, want user:bob", got.OwnerActor)
	}
	if err := cs.TransferOwnership(ctx, "t1", "missing", "user:bob"); !errors.Is(err, store.ErrNodeNotFound) {
		t.Fatalf("TransferOwnership missing: got %v, want ErrNodeNotFound", err)
	}
}

func TestGroupMembership(t *testing.T) {
	cs := newStore(t)
	ctx := context.Background()
	_ = cs.OpenTenant(ctx, "t1")

	if err := cs.AddGroupMember(ctx, "t1", "group:eng", "user:alice", "member"); err != nil {
		t.Fatalf("AddGroupMember: %v", err)
	}
	existed, err := cs.RemoveGroupMember(ctx, "t1", "group:eng", "user:alice")
	if err != nil || !existed {
		t.Fatalf("RemoveGroupMember: existed=%v err=%v", existed, err)
	}
}

func TestRevokeUserAccess(t *testing.T) {
	cs := newStore(t)
	ctx := context.Background()
	_ = cs.OpenTenant(ctx, "t1")
	_, _ = cs.CreateNodeRaw(ctx, "t1", store.NodeInput{
		NodeID: "n1", TypeID: 1, OwnerActor: "user:alice",
	})
	_ = cs.ShareNode(ctx, "t1", store.ShareNodeInput{
		NodeID: "n1", ActorID: "user:bob", Permission: "read", GrantedBy: "user:alice",
	})
	_ = cs.AddGroupMember(ctx, "t1", "group:eng", "user:bob", "member")
	_ = cs.AddVisibility(ctx, "t1", "n1", "user:bob")

	grants, groups, err := cs.RevokeUserAccess(ctx, "t1", "user:bob")
	if err != nil {
		t.Fatalf("RevokeUserAccess: %v", err)
	}
	if grants < 1 || groups < 1 {
		t.Fatalf("RevokeUserAccess returned grants=%d groups=%d", grants, groups)
	}
}

func TestBatchTxnCommit(t *testing.T) {
	cs := newStore(t)
	ctx := context.Background()
	_ = cs.OpenTenant(ctx, "t1")

	bt, err := cs.BeginBatch(ctx, "t1")
	if err != nil {
		t.Fatalf("BeginBatch: %v", err)
	}
	if err := cs.RecordIdempotencyTx(ctx, bt, "req-A", "topic:0:1"); err != nil {
		_ = bt.Rollback()
		t.Fatalf("RecordIdempotencyTx: %v", err)
	}
	if err := cs.UpdateAppliedOffsetTx(ctx, bt, "topic", 0, 1); err != nil {
		_ = bt.Rollback()
		t.Fatalf("UpdateAppliedOffsetTx: %v", err)
	}
	if err := bt.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	exists, _ := cs.CheckIdempotency(ctx, "t1", "req-A")
	if !exists {
		t.Fatalf("idempotency row missing after commit")
	}
}

func TestBatchTxnRollback(t *testing.T) {
	cs := newStore(t)
	ctx := context.Background()
	_ = cs.OpenTenant(ctx, "t1")

	bt, err := cs.BeginBatch(ctx, "t1")
	if err != nil {
		t.Fatalf("BeginBatch: %v", err)
	}
	if err := cs.RecordIdempotencyTx(ctx, bt, "req-X", "topic:0:9"); err != nil {
		_ = bt.Rollback()
		t.Fatalf("RecordIdempotencyTx: %v", err)
	}
	if err := bt.Rollback(); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	exists, _ := cs.CheckIdempotency(ctx, "t1", "req-X")
	if exists {
		t.Fatalf("idempotency row leaked through rollback")
	}
}

func TestPerTenantIsolation(t *testing.T) {
	// CLAUDE.md invariant #4: per-tenant SQLite isolation. A node
	// inserted into tenant1 must NOT be visible from tenant2.
	cs := newStore(t)
	ctx := context.Background()
	_ = cs.OpenTenant(ctx, "tenant1")
	_ = cs.OpenTenant(ctx, "tenant2")

	_, _ = cs.CreateNodeRaw(ctx, "tenant1", store.NodeInput{
		NodeID: "n1", TypeID: 1, OwnerActor: "u",
	})
	_, err := cs.GetNode(ctx, "tenant2", "n1")
	if !errors.Is(err, store.ErrNodeNotFound) {
		t.Fatalf("cross-tenant read: got %v, want ErrNodeNotFound", err)
	}
}
