// Shared external-test (package api_test) helpers for the api package.

package api_test

import (
	"context"
	"testing"

	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
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
// search_nodes assertion blocks (consolidated in the round-3 Wave-2
// dedupe — the two parallel PRs each declared a private copy).
func nodeIDs(ns []*pb.Node) []string {
	out := make([]string, 0, len(ns))
	for _, n := range ns {
		out = append(out, n.GetNodeId())
	}
	return out
}
