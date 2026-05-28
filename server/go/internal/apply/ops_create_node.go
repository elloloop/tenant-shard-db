// SPDX-License-Identifier: AGPL-3.0-only

package apply

import (
	"context"
	"encoding/json"
	"fmt"
)

// applyCreateNode dispatches a "create_node" op.
//
// Op shape (field-id-keyed payload, per CLAUDE.md invariant #6):
//
//	{
//	  "op": "create_node",
//	  "id": "<node_id>",
//	  "as": "alias", // optional
//	  "type_id": <int>,
//	  "data": {"<fid>": value...}, // field-id keyed
//	  "acl": [{principal, permission}, ...] // optional
//	}
//
// owner_actor comes from the surrounding event.actor. Per the spec, the
// applier never generates node ids — caller-supplied ids are required
// so determinism doesn't depend on the UUID source.
func (a *Applier) applyCreateNode(ctx context.Context, tx *BatchTxn, ev *Event, op map[string]any, aliases nodeAliasMap, res *Result) error {
	nodeID := stringField(op, "id")
	if nodeID == "" {
		return fmt.Errorf("%w: create_node missing id", ErrPoisonEvent)
	}
	typeID := intField(op, "type_id")
	if typeID == 0 {
		return fmt.Errorf("%w: create_node missing type_id", ErrPoisonEvent)
	}
	payload := mapField(op, "data")
	if payload == nil {
		payload = map[string]any{}
	}
	// ACL list. Tolerate either []map[string]any (from JSON unmarshal)
	// or already-typed []store.ACLEntry slices supplied by tests.
	var acl []aclEntry
	if raw, ok := op["acl"].([]any); ok {
		for _, r := range raw {
			if m, ok := r.(map[string]any); ok {
				acl = append(acl, aclEntry{
					Principal:  stringField(m, "principal"),
					Permission: stringField(m, "permission"),
				})
			}
		}
	}

	// Storage classification (#568, ADR-020). The op carries the internal
	// string name ("USER_MAILBOX"/"PUBLIC"); TENANT is the absent default.
	// target_user_id names the owning user for mailbox nodes. Both are
	// immutable after creation and persisted alongside the node row so the
	// mailbox read path can scope by (tenant_id, target_user_id).
	storageModeVal := storageModeCode(stringField(op, "storage_mode"))
	targetUserID := stringField(op, "target_user_id")
	if storageModeVal == storageModeUserMailbox && targetUserID == "" {
		return fmt.Errorf("%w: USER_MAILBOX create_node missing target_user_id", ErrPoisonEvent)
	}
	if storageModeVal != storageModeUserMailbox && targetUserID != "" {
		return fmt.Errorf("%w: target_user_id only valid for USER_MAILBOX", ErrPoisonEvent)
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("apply create_node: marshal payload: %w", err)
	}
	aclJSON, err := json.Marshal(orEmpty(acl))
	if err != nil {
		return fmt.Errorf("apply create_node: marshal acl: %w", err)
	}

	now := ev.TsMs
	if now == 0 {
		now = a.now()
	}

	// Ensure the unique / composite-unique / query expression indexes
	// for this type exist on the txn's own connection BEFORE the INSERT,
	// so a declared constraint fires synchronously inside this batch
	// (ADR-023 + the composite-unique ADR / issue #566).
	if err := a.store.EnsureFieldIndexesTx(ctx, tx, int32(typeID)); err != nil {
		return fmt.Errorf("apply create_node: ensure indexes: %w", err)
	}

	conn := tx.Conn()
	onConflict := stringField(op, "on_conflict") // "" (default ERROR) | "skip" (v2.2 #599)
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO nodes (tenant_id, node_id, type_id, payload_json,
		                   created_at, updated_at, owner_actor, acl_blob,
		                   storage_mode, target_user_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ev.TenantID, nodeID, typeID, string(payloadJSON),
		now, now, ev.Actor, string(aclJSON),
		storageModeVal, targetUserID,
	); err != nil {
		// v2.2 single-RTT InsertIfNotExists (#599): when the op opted
		// into on_conflict=SKIP, look up the colliding row's node_id
		// from the SAME txn (no cross-connection race against the
		// just-inserted row) and swallow the violation. The applier
		// keeps the batch APPLIED; the handler routes the found id
		// into ExecuteAtomicResponse.existing_node_ids at this op's
		// slot. CreatedNodes/ExistingNodes stay index-aligned.
		if onConflict == "skip" {
			existing, found, lookupErr := a.store.LookupNodeIDByUniqueViolation(ctx, conn, ev.TenantID, payload, err)
			if lookupErr != nil {
				return fmt.Errorf("apply create_node: SKIP lookup: %w", lookupErr)
			}
			if found {
				res.CreatedNodes = append(res.CreatedNodes, "")
				res.ExistingNodes = append(res.ExistingNodes, existing)
				if alias := stringField(op, "as"); alias != "" {
					aliases[alias] = existing
				}
				return nil
			}
			// Not a recognised unique-index violation — fall through
			// to the typed UniqueViolation path (SKIP only applies to
			// declared unique constraints; foreign-key / NOT NULL
			// failures still abort the batch).
		}
		// A declared unique/composite constraint trip is an EXPECTED,
		// deterministic outcome — translate it to the typed
		// *UniqueViolation so the per-event loop memoizes + replays it
		// (ALREADY_EXISTS) instead of halting the consumer.
		if detail, ok := a.store.BuildUniqueViolationDetail(ev.TenantID, payload, err); ok {
			return &UniqueViolation{Detail: detail}
		}
		return fmt.Errorf("apply create_node: insert: %w", err)
	}
	// Refresh visibility: owner + each ACL principal.
	if err := refreshVisibility(ctx, conn, ev.TenantID, nodeID, ev.Actor, acl); err != nil {
		return err
	}

	// Maintain the FTS5 search index in the SAME transaction as the node
	// insert so search results never lag (or precede) the canonical row.
	// Mailbox and tenant nodes are indexed identically; the mailbox read
	// path scopes the JOIN by target_user_id at query time.
	if err := a.indexNodeFTS(ctx, conn, ev.TenantID, int32(typeID), nodeID, payload); err != nil {
		return fmt.Errorf("apply create_node: fts index: %w", err)
	}

	if alias := stringField(op, "as"); alias != "" {
		aliases[alias] = nodeID
	}
	res.CreatedNodes = append(res.CreatedNodes, nodeID)
	// ExistingNodes is the index-aligned twin of CreatedNodes (one
	// slot per create_node op). Always extend it in lockstep on the
	// success path so a mixed batch with some SKIPs preserves the
	// per-op alignment the SDK relies on. The handler drops it from
	// the wire response when every slot is empty (preserves the
	// pre-v2.2 ExecuteAtomicResponse shape for batches that never
	// opted into SKIP).
	res.ExistingNodes = append(res.ExistingNodes, "")
	return nil
}

// storageMode internal codes, mirroring entdb.v1.StorageMode and the
// names produced by api.storageModeName.
const (
	storageModeTenant      int32 = 0
	storageModeUserMailbox int32 = 1
	storageModePublic      int32 = 2
)

// storageModeCode maps the internal op string ("USER_MAILBOX"/"PUBLIC"/
// "" -> TENANT) to the integer column value stored on the node row.
func storageModeCode(name string) int32 {
	switch name {
	case "USER_MAILBOX":
		return storageModeUserMailbox
	case "PUBLIC":
		return storageModePublic
	default:
		return storageModeTenant
	}
}

// aclEntry mirrors store.ACLEntry; we don't import store here because
// the per-op handlers run raw SQL — the store types are only the
// surface area the applier exposes back out (see alias.go).
type aclEntry struct {
	Principal  string `json:"principal"`
	Permission string `json:"permission"`
}

func orEmpty(a []aclEntry) []aclEntry {
	if a == nil {
		return []aclEntry{}
	}
	return a
}
