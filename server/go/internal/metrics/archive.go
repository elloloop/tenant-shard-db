// SPDX-License-Identifier: AGPL-3.0-only

// Archive metrics expose the health of the S3 Object Lock WAL archive
// sidecar (ADR-015). The lag gauge is the primary alerting signal:
// operators alarm on it climbing because a stalled archiver means
// events live only in Kafka/Redpanda, outside tamper-evident storage.
//
// Metric names and label sets are deliberately stable so dashboards
// and alert rules survive across releases:
//
//	entdb_archive_lag_events{topic,partition}     Gauge
//	entdb_archive_writes_total                    Counter
//	entdb_archive_errors_total                    Counter

package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	archiveLag = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "entdb_archive_lag_events",
			Help: "WAL records polled by the archive sidecar but not yet written to S3 Object Lock storage, per partition",
		},
		[]string{"topic", "partition"},
	)

	archiveWrites = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "entdb_archive_writes_total",
			Help: "Total WAL records written to S3 Object Lock archive objects and committed",
		},
	)

	archiveErrors = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "entdb_archive_errors_total",
			Help: "Total archive poll/build/put/commit failures observed by the archive sidecar",
		},
	)
)

func init() {
	prometheus.MustRegister(archiveLag, archiveWrites, archiveErrors)
}

// SetArchiveLag publishes the current archive lag (records polled but
// not yet durably archived) for a (topic, partition). Called by the
// archive sidecar after each poll/commit cycle.
func SetArchiveLag(topic, partition string, lag float64) {
	archiveLag.WithLabelValues(topic, partition).Set(lag)
}

// AddArchiveWrites records that n WAL records were archived and committed.
func AddArchiveWrites(n int) {
	if n > 0 {
		archiveWrites.Add(float64(n))
	}
}

// IncArchiveErrors records one archive poll/build/put/commit failure.
func IncArchiveErrors() {
	archiveErrors.Inc()
}

// ArchiveCollectors returns the archive collectors so tests can register
// them against a fresh prometheus.Registry without touching the default.
func ArchiveCollectors() []prometheus.Collector {
	return []prometheus.Collector{archiveLag, archiveWrites, archiveErrors}
}
