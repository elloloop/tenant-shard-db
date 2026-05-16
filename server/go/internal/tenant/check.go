// Package tenant is the per-RPC tenant gate. Every tenant-scoped
// handler calls CheckTenant before doing real work. The gate enforces
// three independent contracts:
//
//  1. Tenant existence — missing tenant -> codes.NotFound.
//  2. Sharding ownership — a different node owns this tenant ->
//     codes.Unavailable plus an `entdb-redirect-node` trailer carrying
//     the owning node id (so SDK redirect caches can retry there).
//  3. Region pinning — tenant_registry.region != this node's region
//     -> codes.FailedPrecondition (permanent for this node;
//     UNAVAILABLE would invite retries, see Python comment at
//     api/grpc_server.py:399).
//
// Spec: docs/go-port/shared/error-mapping.md, plus the per-RPC specs
// in docs/go-port/rpcs/* (Health.md, GetMailbox.md). Source-of-truth
// (`_check_tenant`) and sharding.py.
//
// Single-node default: an unset Sharding ({}) is treated as
// "always-mine" — every tenant is owned by this node. This matches the
// Python ShardingConfig with empty assigned_tenants
// (sharding.py:85-97).

package tenant

import (
	"context"
	"fmt"

	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
	"google.golang.org/grpc/codes"
)

// Sharding describes node identity and tenant ownership for the
// CheckTenant gate. A zero value is the single-node default — IsMine
// returns true for everything.
//
// IsMine and Owner are functions rather than maps so the gate works
// with any backend (env-driven map, consul, etc.) without coupling
// this package to a specific config source.
type Sharding struct {
	// NodeID is this node's identifier (e.g. "node-a"). Used as the
	// fallback Owner when a tenant is owned by us — handlers that emit
	// a redirect trailer expect a non-empty owner.
	NodeID string

	// AssignedTenants is the set of tenant IDs this node owns. Empty
	// (nil) means single-node mode — IsMine returns true for all
	// tenant IDs.
	AssignedTenants []string

	// IsMine reports whether this node owns the given tenant. nil is
	// treated as "always true" (matches sharding.py:95-97 — empty
	// assigned_tenants accepts every tenant).
	IsMine func(tenantID string) bool

	// Owner returns the node id that owns the given tenant, or "" if
	// unknown. Used to populate the entdb-redirect-node trailer on
	// UNAVAILABLE. nil is treated as "unknown for every tenant".
	Owner func(tenantID string) string
}

// isMine is the safe IsMine wrapper. nil function => true.
func (s *Sharding) isMine(tenantID string) bool {
	if s == nil || s.IsMine == nil {
		return true
	}
	return s.IsMine(tenantID)
}

// owner is the safe Owner wrapper. nil function => "".
func (s *Sharding) owner(tenantID string) string {
	if s == nil || s.Owner == nil {
		return ""
	}
	return s.Owner(tenantID)
}

// Options describes the per-call/per-server config that CheckTenant
// needs but is not the tenant id itself.
type Options struct {
	// ServedRegion is the region this node is configured to serve
	// (e.g. "us-east-1"). Empty => region pinning is not enforced.
	ServedRegion string
}

// CheckTenant is the gate. It MUST be called from every tenant-scoped
// gRPC handler before reading or writing per-tenant state.
//
// Returns nil on success. On failure, returns an errs.* sentinel-bearing
// error. For the sharding-mismatch case it also calls
// errs.SetRedirectTrailer to attach the entdb-redirect-node trailer
// (the trailer must be set before the handler returns the error;
// grpc-go flushes trailers with the closing status — same contract the
// Python comment at api/grpc_server.py:390 documents).
//
// gs may be nil; in that case the region check is skipped (matches the
// Python guard at api/grpc_server.py:400 — region pinning only fires
// when global_store is wired up).
func CheckTenant(ctx context.Context, tenantID string, gs *globalstore.GlobalStore, sh *Sharding, opts Options) error {
	if tenantID == "" {
		return errs.Errorf(codes.InvalidArgument, "tenant: empty tenant_id")
	}

	// 1. Sharding ownership. Set the trailer FIRST, then return the
	//    UNAVAILABLE error — once the handler returns, grpc-go closes
	//    the call and trailers are no longer mutable.
	if !sh.isMine(tenantID) {
		owner := sh.owner(tenantID)
		hint := ""
		if owner != "" {
			hint = fmt.Sprintf(" (try node %s)", owner)
			if err := errs.SetRedirectTrailer(ctx, owner); err != nil {
				// Failing to set the trailer is a non-fatal
				// programming-error signal; we still return the
				// UNAVAILABLE so the SDK at least sees the right
				// status code.
				_ = err
			}
		}
		return errs.Errorf(codes.Unavailable,
			"tenant %q is not served by this node%s", tenantID, hint)
	}

	// 2. Tenant existence + region pinning. Both require globalstore.
	//    Missing globalstore => skip both checks (single-node bring-up
	//    path); same as the Python `if self.global_store is not None`
	//    guard.
	if gs == nil {
		return nil
	}
	t, err := gs.GetTenant(ctx, tenantID)
	if err != nil {
		// A scan/IO error from globalstore is an internal fault. We
		// don't want to upgrade it to NotFound; surface it so the
		// recover-and-wrap fallback in the gRPC layer can label it.
		return fmt.Errorf("tenant: globalstore.GetTenant: %w", err)
	}
	if t == nil {
		return errs.Errorf(codes.NotFound, "tenant %q does not exist", tenantID)
	}

	// 3. Region pinning. ServedRegion empty => not enforced.
	if opts.ServedRegion != "" && t.Region != "" && t.Region != opts.ServedRegion {
		return errs.Errorf(codes.FailedPrecondition,
			"tenant %q is pinned to region %q; this node serves %q",
			tenantID, t.Region, opts.ServedRegion)
	}

	return nil
}
