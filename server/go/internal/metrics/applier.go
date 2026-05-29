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

func init() {
	prometheus.MustRegister(applierTransientPollErrors)
}

// IncApplierTransientPollError records one retried transient WAL poll
// error.
func IncApplierTransientPollError() {
	applierTransientPollErrors.Inc()
}

// ApplierTransientPollErrorsCollector exposes the collector so tests can
// register it against a fresh registry without polluting the default
// one.
func ApplierTransientPollErrorsCollector() prometheus.Collector {
	return applierTransientPollErrors
}
