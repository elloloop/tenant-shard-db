// SPDX-License-Identifier: AGPL-3.0-only

package wal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/elloloop/tenant-shard-db/server/go/internal/jsonnum"
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

// Event is the wire-level payload appended to the WAL for every
// mutation. Required fields: TenantID, Actor, IdempotencyKey, Ops.
// SchemaFingerprint may be empty when the schema registry has not
// stamped a fingerprint yet. TsMs defaults to the current wall clock
// when zero (see NewEvent / Encode).
//
// Ops is intentionally typed as []map[string]any to preserve the
// encoded byte layout required by cross-impl contract tests (see
// docs/go-port/shared/wal-producer.md "Op typing").
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
// milliseconds.
func (e Event) Encode() ([]byte, error) {
	if e.TsMs == 0 {
		e.TsMs = time.Now().UnixMilli()
	}
	return json.Marshal(e)
}

// DecodeEvent parses a JSON-encoded Event from a record value.
// tenant_id, actor, idempotency_key, and ops are required; missing
// fields surface as a non-nil error so the applier can halt rather
// than silently proceeding.
func DecodeEvent(value []byte) (Event, error) {
	var e Event
	// UseNumber + normalize so integer payload values inside Ops survive
	// as int64 rather than collapsing to float64 (ADR-028 / Bug C #563).
	// Numeric op fields (type_id, edge_id, ...) read through accessors
	// that handle int64; CAS/DeleteWhere compare against the same
	// canonical representation produced by jsonnum at every boundary.
	dec := json.NewDecoder(bytes.NewReader(value))
	dec.UseNumber()
	if err := dec.Decode(&e); err != nil {
		return Event{}, fmt.Errorf("wal: decode event: %w", err)
	}
	for i := range e.Ops {
		if e.Ops[i] != nil {
			e.Ops[i] = jsonnum.Normalize(e.Ops[i]).(map[string]any)
		}
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

// StreamPos is a position in the WAL stream. The String() form is
// "topic:partition:offset"; TimestampMs is record metadata, not part
// of the position identity.
type StreamPos struct {
	Topic       string
	Partition   int32
	Offset      int64
	TimestampMs int64
}

// String returns the canonical "topic:partition:offset" form. Used as
// the stream-position receipt the SDK returns to clients.
func (p StreamPos) String() string {
	return fmt.Sprintf("%s:%d:%d", p.Topic, p.Partition, p.Offset)
}

// Record is a record consumed from the WAL stream. Headers is
// map[string][]byte (matches the Go interface spec in
// docs/go-port/shared/wal-producer.md).
type Record struct {
	Key      string
	Value    []byte
	Position StreamPos
	Headers  map[string][]byte
}
