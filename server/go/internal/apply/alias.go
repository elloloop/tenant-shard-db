// SPDX-License-Identifier: AGPL-3.0-only

package apply

import (
	"strings"

	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

// Re-exports of types from sibling packages so callers (and tests) of
// the apply package don't need to import wal / store directly just to
// construct an Event or assert on a Result.
type (
	// Record is an alias for wal.Record — the unit a consumer reads.
	Record = wal.Record
	// StreamPos is an alias for wal.StreamPos — the receipt identity.
	StreamPos = wal.StreamPos
	// BatchTxn is an alias for store.BatchTxn — the per-event txn handle.
	BatchTxn = store.BatchTxn
)

// nodeAliasMap is the per-event alias table backing the "as" / "$alias"
// reference syntax in TransactionEvent.ops. Mirrors
// server/python/entdb_server/apply/applier.py:_node_alias_map (375).
//
// CRITICAL: this map is per-event (a local variable on each
// applyEvent call). Python's instance-attribute version has a latent
// race; the Go port pins it scoped to one apply.
type nodeAliasMap map[string]string

// resolveRef resolves a node-id reference, expanding "$alias" into the
// concrete node_id assigned earlier in the same event. Bare ids pass
// through unchanged. Mirrors applier.py:_resolve_ref (760-770).
func (m nodeAliasMap) resolveRef(ref string) string {
	if strings.HasPrefix(ref, "$") {
		if id, ok := m[ref[1:]]; ok {
			return id
		}
	}
	return ref
}

// resolveNodeRef is a convenience for the create_edge/delete_edge ops
// that always resolve the ref before use. Mirrors applier.py:_resolve_node_ref.
func (m nodeAliasMap) resolveNodeRef(ref string) string {
	return m.resolveRef(ref)
}
