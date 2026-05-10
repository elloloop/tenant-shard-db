// Shared external-test (package api_test) helpers for the api package.

package api_test

import (
	"testing"

	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
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
