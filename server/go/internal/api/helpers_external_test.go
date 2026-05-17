// Shared external-test (package api_test) helpers for the api package.

package api_test

import (
	"context"
	"testing"
	"time"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	"github.com/elloloop/tenant-shard-db/server/go/internal/apply"
	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

func newGlobalStore(t *testing.T) *globalstore.GlobalStore {
	t.Helper()
	gs, err := globalstore.New(globalstore.Options{DataDir: t.TempDir(), WALMode: true})
	if err != nil {
		t.Fatalf("globalstore.New: %v", err)
	}
	t.Cleanup(func() { _ = gs.Close() })
	return gs
}

// newCanonicalStore returns a tmpdir-backed *store.CanonicalStore. Used
// by the cross-tenant ExportUserData tests so each one gets a fresh
// per-tenant SQLite tree on disk.
func newCanonicalStore(t *testing.T) *store.CanonicalStore {
	t.Helper()
	cs, err := store.New(store.Options{RootDir: t.TempDir(), WALMode: true})
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

type adminWALFixture struct {
	srv   *api.Server
	gs    *globalstore.GlobalStore
	cs    *store.CanonicalStore
	wal   *wal.InMemory
	apply *apply.Applier
}

func newAdminWALFixture(t *testing.T, opts ...api.Option) *adminWALFixture {
	t.Helper()
	dir := t.TempDir()
	gs, err := globalstore.New(globalstore.Options{DataDir: dir, WALMode: true})
	if err != nil {
		t.Fatalf("globalstore.New: %v", err)
	}
	t.Cleanup(func() { _ = gs.Close() })

	cs, err := store.New(store.Options{RootDir: dir, WALMode: true})
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	w := wal.NewInMemory(1)
	if err := w.Connect(context.Background()); err != nil {
		t.Fatalf("wal.Connect: %v", err)
	}

	a, err := apply.New(apply.Options{
		Store:       cs,
		Global:      gs,
		Consumer:    w,
		Topic:       "entdb-wal",
		GroupID:     "admin-global-test",
		PollTimeout: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("apply.New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- a.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	serverOpts := []api.Option{
		api.WithGlobalStore(gs),
		api.WithStore(cs),
		api.WithWALProducer(w),
	}
	serverOpts = append(serverOpts, opts...)
	return &adminWALFixture{
		srv:   api.New(serverOpts...),
		gs:    gs,
		cs:    cs,
		wal:   w,
		apply: a,
	}
}

// withTrustedUser returns a ctx that carries the given subject as the
// trusted user identity, mirroring what the auth interceptor sets on
// every authenticated request. Method is arbitrary for handler-level
// tests because auth.Authoritative only consults Subject for the
// trusted path; MethodSession is used here as a neutral default.
func withTrustedUser(ctx context.Context, subject string) context.Context {
	return auth.WithIdentity(ctx, auth.Identity{
		Method:  auth.MethodSession,
		Subject: subject,
	})
}

// nodeIDs flattens a []*pb.Node to its node_id slice, preserving
// order. Shared diagnostic helper used by the get_nodes and
// search_nodes assertion blocks (consolidated in the round-3
// dedupe — the two parallel PRs each declared a private copy).
func nodeIDs(ns []*pb.Node) []string {
	out := make([]string, 0, len(ns))
	for _, n := range ns {
		out = append(out, n.GetNodeId())
	}
	return out
}
