// SPDX-License-Identifier: MIT
package entdb

import (
	"context"
	"strconv"
	"sync"
)

// offsetTracker records the last WAL stream position the client has
// written, keyed by tenant id. It is the Go analogue of the Python
// SDK's ``DbClient._last_offsets`` map (see
// ``sdk/python/entdb_sdk/client.py``): every successful Commit /
// ExecuteAtomic stores ``receipt.stream_position`` for the tenant,
// and every subsequent read auto-attaches it as the proto
// ``after_offset`` so the read observes the just-written data
// (read-after-write consistency) with zero application code.
//
// The map is guarded by an RWMutex because a single DbClient is
// safe for concurrent use across goroutines (commits and reads can
// race), unlike the Python SDK whose asyncio client is
// single-threaded.
type offsetTracker struct {
	mu      sync.RWMutex
	offsets map[string]string // tenant_id -> stream_position
}

func newOffsetTracker() *offsetTracker {
	return &offsetTracker{offsets: make(map[string]string)}
}

// record stores the stream position written for a tenant. An empty
// position is ignored — mirrors the Python guard
// ``if result.receipt.stream_position:``. A nil tracker is a no-op
// so transports built without newGRPCTransport (bare struct literals
// in tests) degrade gracefully instead of panicking.
func (o *offsetTracker) record(tenantID, streamPosition string) {
	if o == nil || streamPosition == "" {
		return
	}
	o.mu.Lock()
	if o.offsets == nil {
		o.offsets = make(map[string]string)
	}
	o.offsets[tenantID] = streamPosition
	o.mu.Unlock()
}

// get returns the tracked stream position for a tenant, or ""
// when nothing has been written for it yet. Nil-safe.
func (o *offsetTracker) get(tenantID string) string {
	if o == nil {
		return ""
	}
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.offsets[tenantID]
}

// clear drops every tracked offset. Mirrors the Python
// ``DbClient.clear_offsets()`` — handy in tests and when a caller
// deliberately wants to stop pinning reads to past writes. Nil-safe.
func (o *offsetTracker) clear() {
	if o == nil {
		return
	}
	o.mu.Lock()
	o.offsets = make(map[string]string)
	o.mu.Unlock()
}

// resolve computes the after_offset to send for a read against
// ``tenantID``. It mirrors the Python ``_resolve_offset`` three-way
// semantics, expressed through the request context because Go has
// no keyword-argument sentinel like Python's ``_UNSET``:
//
//   - no offset directive on the context (the common case): use the
//     tracked offset for the tenant (or "" if none) — automatic
//     read-after-write.
//   - WithoutOffsetTracking(ctx): opt out entirely; return "" (the
//     Python ``after_offset=None`` path).
//   - WithAfterOffset(ctx, pos): caller-supplied explicit pin;
//     return ``pos`` verbatim (the Python explicit-string path).
func (o *offsetTracker) resolve(ctx context.Context, tenantID string) string {
	switch d := offsetDirectiveFromContext(ctx); d.kind {
	case offsetOptOut:
		return ""
	case offsetExplicit:
		return d.value
	default: // offsetAuto
		return o.get(tenantID)
	}
}

// ── Per-request override via context ────────────────────────────────
//
// The single-shape Go read surface ([Get] / [GetByKey] / [Query] /
// [GetMany]) takes no optional offset argument, so a caller that
// needs to override automatic tracking for one call threads the
// directive through the request context. This keeps the typed API
// unchanged while still exposing the Python SDK's three behaviours.

type offsetDirectiveKind int

const (
	offsetAuto offsetDirectiveKind = iota // use the tracked offset
	offsetOptOut                          // do not send any after_offset
	offsetExplicit                        // send an exact after_offset
)

type offsetDirective struct {
	kind  offsetDirectiveKind
	value string
}

type offsetDirectiveKeyType struct{}

var offsetDirectiveKey offsetDirectiveKeyType

// WithAfterOffset returns a context that pins the next read to the
// given WAL stream position, overriding automatic offset tracking
// for that call. Equivalent to passing an explicit ``after_offset``
// string in the Python SDK.
//
//	ctx = entdb.WithAfterOffset(ctx, receipt.StreamPosition)
//	p, err := entdb.Get[*shop.Product](ctx, scope, "node-42")
func WithAfterOffset(ctx context.Context, streamPosition string) context.Context {
	return context.WithValue(ctx, offsetDirectiveKey,
		offsetDirective{kind: offsetExplicit, value: streamPosition})
}

// WithoutOffsetTracking returns a context that opts the next read
// out of automatic offset tracking — no ``after_offset`` is sent
// even if the client has written for this tenant. Equivalent to
// passing ``after_offset=None`` in the Python SDK.
func WithoutOffsetTracking(ctx context.Context) context.Context {
	return context.WithValue(ctx, offsetDirectiveKey,
		offsetDirective{kind: offsetOptOut})
}

func offsetDirectiveFromContext(ctx context.Context) offsetDirective {
	if ctx == nil {
		return offsetDirective{kind: offsetAuto}
	}
	if d, ok := ctx.Value(offsetDirectiveKey).(offsetDirective); ok {
		return d
	}
	return offsetDirective{kind: offsetAuto}
}

// waitTimeoutForOffset mirrors the Python SDK's
// ``wait_timeout_ms=30000 if resolved_offset else 0`` — when a
// non-empty offset is attached we let the server block for up to
// 30s for the applier to catch up; otherwise no wait.
func waitTimeoutForOffset(offset string) int32 {
	if offset == "" {
		return 0
	}
	return 30000
}

// offsetAsInt64 converts a resolved string offset to the int64 the
// GetNodeByKey RPC expects. Mirrors the Python ``get_by_key``
// behaviour: a non-parseable / empty value degrades to 0 rather
// than erroring.
func offsetAsInt64(offset string) int64 {
	if offset == "" {
		return 0
	}
	n, err := strconv.ParseInt(offset, 10, 64)
	if err != nil {
		return 0
	}
	return n
}
