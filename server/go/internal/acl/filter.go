// SPDX-License-Identifier: AGPL-3.0-only

package acl

import (
	"context"
)

// VisibilityReader is the bulk read-path predicate. Implements the SQL
// JOIN against node_visibility post-filter pattern documented in
// docs/go-port/shared/acl.md "Post-read filtering / SQL JOIN".
//
// store.CanonicalStore.GetVisibleNodeIDs already implements exactly
// this contract for the same-tenant case. The interface lets tests
// inject an in-memory map without spinning up SQLite.
type VisibilityReader interface {
	// VisibleNodeIDs returns the subset of nodeIDs that the actor set
	// (already group-resolved) can see by owner_actor or
	// node_visibility (including the tenant:* wildcard). Implementations
	// MUST handle the tenant:* expansion themselves — callers don't
	// pre-add it.
	VisibleNodeIDs(ctx context.Context, tenantID string, actorIDs []string, nodeIDs []string) (map[string]struct{}, error)
}

// Filter is the bulk-read variant of Checker. It returns the subset of
// nodeIDs the actor can READ — used by every multi-node read RPC
// (GetNodes, QueryNodes, GetConnectedNodes) to post-filter results
// before egress.
//
// The Filter does NOT enforce typed-capability requirements beyond
// READ; richer per-row checks (capability_mappings on field reads)
// belong on the Checker single-node path. The design uses a split:
// SQL JOIN for read filtering, single-node capability check for
// per-row authorization.
type Filter struct {
	resolver *Resolver
	vis      VisibilityReader
}

// NewFilter constructs a Filter. Both readers are required.
func NewFilter(resolver *Resolver, vis VisibilityReader) *Filter {
	return &Filter{resolver: resolver, vis: vis}
}

// FilterReadable returns the subset of nodeIDs the actor can read.
// Order is unspecified (callers should not rely on it).
//
// Special cases:
//   - System actor → returns nodeIDs unchanged (system bypass).
//   - Empty nodeIDs → returns nil.
//   - Empty actor → returns nil (zero-trust default).
func (f *Filter) FilterReadable(ctx context.Context, tenantID string, a Actor, nodeIDs []string) ([]string, error) {
	if len(nodeIDs) == 0 {
		return nil, nil
	}
	if a.IsZero() {
		return nil, nil
	}
	if a.IsSystem() {
		out := make([]string, len(nodeIDs))
		copy(out, nodeIDs)
		return out, nil
	}
	resolved, err := f.resolver.Expand(ctx, tenantID, a)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(resolved))
	for i, x := range resolved {
		ids[i] = x.String()
	}
	visible, err := f.vis.VisibleNodeIDs(ctx, tenantID, ids, nodeIDs)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(visible))
	for _, id := range nodeIDs {
		if _, ok := visible[id]; ok {
			out = append(out, id)
		}
	}
	return out, nil
}
