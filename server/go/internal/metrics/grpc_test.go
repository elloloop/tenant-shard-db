package metrics

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
)

func TestRecordGRPCRequestIncrementsCounterAndHistogram(t *testing.T) {
	// Snapshot before/after so the assertion is robust to other tests in the
	// same package recording against the shared default registry.
	method := "/entdb.v1.EntDBService/Health"
	statusOK := "OK"

	beforeCount := testutil.ToFloat64(grpcRequests.WithLabelValues(method, statusOK))
	beforeHist := histogramSampleCount(t, grpcLatency.WithLabelValues(method))

	RecordGRPCRequest(method, statusOK, 50*time.Millisecond)

	afterCount := testutil.ToFloat64(grpcRequests.WithLabelValues(method, statusOK))
	if afterCount-beforeCount != 1 {
		t.Errorf("counter delta = %v, want 1", afterCount-beforeCount)
	}

	afterHist := histogramSampleCount(t, grpcLatency.WithLabelValues(method))
	if afterHist-beforeHist != 1 {
		t.Errorf("histogram sample-count delta = %d, want 1", afterHist-beforeHist)
	}
}

func TestRecordGRPCRequestSeparatesStatusLabels(t *testing.T) {
	method := "/entdb.v1.EntDBService/SeparateLabelsTest"

	beforeOK := testutil.ToFloat64(grpcRequests.WithLabelValues(method, "OK"))
	beforeErr := testutil.ToFloat64(grpcRequests.WithLabelValues(method, "INTERNAL"))

	RecordGRPCRequest(method, "OK", time.Millisecond)
	RecordGRPCRequest(method, "OK", time.Millisecond)
	RecordGRPCRequest(method, "INTERNAL", time.Millisecond)

	if got := testutil.ToFloat64(grpcRequests.WithLabelValues(method, "OK")) - beforeOK; got != 2 {
		t.Errorf("OK delta = %v, want 2", got)
	}
	if got := testutil.ToFloat64(grpcRequests.WithLabelValues(method, "INTERNAL")) - beforeErr; got != 1 {
		t.Errorf("INTERNAL delta = %v, want 1", got)
	}
}

func TestCollectorsRegisterableInFreshRegistry(t *testing.T) {
	// Sanity-check: Collectors() exposes the same instances; registering them
	// in a fresh registry must succeed (this is what scrape tooling does).
	reg := prometheus.NewRegistry()
	for _, c := range Collectors() {
		if err := reg.Register(c); err != nil {
			t.Fatalf("register collector in fresh registry: %v", err)
		}
	}
}

// histogramSampleCount reads the cumulative sample count out of a histogram
// observer. HistogramVec.WithLabelValues returns an Observer whose concrete
// type also implements prometheus.Metric.
func histogramSampleCount(t *testing.T, obs prometheus.Observer) uint64 {
	t.Helper()
	m, ok := obs.(prometheus.Metric)
	if !ok {
		t.Fatalf("observer %T does not implement prometheus.Metric", obs)
	}
	pb := &dto.Metric{}
	if err := m.Write(pb); err != nil {
		t.Fatalf("write metric: %v", err)
	}
	if pb.Histogram == nil {
		t.Fatalf("expected histogram, got %+v", pb)
	}
	return pb.Histogram.GetSampleCount()
}
