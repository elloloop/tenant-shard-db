// SPDX-License-Identifier: AGPL-3.0-only

package api_test

import (
	"context"
	"testing"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

// TestHealth_ApplierComponent pins the readiness gate (#653): with WAL +
// storage healthy, the Health RPC surfaces the wired applier state in
// components["applier"] and gates `healthy` on it — a stalled or exited
// applier flips healthy=false, while applying/idle leaves it healthy. An
// unwired probe is "unknown" and never gates (preserves the legacy
// wal+storage contract).
func TestHealth_ApplierComponent(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		readiness   func() (string, bool) // nil = not wired
		wantApplier string
		wantHealthy bool
	}{
		{"not wired", nil, "unknown", true},
		{"applying", func() (string, bool) { return "applying", true }, "applying", true},
		{"idle", func() (string, bool) { return "idle", true }, "idle", true},
		{"stalled gates unhealthy", func() (string, bool) { return "stalled", false }, "stalled", false},
		{"exited gates unhealthy", func() (string, bool) { return "exited", false }, "exited", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			opts := []api.Option{
				api.WithStore(newRealStore(t)),
				api.WithWALProducer(fakeProducerConnected{}),
			}
			if tc.readiness != nil {
				opts = append(opts, api.WithApplierReadiness(tc.readiness))
			}
			srv := api.New(opts...)

			resp, err := srv.Health(context.Background(), &pb.HealthRequest{})
			if err != nil {
				t.Fatalf("Health: %v", err)
			}
			if got := resp.Components["applier"]; got != tc.wantApplier {
				t.Fatalf("components[applier] = %q, want %q", got, tc.wantApplier)
			}
			if resp.Healthy != tc.wantHealthy {
				t.Fatalf("healthy = %v, want %v (components=%v)", resp.Healthy, tc.wantHealthy, resp.Components)
			}
		})
	}
}

// TestHealth_ApplierProbePanicFailsOpen pins that a panic inside the
// readiness closure becomes "unknown" and does NOT gate health — a buggy
// probe must never take the whole server unhealthy; the apply-progress
// metrics remain the durable signal.
func TestHealth_ApplierProbePanicFailsOpen(t *testing.T) {
	t.Parallel()

	srv := api.New(
		api.WithStore(newRealStore(t)),
		api.WithWALProducer(fakeProducerConnected{}),
		api.WithApplierReadiness(func() (string, bool) { panic("boom") }),
	)

	resp, err := srv.Health(context.Background(), &pb.HealthRequest{})
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if got := resp.Components["applier"]; got != "unknown" {
		t.Fatalf("components[applier] = %q, want unknown after panic", got)
	}
	if !resp.Healthy {
		t.Fatalf("healthy = false; a panicking applier probe must fail open, not gate health")
	}
}
