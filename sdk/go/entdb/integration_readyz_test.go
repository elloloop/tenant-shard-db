//go:build integration

// End-to-end coverage for the applier readiness surface (#653) on a REAL
// booted server. The unit tests cover applierReadyzHandler with mock probes
// and Readiness in isolation; this is the only test that proves the full
// wiring on a running binary: the --metrics-addr flag is parsed, the /readyz
// endpoint is registered on the metrics mux, the main.go closure passes the
// live applier's Readiness + threshold, and the apply-progress gauges are
// actually registered and scraped on /metrics.
//
// The 200 (healthy/idle) path is what an end-to-end test can deterministically
// reach — wedging a separately-booted server's applier from outside isn't
// feasible. The 503 (stalled/exited) path is covered by the handler unit test
// (cmd/entdb-server/readyz_test.go) and the in-process stall integration test
// (internal/apply/applier_progress_test.go: TestApplier_StallIsObservable).

package entdb

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func httpGet(t *testing.T, url string) (int, string) {
	t.Helper()
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s body: %v", url, err)
	}
	return resp.StatusCode, string(body)
}

// TestIntegration_ReadyzAndApplierMetrics pins that a healthy booted server
// serves /readyz (200, applier making progress / idle) and exposes the #653
// apply-progress gauges on /metrics.
func TestIntegration_ReadyzAndApplierMetrics(t *testing.T) {
	if itMetricsAddr == "" {
		t.Fatal("itMetricsAddr not set — metrics listener was not booted")
	}
	base := "http://" + itMetricsAddr

	// /readyz: the applier on a healthy idle server is not stalled -> 200,
	// and the body names a non-stalled state.
	code, body := httpGet(t, base+"/readyz")
	if code != http.StatusOK {
		t.Fatalf("/readyz status = %d, want 200 (healthy server); body=%q", code, body)
	}
	state := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(body), "applier"))
	switch state {
	case "idle", "applying":
		// expected on a healthy server
	default:
		t.Fatalf("/readyz body = %q, want applier idle|applying", body)
	}

	// /metrics: the apply-progress signals (#653) must be registered and
	// scraped on the real server, not just in unit tests.
	code, metricsBody := httpGet(t, base+"/metrics")
	if code != http.StatusOK {
		t.Fatalf("/metrics status = %d, want 200", code)
	}
	for _, want := range []string{
		"entdb_applier_last_progress_timestamp_seconds",
		"entdb_applier_apply_in_progress_since_timestamp_seconds",
		"entdb_applier_records_applied_total",
		"entdb_applier_batches_applied_total",
	} {
		if !strings.Contains(metricsBody, want) {
			t.Fatalf("/metrics missing %q (apply-progress gauge not registered on the real server)", want)
		}
	}
}
