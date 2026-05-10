// Package metrics is the in-process Prometheus chokepoint for the Go server.
//
// Wave 1 scope: the single RecordGRPCRequest entry point used by every gRPC
// handler (mirrors server/python/entdb_server/metrics.py:103). Counter and
// histogram are registered against an in-process registry; the HTTP scrape
// endpoint is deferred to Phase 2.
//
// Metric names and label sets are deliberately identical to the Python
// chokepoint so dashboards and alert rules survive the port:
//
//	entdb_grpc_requests_total{method,status}      Counter
//	entdb_grpc_latency_seconds{method}            Histogram
package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// latencyBuckets matches Python's histogram bucket layout at
// server/python/entdb_server/metrics.py:64.
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
// per-method latency histogram.
//
// Mirrors record_grpc_request in server/python/entdb_server/metrics.py:103.
// The Python version is no-op when metrics are disabled at process start; the
// Go version always records (Prometheus client is cheap and the default
// registry has no scrape endpoint until Phase 2 wires one).
func RecordGRPCRequest(method, status string, duration time.Duration) {
	grpcRequests.WithLabelValues(method, status).Inc()
	grpcLatency.WithLabelValues(method).Observe(duration.Seconds())
}

// Collectors returns the package's collectors. Tests use this to register
// against a fresh prometheus.Registry without polluting the default one.
func Collectors() []prometheus.Collector {
	return []prometheus.Collector{grpcRequests, grpcLatency}
}
