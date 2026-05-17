// SPDX-License-Identifier: AGPL-3.0-only

// Pagination guardrails (issue #135, SEC-4).
//
// Several read RPCs historically applied only a default-when-zero limit
// with NO upper ceiling. A hostile or buggy client passing
// limit=10_000_000 would make the server materialise that many protos
// in memory — an unauthenticated-amplification DoS.
//
// MaxPageSize is the single server-wide ceiling for a paged read. It
// matches the value GetConnectedNodes (MaxConnectedResultLimit) and
// ListSharedWithMe (listSharedWithMeMaxLimit) already enforce, so the
// whole read surface now shares one cap.
//
// The clamp is applied at handler entry for every uncapped read RPC
// (QueryNodes, SearchNodes, ListUsers, GetEdgesFrom, GetEdgesTo) AFTER
// the existing "<=0 => default" coercion, so small/default page sizes
// are unaffected and only oversized requests are trimmed. The store
// layer (store.QueryNodes) re-applies the same ceiling as
// defence-in-depth for any future caller that bypasses the handler.

package api

// MaxPageSize is the maximum number of rows any single paged read RPC
// will return. Requests asking for more are silently clamped to this
// value (no error — pagination clients keep working, they just page
// more). 1000 matches MaxConnectedResultLimit and
// listSharedWithMeMaxLimit so the read surface is uniform.
const MaxPageSize = 1000

// clampPageSize applies the MaxPageSize ceiling. It does NOT touch the
// "<=0 => default" behaviour: callers must coerce the default first,
// then pass the already-defaulted positive limit through here. A
// non-positive input is returned unchanged so an accidental call order
// can't turn "use default" into "return 1000".
func clampPageSize(limit int) int {
	if limit > MaxPageSize {
		return MaxPageSize
	}
	return limit
}
