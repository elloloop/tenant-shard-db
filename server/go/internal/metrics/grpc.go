// Package metrics is the in-process Prometheus chokepoint for the Go server.
//
// The single RecordGRPCRequest entry point is called by every gRPC
// handler. Counter and histogram are registered against an in-process
// registry; an HTTP scrape endpoint is configured separately.
//
// Metric names and label sets are deliberately stable so dashboards
// and alert rules survive across releases:
//
//	entdb_grpc_requests_total{method,status}      Counter
//	entdb_grpc_latency_seconds{method}            Histogram
package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// latencyBuckets is the histogram bucket layout used by gRPC latency
// metrics. Tuned for sub-second p99 + occasional multi-second tail.
var latencyBuckets = []float64{
	0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 5.0,
}

var (
	grpcRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "entdb_grpc_requests_total",
			Help: "Total gRPC requests",
		},
		[]string{"method", "status"},
	)

	grpcLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "entdb_grpc_latency_seconds",
			Help:    "gRPC request latency",
			Buckets: latencyBuckets,
		},
		[]string{"method"},
	)
)

func init() {
	prometheus.MustRegister(grpcRequests, grpcLatency)
}

// RecordGRPCRequest is the single chokepoint every gRPC handler calls on its
// way out. It increments the per-(method,status) counter and observes the
// per-method latency histogram. The Prometheus client is cheap so we always
// record; an HTTP scrape endpoint is wired separately.
func RecordGRPCRequest(method, status string, duration time.Duration) {
	grpcRequests.WithLabelValues(method, status).Inc()
	grpcLatency.WithLabelValues(method).Observe(duration.Seconds())
}

// Collectors returns the package's collectors. Tests use this to register
// against a fresh prometheus.Registry without polluting the default one.
func Collectors() []prometheus.Collector {
	return []prometheus.Collector{grpcRequests, grpcLatency}
}
