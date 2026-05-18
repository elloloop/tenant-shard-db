// SPDX-License-Identifier: AGPL-3.0-only

// Legal-hold-lift metrics expose the health of the durable S3 Object
// Lock legal-hold lift queue + worker (EPIC #511 Gap 1, ADR-015).
//
// Before this, an explicit SetLegalHold OFF launched a detached
// fire-and-forget goroutine: a crash / S3 outage / timeout left a
// released tenant's objects stuck LegalHold=ON with NO signal, which
// silently broke GDPR right-to-erasure-after-release. The pending gauge
// is now the operator alerting signal: it climbs when a release was
// recorded but the sweep has not yet completed, and drains to 0 once the
// worker has lifted every object.
//
// Metric names and label sets are deliberately stable so dashboards and
// alert rules survive across releases:
//
//	entdb_legal_hold_lift_pending             Gauge
//	entdb_legal_hold_lift_completed_total     Counter
//	entdb_legal_hold_lift_errors_total        Counter

package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	legalHoldLiftPending = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "entdb_legal_hold_lift_pending",
			Help: "Tenants whose legal hold was released but whose archived S3 objects' Object Lock legal hold has not yet been fully lifted (durable legal_hold_lift_queue depth)",
		},
	)

	legalHoldLiftCompleted = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "entdb_legal_hold_lift_completed_total",
			Help: "Total tenants whose archive legal-hold lift sweep fully completed and was removed from the durable queue",
		},
	)

	legalHoldLiftErrors = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "entdb_legal_hold_lift_errors_total",
			Help: "Total legal-hold lift sweep failures (queue/list/get/put). The queue row is retained and retried on the next worker tick",
		},
	)
)

func init() {
	prometheus.MustRegister(legalHoldLiftPending, legalHoldLiftCompleted, legalHoldLiftErrors)
}

// SetLegalHoldLiftPending publishes the current durable lift-queue depth
// (tenants released but not yet fully swept). Called by the lift worker
// at the start and end of every tick.
func SetLegalHoldLiftPending(n int) {
	legalHoldLiftPending.Set(float64(n))
}

// IncLegalHoldLiftCompleted records that one tenant's lift sweep fully
// completed and its durable queue row was removed.
func IncLegalHoldLiftCompleted() {
	legalHoldLiftCompleted.Inc()
}

// IncLegalHoldLiftErrors records one lift sweep failure. The queue row is
// left in place so the next worker tick retries (resumable sweep).
func IncLegalHoldLiftErrors() {
	legalHoldLiftErrors.Inc()
}

// LegalHoldLiftCollectors returns the lift collectors so tests can
// register them against a fresh prometheus.Registry without touching the
// default one.
func LegalHoldLiftCollectors() []prometheus.Collector {
	return []prometheus.Collector{legalHoldLiftPending, legalHoldLiftCompleted, legalHoldLiftErrors}
}
