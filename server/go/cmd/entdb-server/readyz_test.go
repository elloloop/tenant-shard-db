// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestApplierReadyzHandler pins the /readyz contract (#653): 200 for a
// progressing/idle applier, 503 for a stalled or exited one, with the
// state echoed in the body.
func TestApplierReadyzHandler(t *testing.T) {
	cases := []struct {
		name     string
		stalled  bool
		state    string
		wantCode int
	}{
		{"idle is ready", false, "idle", http.StatusOK},
		{"applying is ready", false, "applying", http.StatusOK},
		{"stalled is not ready", true, "stalled", http.StatusServiceUnavailable},
		{"exited is not ready", true, "exited", http.StatusServiceUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := applierReadyzHandler(func() (bool, string) { return tc.stalled, tc.state })
			rec := httptest.NewRecorder()
			h(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

			if rec.Code != tc.wantCode {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantCode)
			}
			if !strings.Contains(rec.Body.String(), tc.state) {
				t.Fatalf("body %q does not mention state %q", rec.Body.String(), tc.state)
			}
		})
	}
}
