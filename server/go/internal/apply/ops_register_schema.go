// SPDX-License-Identifier: AGPL-3.0-only

package apply

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/elloloop/tenant-shard-db/server/go/internal/schema"
)

// applyRegisterSchema materializes a "register_schema" op (SELF-DESCRIBING
// WRITES). It runs BEFORE the data ops in the SAME transaction so that:
//
//   - the types a write touches are registered into the process-wide
//     schema.Registry (establish-or-reject), and
//   - the per-tenant unique / composite-unique / query expression indexes
//     for those node types exist on THIS txn's connection before the
//     data ops INSERT, so a declared constraint fires synchronously
//     inside the same batch (mirroring applyCreateNode's own
//     EnsureFieldIndexesTx call).
//
// Op shape (mirrors the cross-language schema JSON contract — the same
// shape schema.LoadFromJSON consumes). NAME-FREE per ADR-031: types are
// keyed by type_id / edge_id, fields by field_id, composites by their
// field_ids tuple. No names appear anywhere:
//
//	{
//	  "op": "register_schema",
//	  "node_types": [ {type_id, fields:[...], composite_unique:[...], ...} ],
//	  "edge_types": [ {edge_id, from_type_id, to_type_id, props:[...], ...} ]
//	}
//
// Conflict handling (deterministic outcomes, NOT poison):
//
//   - A type absent from the registry is registered.
//   - A type present and byte-identical is a no-op (idempotent — this is
//     why replaying the WAL on boot rebuilds the registry cleanly even
//     though the same schema op recurs across many events).
//   - A type present but DIFFERENT is rejected with *SchemaConflict,
//     which applyEvent memoizes as FAILED_PRECONDITION and advances the
//     offset without halting (online evolution / ALTER is out of scope).
//
// A nil registry (the schema-less server profile) makes this op a
// structural error: a self-describing write only reaches the applier
// when the server advertises a fingerprint, which requires a registry.
func (a *Applier) applyRegisterSchema(ctx context.Context, tx *BatchTxn, ev *Event, op map[string]any) error {
	reg := a.store.Registry()
	if reg == nil {
		return fmt.Errorf("%w: register_schema without a schema registry", ErrPoisonEvent)
	}

	nodeDefs, edgeDefs, err := decodeSchemaOp(op)
	if err != nil {
		return fmt.Errorf("%w: register_schema: %v", ErrPoisonEvent, err)
	}

	// Register node types first so edge from/to cross-references and the
	// per-tenant index creation below see them.
	for i := range nodeDefs {
		nt := &nodeDefs[i]
		if _, err := reg.RegisterOrVerifyNode(nt); err != nil {
			if errors.Is(err, schema.ErrSchemaConflict) {
				return &SchemaConflict{Detail: err.Error()}
			}
			// A malformed type that slips past the handler's own
			// validation is a poison event (deterministic, but not a
			// recoverable client outcome).
			return fmt.Errorf("%w: register_schema node type_id %d: %v", ErrPoisonEvent, nt.TypeID, err)
		}
		// Ensure this type's unique / composite-unique / query indexes
		// exist on the txn connection BEFORE the data ops run. Idempotent
		// (CREATE ... IF NOT EXISTS), so re-running for an already-known
		// type is cheap.
		if err := a.store.EnsureFieldIndexesTx(ctx, tx, nt.TypeID); err != nil {
			return fmt.Errorf("apply register_schema: ensure indexes t%d: %w", nt.TypeID, err)
		}
	}
	for i := range edgeDefs {
		et := &edgeDefs[i]
		if _, err := reg.RegisterOrVerifyEdge(et); err != nil {
			if errors.Is(err, schema.ErrSchemaConflict) {
				return &SchemaConflict{Detail: err.Error()}
			}
			return fmt.Errorf("%w: register_schema edge edge_id %d: %v", ErrPoisonEvent, et.EdgeID, err)
		}
	}
	return nil
}

// decodeSchemaOp lifts the node_types / edge_types arrays out of a
// register_schema op (which the WAL decoded into map[string]any with
// jsonnum-normalised numbers) and re-marshals them into the typed
// schema.NodeTypeDef / schema.EdgeTypeDef structs the registry expects.
//
// Round-tripping through JSON is intentional: the schema JSON contract is
// the single source of truth for the field names/shape (snake_case tags
// on the schema structs), so re-using encoding/json keeps this decode in
// lockstep with schema.LoadFromJSON and the handler's encode side.
func decodeSchemaOp(op map[string]any) ([]schema.NodeTypeDef, []schema.EdgeTypeDef, error) {
	var nodes []schema.NodeTypeDef
	if raw, ok := op["node_types"]; ok && raw != nil {
		b, err := json.Marshal(raw)
		if err != nil {
			return nil, nil, fmt.Errorf("marshal node_types: %w", err)
		}
		if err := json.Unmarshal(b, &nodes); err != nil {
			return nil, nil, fmt.Errorf("decode node_types: %w", err)
		}
	}
	var edges []schema.EdgeTypeDef
	if raw, ok := op["edge_types"]; ok && raw != nil {
		b, err := json.Marshal(raw)
		if err != nil {
			return nil, nil, fmt.Errorf("marshal edge_types: %w", err)
		}
		if err := json.Unmarshal(b, &edges); err != nil {
			return nil, nil, fmt.Errorf("decode edge_types: %w", err)
		}
	}
	if len(nodes) == 0 && len(edges) == 0 {
		return nil, nil, errors.New("no node_types or edge_types")
	}
	return nodes, edges, nil
}
