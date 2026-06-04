// SPDX-License-Identifier: AGPL-3.0-only

package main

import (
	"fmt"
	"net/http"
)

// applierReadyzHandler builds the /readyz HTTP handler (#653). It returns
// 200 when the applier is making progress (or is idle / caught up) and 503
// when it is stalled (beyond --applier-stall-threshold) or has exited — so
// an HTTP-based orchestrator can drain or restart a wedged pod instead of
// leaving a silent stall serving writes it never applies. The gRPC Health
// RPC carries the same signal in components["applier"]. probe returns
// (stalled, state) so this handler stays decoupled from the apply package.
func applierReadyzHandler(probe func() (stalled bool, state string)) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		stalled, state := probe()
		if stalled {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
		fmt.Fprintf(w, "applier %s\n", state)
	}
}
