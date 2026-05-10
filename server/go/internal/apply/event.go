// SPDX-License-Identifier: AGPL-3.0-only

// Package apply hosts the WAL consumer (Applier) that materializes
// TransactionEvents into per-tenant SQLite via the store package.
//
// Spec: docs/go-port/shared/applier.md.
//
// Wave-0 architectural-drift fixes implemented here (per
// docs/go-port/PLAN.md §6):
//
//   - DelegateAccess applier dispatch (was silently dropped in Python).
//   - WAL-first restoration for share_node, revoke_access, delegate_access,
//     transfer_ownership, add_group_member, remove_group_member,
//     set_legal_hold.
//   - admin_revoke_access broadened to also delete node_access and
//     group_users.
//   - add_tenant_member / remove_tenant_member / change_member_role added
//     so global-store membership writes can be replayed.
//
// Concurrency model: a single consumer goroutine drives Run; per-tenant
// SQLite isolation comes from the store package's per-tenant write
// mutex. Halt-on-poison: any error in op-dispatch returns from Run with
// a non-nil error and does not advance the WAL offset.
package apply

import (
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

// Event is the in-Go transaction event the applier consumes from the
// WAL. It is a re-export of wal.Event so callers (and tests) don't need
// to depend on both packages just to construct one.
type Event = wal.Event

// OpType enumerates the op-type strings carried inside Event.Ops[i]["op"].
// Mirrors the Python applier's `op_type` if/elif ladder
// (server/python/entdb_server/apply/applier.py:929-1248) plus the new
// op types added by the Go port to close the WAL-first drift gap
// (docs/go-port/PLAN.md §6).
type OpType string

const (
	// Steady-state ops the Python applier already implements.
	OpCreateNode OpType = "create_node"
	OpUpdateNode OpType = "update_node"
	OpDeleteNode OpType = "delete_node"
	OpCreateEdge OpType = "create_edge"
	OpDeleteEdge OpType = "delete_edge"

	// admin ops the Python applier already implements (broadened in Go).
	OpAdminTransferContent OpType = "admin_transfer_content"
	OpAdminRevokeAccess    OpType = "admin_revoke_access"

	// WAL-first restorations from PLAN.md §6.1. The Python handlers
	// write SQLite directly today; the Go port appends a WAL event and
	// the dispatch branches below apply it on replay.
	OpShareNode         OpType = "share_node"
	OpRevokeAccess      OpType = "revoke_access"
	OpDelegateAccess    OpType = "delegate_access"
	OpTransferOwnership OpType = "transfer_ownership"
	OpAddGroupMember    OpType = "add_group_member"
	OpRemoveGroupMember OpType = "remove_group_member"
	OpSetLegalHold      OpType = "set_legal_hold"

	// Global-store membership ops. globalstore is its own durable
	// substrate (carve-out from CLAUDE.md invariant #1), but recording
	// these in the WAL keeps audit history reconstructible.
	OpAddTenantMember    OpType = "add_tenant_member"
	OpRemoveTenantMember OpType = "remove_tenant_member"
	OpChangeMemberRole   OpType = "change_member_role"
)

// opTypeOf extracts the "op" field as a typed OpType, returning an
// empty string when missing or non-string.
func opTypeOf(op map[string]any) OpType {
	v, ok := op["op"].(string)
	if !ok {
		return ""
	}
	return OpType(v)
}

// stringField extracts a string-valued op field, returning "" when
// missing or wrongly-typed. Used by the per-op helpers to keep the
// dispatch table free of type-switch boilerplate.
func stringField(op map[string]any, key string) string {
	v, ok := op[key].(string)
	if !ok {
		return ""
	}
	return v
}

// intField extracts an int32-valued op field. JSON numbers come through
// as float64; protobuf JSON-encoded ints arrive that way too. Returns 0
// when missing or wrongly-typed.
func intField(op map[string]any, key string) int32 {
	switch v := op[key].(type) {
	case float64:
		return int32(v)
	case int:
		return int32(v)
	case int32:
		return v
	case int64:
		return int32(v)
	}
	return 0
}

// mapField extracts a nested map-valued op field. Returns nil when
// missing or wrongly-typed.
func mapField(op map[string]any, key string) map[string]any {
	v, ok := op[key].(map[string]any)
	if !ok {
		return nil
	}
	return v
}

// boolField extracts a boolean-valued op field. Returns false when
// missing or wrongly-typed.
func boolField(op map[string]any, key string) bool {
	v, ok := op[key].(bool)
	if !ok {
		return false
	}
	return v
}
