package api

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/elloloop/tenant-shard-db/server/go/internal/apply"
	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

func TestAppendGlobalAdminOp_DeterministicConflictReturnsAlreadyExists(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := t.TempDir()

	gs, err := globalstore.New(globalstore.Options{DataDir: dir, WALMode: true})
	if err != nil {
		t.Fatalf("globalstore.New: %v", err)
	}
	t.Cleanup(func() { _ = gs.Close() })
	if _, err := gs.ApplyUserCreated(ctx, globalstore.UserApply{
		UserID:    "alice",
		Email:     "alice@example.com",
		Name:      "Alice",
		Status:    "active",
		CreatedAt: 1700000000,
		UpdatedAt: 1700000000,
	}); err != nil {
		t.Fatalf("seed ApplyUserCreated: %v", err)
	}

	cs, err := store.New(store.Options{RootDir: dir, WALMode: true})
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	w := wal.NewInMemory(1)
	if err := w.Connect(ctx); err != nil {
		t.Fatalf("wal.Connect: %v", err)
	}

	applier, err := apply.New(apply.Options{
		Store:       cs,
		Global:      gs,
		Consumer:    w,
		Topic:       "entdb-wal",
		GroupID:     "admin-wal-regression",
		PollTimeout: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("apply.New: %v", err)
	}
	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- applier.Run(runCtx) }()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	srv := New(WithGlobalStore(gs), WithStore(cs), WithWALProducer(w))
	_, idem, err := srv.appendGlobalAdminOp(ctx, "admin:root", map[string]any{
		"op":         string(apply.OpUserCreated),
		"user_id":    "alice",
		"email":      "alice2@example.com",
		"name":       "Alice 2",
		"status":     "active",
		"created_at": int64(1700000001),
		"updated_at": int64(1700000001),
	})
	if err == nil {
		t.Fatalf("appendGlobalAdminOp: expected ALREADY_EXISTS, got nil")
	}
	if got := status.Code(err); got != codes.AlreadyExists {
		t.Fatalf("appendGlobalAdminOp code = %v, want AlreadyExists (err=%v)", got, err)
	}

	rec, err := cs.CheckIdempotencyStatus(ctx, wal.GlobalTenantID, idem)
	if err != nil {
		t.Fatalf("CheckIdempotencyStatus: %v", err)
	}
	if !rec.Present || rec.Status != store.IdempotencyStatusFailedPrecondition {
		t.Fatalf("idempotency record = %+v, want FAILED_PRECONDITION", rec)
	}
	var failure struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal([]byte(rec.FailureJSON), &failure); err != nil {
		t.Fatalf("failure_json decode: %v", err)
	}
	if failure.Code != "ALREADY_EXISTS" {
		t.Fatalf("failure code = %q, want ALREADY_EXISTS", failure.Code)
	}

	alice, err := gs.GetUser(ctx, "alice")
	if err != nil {
		t.Fatalf("GetUser(alice): %v", err)
	}
	if alice == nil || alice.Email != "alice@example.com" || alice.Name != "Alice" {
		t.Fatalf("conflict mutated existing user: %+v", alice)
	}
}
