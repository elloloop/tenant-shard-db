// SPDX-License-Identifier: MIT
package entdb

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/protobuf/proto"
)

// InsertIfNotExists is an upsert-style helper for the racy
// query-then-create idiom: under N concurrent writers of the same
// uniquely-keyed payload, exactly one wins and the rest learn the
// existing canonical node id without a second round trip into client
// code.
//
// Mechanics: a single CreateNodeOp is committed with WaitApplied so
// the server-side outcome is surfaced synchronously. On success the
// new id is returned (created != ""). When the applier trips a
// single-field unique constraint the typed *UniqueConstraintError
// surfaces the (type_id, field_id, value) tuple; the helper follows
// up with GetNodeByKey on the same connection and returns the
// existing id (existed != ""). Exactly one of created / existed is
// non-empty on a non-error return; the caller branches on whichever
// side it cares about.
//
// Issue #599. Notes for v2.1.0:
//
//   - SINGLE-FIELD unique only. A composite-unique violation (the
//     v2.x server enforces these atomically; see ADR-030) returns the
//     raw *UniqueConstraintError without a follow-up lookup —
//     there is no GetByCompositeKey RPC, so callers needing
//     composite upsert must query themselves. Tracked for v2.2.
//
//   - The follow-up GetNodeByKey is a SECOND round trip. The truly
//     atomic insert-if-not-exists (one round trip via a server-side
//     NodeConflictPolicy_SKIP) is the v2.2 path; this helper closes
//     the correctness gap (the racy query-then-create) NOW with the
//     primitives already shipped in v2.0.x.
//
//   - WaitApplied is forced on so the synchronous error path
//     observes the applier's outcome (without it, the loser sees a
//     phantom success — issue #606).
func (s *Scope) InsertIfNotExists(ctx context.Context, idempotencyKey string, msg proto.Message) (created, existed string, err error) {
	if msg == nil {
		return "", "", fmt.Errorf("entdb: InsertIfNotExists: msg is nil")
	}
	plan := s.PlanWithKey(idempotencyKey)
	plan.Create(msg)
	res, commitErr := plan.Commit(ctx, WithWaitApplied(true))
	if commitErr == nil {
		if len(res.CreatedNodeIDs) == 0 {
			return "", "", fmt.Errorf("entdb: InsertIfNotExists: commit returned no created_node_ids")
		}
		return res.CreatedNodeIDs[0], "", nil
	}
	// On a typed UniqueConstraintError we know exactly which key
	// collided; one follow-up GetNodeByKey returns the canonical
	// existing id. Anything else propagates.
	var uce *UniqueConstraintError
	if !errors.As(commitErr, &uce) {
		return "", "", commitErr
	}
	if uce.IsComposite() {
		// No GetByCompositeKey RPC in v2.x. Surface the typed error
		// so the caller can do their own composite lookup. Tracked
		// for v2.2 (issue #599 server-side path).
		return "", "", commitErr
	}
	node, lookupErr := s.client.transport.GetNodeByKey(
		ctx, s.tenantID, s.actor.String(),
		uce.TypeID, uce.FieldID, uce.Value,
	)
	if lookupErr != nil {
		return "", "", fmt.Errorf("entdb: InsertIfNotExists: post-conflict GetNodeByKey: %w", lookupErr)
	}
	if node == nil {
		// Should not happen — the applier just told us this key
		// collided with a committed row. Surface the original error
		// so the caller can act on the unique-constraint detail.
		return "", "", commitErr
	}
	return "", node.NodeID, nil
}
