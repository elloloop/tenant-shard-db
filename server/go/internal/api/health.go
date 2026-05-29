// SPDX-License-Identifier: AGPL-3.0-only

package api

import (
	"context"
	"time"

	"github.com/elloloop/tenant-shard-db/server/go/internal/metrics"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/version"
)

// walPinger is the optional probe contract a WAL Producer may implement.
// Handlers must NOT round-trip to the broker inside Health, so
// implementations are expected to return a cached connection flag rather
// than performing a live ping.
//
// Producers that do not implement walPinger are treated as healthy.
type walPinger interface {
	IsConnected() bool
}

// storagePinger is the optional probe contract a CanonicalStore may
// implement. Today no Go store has a Health hook (see
// docs/go-port/rpcs/Health.md "storage='healthy' is a lie"), so this
// lives here only to keep the door open for a future PRAGMA quick_check
// probe behind a flag.
type storagePinger interface {
	Health(ctx context.Context) error
}

// Health implements the entdb.v1.EntDBService/Health RPC. Spec:
// docs/go-port/rpcs/Health.md.
//
// Contract pins:
//   - No auth, no rate-limit, no actor — allow-listed by the auth and
//     rate-limit interceptors. The handler MUST NOT consult ACLs.
//   - No WAL append, no store write, no global_store read. Strictly
//     read-only, in-memory introspection.
//   - `healthy` is true iff components["wal"] == "healthy" AND
//     components["storage"] == "healthy". The multi-node info keys
//     (node_id, assigned_tenants) are informational and MUST NOT gate
//     healthy.
//   - Always returns OK; an internal panic is the only path to INTERNAL.
func (s *Server) Health(ctx context.Context, _ *pb.HealthRequest) (*pb.HealthResponse, error) {
	start := time.Now()
	resultStatus := "ok"
	defer func() {
		if r := recover(); r != nil {
			metrics.RecordGRPCRequest(ctx, "Health", "error", time.Since(start))
			panic(r)
		}
		metrics.RecordGRPCRequest(ctx, "Health", resultStatus, time.Since(start))
	}()

	components := make(map[string]string, 4)

	// WAL probe. Cheap, non-blocking; never round-trip to the broker —
	// otherwise the Docker HEALTHCHECK starves. See spec "Open questions
	// / risks: WAL IsConnected semantics differ across backends".
	if s.producer == nil {
		components["wal"] = "unknown"
	} else {
		components["wal"] = probeWAL(s.producer)
	}

	// Storage probe. The contract is "healthy" when a store handle is
	// wired (no live SQLite cursor check is performed); still report
	// "unknown" when no store handle is wired so a misconfigured server
	// doesn't silently report healthy. The nil check is on the concrete
	// *store.CanonicalStore pointer to avoid the typed-nil-in-interface
	// trap.
	if s.store == nil {
		components["storage"] = "unknown"
	} else {
		components["storage"] = probeStorage(ctx, s.store)
	}

	// Multi-node sharding info keys (node_id, assigned_tenants) are not
	// emitted yet — no Go sharding package is wired. Crucially, when they
	// DO land, they MUST be appended to `components` AFTER the `healthy`
	// gate below — the info keys MUST NOT count against health. Regression
	// pinned by TestHealth_MultiNodeInfoKeysDoNotGateHealth.

	healthy := components["wal"] == "healthy" && components["storage"] == "healthy"

	return &pb.HealthResponse{
		Healthy:    healthy,
		Version:    version.Version,
		Components: components,
	}, nil
}

// probeWAL returns the wal component string for HealthResponse.components.
// Caller must guarantee p is non-nil (typed-nil-in-interface guard lives
// in the Health handler so this helper can stay simple).
// Contract:
//   - producer w/o IsConnected() (e.g. in-memory) -> "healthy"
//   - IsConnected() panics -> "unknown"
//   - IsConnected() == true -> "healthy"
//   - IsConnected() == false -> "unhealthy"
func probeWAL(p any) (result string) {
	defer func() {
		if r := recover(); r != nil {
			result = "unknown"
		}
	}()
	pinger, ok := p.(walPinger)
	if !ok {
		return "healthy"
	}
	if pinger.IsConnected() {
		return "healthy"
	}
	return "unhealthy"
}

// probeStorage returns the storage component string. Caller must
// guarantee st is non-nil. Today has no SQLite probe, so we report
// "healthy" by default when a store is present. If the store ever
// grows a Health(ctx) hook, it's consulted here.
func probeStorage(ctx context.Context, st any) (result string) {
	defer func() {
		if r := recover(); r != nil {
			result = "unknown"
		}
	}()
	if pinger, ok := st.(storagePinger); ok {
		if err := pinger.Health(ctx); err != nil {
			return "unhealthy"
		}
	}
	return "healthy"
}
