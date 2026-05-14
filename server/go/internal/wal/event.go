// SPDX-License-Identifier: AGPL-3.0-only

package wal

import (
	"encoding/json"
	"fmt"
	"time"
)

// Event scope values. Tenant-scoped events materialize into the
// per-tenant canonical store keyed by Event.TenantID. Global-scoped
// events materialize into globalstore and use GlobalTenantID as the WAL
// partition key / idempotency tenant.
type Scope string

const (
	ScopeTenant Scope = "tenant"
	ScopeGlobal Scope = "global"

	// GlobalTenantID is the synthetic tenant key used for control-plane
	// WAL events. It is intentionally filesystem-safe so the applier can
	// reuse the canonical store's applied_events / applied_offsets
	// machinery for global events.
	GlobalTenantID = "__global__"
)

// Event mirrors server/python/entdb_server/apply/applier.py:204
// (TransactionEvent). It is the wire-level payload appended to the WAL
// for every mutation. Required fields: TenantID, Actor, IdempotencyKey,
// Ops. SchemaFingerprint may be empty when the schema registry has not
// stamped a fingerprint yet. TsMs defaults to the current wall clock
// when zero (see NewEvent / Encode).
//
// Ops is intentionally typed as []map[string]any: until the op proto
// lands (see docs/go-port/shared/wal-producer.md "Op typing"), the
// Python side stores ops as list[dict[str, Any]] and the Go port must
// preserve the same encoded byte layout for cross-impl contract tests.
type Event struct {
	TenantID          string           `json:"tenant_id"`
	Scope             Scope            `json:"scope,omitempty"`
	Actor             string           `json:"actor"`
	IdempotencyKey    string           `json:"idempotency_key"`
	SchemaFingerprint string           `json:"schema_fingerprint,omitempty"`
	TsMs              int64            `json:"ts_ms"`
	Ops               []map[string]any `json:"ops"`
}

// Encode serializes e to JSON. Field ordering is fixed by the struct
// tags above; encoding/json emits keys in struct-field order, which
// keeps the byte layout deterministic for cross-impl contract tests.
//
// If TsMs is zero, it is replaced with the current wall-clock time in
// milliseconds (mirrors applier.py:266 default).
func (e Event) Encode() ([]byte, error) {
	if e.TsMs == 0 {
		e.TsMs = time.Now().UnixMilli()
	}
	return json.Marshal(e)
}

// DecodeEvent parses a JSON-encoded Event from a record value. Mirrors
// applier.py:241 (TransactionEvent.from_dict): tenant_id, actor,
// idempotency_key, ops are required; missing fields surface as a
// non-nil error so the applier can halt rather than silently
// proceeding.
func DecodeEvent(value []byte) (Event, error) {
	var e Event
	if err := json.Unmarshal(value, &e); err != nil {
		return Event{}, fmt.Errorf("wal: decode event: %w", err)
	}
	missing := make([]string, 0, 4)
	if e.TenantID == "" {
		missing = append(missing, "tenant_id")
	}
	if e.Actor == "" {
		missing = append(missing, "actor")
	}
	if e.IdempotencyKey == "" {
		missing = append(missing, "idempotency_key")
	}
	if e.Ops == nil {
		missing = append(missing, "ops")
	}
	if len(missing) > 0 {
		return Event{}, fmt.Errorf("wal: missing required fields: %v", missing)
	}
	if e.Scope == "" {
		e.Scope = ScopeTenant
	}
	switch e.Scope {
	case ScopeTenant, ScopeGlobal:
	default:
		return Event{}, fmt.Errorf("wal: unknown scope %q", e.Scope)
	}
	return e, nil
}

// StreamPos is a position in the WAL stream. Mirrors
// server/python/entdb_server/wal/base.py:65 (StreamPos). The String()
// form is "topic:partition:offset"; TimestampMs is record metadata,
// not part of the position identity.
type StreamPos struct {
	Topic       string
	Partition   int32
	Offset      int64
	TimestampMs int64
}

// String returns the canonical "topic:partition:offset" form. Used as
// the stream-position receipt the SDK returns to clients (see Python
// grpc_server.py:799).
func (p StreamPos) String() string {
	return fmt.Sprintf("%s:%d:%d", p.Topic, p.Partition, p.Offset)
}

// Record is a record consumed from the WAL stream. Mirrors
// server/python/entdb_server/wal/base.py:111 (StreamRecord). Headers
// is map[string][]byte to match the Python dict[str, bytes] type and
// the Go interface spec in docs/go-port/shared/wal-producer.md.
type Record struct {
	Key      string
	Value    []byte
	Position StreamPos
	Headers  map[string][]byte
}
