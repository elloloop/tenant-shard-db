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
// Wire mechanics by server version:
//
//   - v2.2+ server (single round trip): the SDK sends
//     OnConflict(ConflictSkip) on the CreateNodeOp. The applier
//     swallows the unique violation, looks the colliding row up in the
//     same txn, and returns its id in
//     ExecuteAtomicResponse.existing_node_ids. Works for BOTH
//     single-field and composite unique constraints because the lookup
//     is driven by the violated index (not by an SDK-visible key
//     token).
//
//   - v2.1.x server (two round trips, fallback): SKIP is unknown to
//     the older server, which falls back to its legacy
//     UniqueConstraintError. The SDK catches that, follows up with
//     GetNodeByKey using the typed (TypeID, FieldID, Value) on the
//     UCE, and returns the existing id. Composite violations on a
//     v2.1.x server re-raise the typed error (no GetByCompositeKey
//     RPC, never shipped); v2.2's server-side SKIP closes that gap.
//
// Exactly one of created / existed is non-empty on a non-error return.
// WaitApplied is forced on so the synchronous outcome is observed
// regardless of server version (without it, the loser of a v2.1.x
// race sees a phantom success — issue #606).
//
// Issue #599.
func (s *Scope) InsertIfNotExists(ctx context.Context, idempotencyKey string, msg proto.Message) (created, existed string, err error) {
	if msg == nil {
		return "", "", fmt.Errorf("entdb: InsertIfNotExists: msg is nil")
	}
	plan := s.PlanWithKey(idempotencyKey)
	plan.Create(msg, OnConflict(ConflictSkip))
	res, commitErr := plan.Commit(ctx, WithWaitApplied(true))
	if commitErr == nil {
		// v2.2+ single-RTT path. The applier filled exactly one of
		// the two parallel slots at index 0. ExistingNodeIDs is the
		// authoritative signal because pre-v2.2 servers never set it,
		// so a non-empty entry there proves we're talking to a
		// server that honoured ConflictSkip.
		if len(res.ExistingNodeIDs) > 0 && res.ExistingNodeIDs[0] != "" {
			return "", res.ExistingNodeIDs[0], nil
		}
		if len(res.CreatedNodeIDs) == 0 {
			return "", "", fmt.Errorf("entdb: InsertIfNotExists: commit returned no created_node_ids")
		}
		return res.CreatedNodeIDs[0], "", nil
	}
	// v2.1.x fallback path: SKIP was not honoured; the server
	// surfaced the legacy *UniqueConstraintError. Do the two-RTT
	// GetNodeByKey lookup. Anything other than a UCE propagates.
	var uce *UniqueConstraintError
	if !errors.As(commitErr, &uce) {
		return "", "", commitErr
	}
	if uce.IsComposite() {
		// Composite-unique collision against a v2.1.x server: there
		// is no GetByCompositeKey RPC, so we cannot resolve the
		// existing id. Surface the typed error; the v2.2 server-side
		// path makes this branch unreachable.
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
