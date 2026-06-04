// SPDX-License-Identifier: AGPL-3.0-only

package metrics

import "github.com/prometheus/client_golang/prometheus"

// applierTransientPollErrors counts transient WAL poll errors the
// applier retried (with backoff) instead of exiting the process. A
// rising rate signals broker instability (e.g. idle-connection reaping
// on Azure Event Hubs) without the old crash-loop behaviour. See
// issue #627.
var applierTransientPollErrors = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "entdb_applier_transient_poll_errors_total",
		Help: "Transient WAL poll errors the applier retried instead of exiting (issue #627).",
	},
)

// The apply-progress signals (issue #653) make a STALLED applier visible.
// A stalled applier (blocked mid-apply, e.g. SQLite lock contention on an
// existing shard) keeps ACK'ing writes into the WAL while never
// materialising them — the prod failure behind #650 — and was previously
// indistinguishable from a healthy idle (caught-up) server.
//
// The two timestamps together disambiguate the states:
//
//   - last_progress moves on every committed record. A flat
//     last_progress means no progress.
//   - apply_in_progress_since is the wall-clock the in-flight batch began
//     applying, or 0 when the applier is idle (nothing to apply).
//
// So a STALL is `apply_in_progress_since > 0 AND time() -
// apply_in_progress_since > threshold`, whereas an IDLE (caught-up)
// applier has `apply_in_progress_since == 0` — the distinction a plain
// "no progress" signal cannot make.
var (
	applierLastProgress = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "entdb_applier_last_progress_timestamp_seconds",
			Help: "Unix time of the applier's most recently committed record (issue #653).",
		},
	)
	applierApplyInProgressSince = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "entdb_applier_apply_in_progress_since_timestamp_seconds",
			Help: "Unix time the in-flight apply batch began; 0 when the applier is idle (caught up). A non-zero value older than your stall threshold means the applier is stalled (issue #653).",
		},
	)
	applierRecordsApplied = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "entdb_applier_records_applied_total",
			Help: "Records whose offset the applier committed (applied, skipped, or deterministically-failed); rate is apply throughput (issue #653).",
		},
	)
	applierBatchesApplied = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "entdb_applier_batches_applied_total",
			Help: "Poll batches the applier fully committed (issue #653).",
		},
	)
)

func init() {
	prometheus.MustRegister(
		applierTransientPollErrors,
		applierLastProgress,
		applierApplyInProgressSince,
		applierRecordsApplied,
		applierBatchesApplied,
	)
}

// IncApplierTransientPollError records one retried transient WAL poll
// error.
func IncApplierTransientPollError() {
	applierTransientPollErrors.Inc()
}

// SetApplierLastProgress records the Unix time (seconds) of the applier's
// most recent committed record.
func SetApplierLastProgress(unixSeconds float64) {
	applierLastProgress.Set(unixSeconds)
}

// SetApplierApplyInProgressSince records the Unix time (seconds) the
// in-flight apply batch began, or 0 when the applier is idle.
func SetApplierApplyInProgressSince(unixSeconds float64) {
	applierApplyInProgressSince.Set(unixSeconds)
}

// IncApplierRecordsApplied records one committed record.
func IncApplierRecordsApplied() {
	applierRecordsApplied.Inc()
}

// IncApplierBatchesApplied records one fully-committed poll batch.
func IncApplierBatchesApplied() {
	applierBatchesApplied.Inc()
}

// ApplierTransientPollErrorsCollector exposes the collector so tests can
// register it against a fresh registry without polluting the default
// one.
func ApplierTransientPollErrorsCollector() prometheus.Collector {
	return applierTransientPollErrors
}

// ApplierProgressCollectors exposes the apply-progress collectors (issue
// #653) so tests can read them via testutil.ToFloat64 without depending
// on the default registry.
func ApplierProgressCollectors() []prometheus.Collector {
	return []prometheus.Collector{
		applierLastProgress,
		applierApplyInProgressSince,
		applierRecordsApplied,
		applierBatchesApplied,
	}
}

// ApplierLastProgressCollector exposes the last-progress gauge for tests.
func ApplierLastProgressCollector() prometheus.Collector { return applierLastProgress }

// ApplierApplyInProgressSinceCollector exposes the in-progress-since gauge for tests.
func ApplierApplyInProgressSinceCollector() prometheus.Collector { return applierApplyInProgressSince }

// ApplierRecordsAppliedCollector exposes the records-applied counter for tests.
func ApplierRecordsAppliedCollector() prometheus.Collector { return applierRecordsApplied }

// ApplierBatchesAppliedCollector exposes the batches-applied counter for tests.
func ApplierBatchesAppliedCollector() prometheus.Collector { return applierBatchesApplied }
