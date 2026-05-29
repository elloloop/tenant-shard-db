// SPDX-License-Identifier: AGPL-3.0-only

package wal

// W3C Trace Context header keys carried on WAL records so a request's
// trace survives the async hop from the ExecuteAtomic handler (which
// appends) to the applier (which writes SQLite later, on a different
// goroutine + context). See ADR-033 §5.
//
// They ride alongside HeaderIdempotencyKey in Record.Headers and never
// touch the Event JSON body, so the cross-impl contract byte layout
// (ADR-006/ADR-018) and replay determinism are unaffected.
const (
	HeaderTraceparent = "traceparent"
	HeaderTracestate  = "tracestate"
)

// HeaderCarrier adapts a WAL Record.Headers map to the
// propagation.TextMapCarrier interface (Get/Set/Keys) so the global
// OpenTelemetry propagator can Inject into / Extract from WAL headers.
// It is intentionally defined here (not in a tracing package) so the
// wal package stays free of any OpenTelemetry import — the interface is
// satisfied structurally.
type HeaderCarrier map[string][]byte

// Get returns the value for key, or "" if absent.
func (c HeaderCarrier) Get(key string) string {
	if v, ok := c[key]; ok {
		return string(v)
	}
	return ""
}

// Set stores value under key.
func (c HeaderCarrier) Set(key, value string) {
	c[key] = []byte(value)
}

// Keys returns the header names currently present.
func (c HeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}
