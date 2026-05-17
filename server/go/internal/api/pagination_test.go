// SPDX-License-Identifier: AGPL-3.0-only

// Unit tests for the SEC-4 (#135) pagination clamp helper. Lives in
// `package api` (not api_test) so it can exercise the unexported
// clampPageSize directly. Handler-level behaviour (the clamp actually
// being applied per RPC) is pinned in pagination_clamp_test.go.

package api

import "testing"

func TestMaxPageSize_MatchesSharedCeiling(t *testing.T) {
	t.Parallel()
	// MaxPageSize must stay equal to the value GetConnectedNodes and
	// ListSharedWithMe already enforce, so the whole read surface caps
	// at one number. If any of these constants is retuned, retune all
	// of them together (and storeMaxQueryLimit in the store package).
	if MaxPageSize != 1000 {
		t.Fatalf("MaxPageSize = %d, want 1000", MaxPageSize)
	}
	if MaxConnectedResultLimit != MaxPageSize {
		t.Fatalf("MaxConnectedResultLimit (%d) != MaxPageSize (%d)",
			MaxConnectedResultLimit, MaxPageSize)
	}
	if listSharedWithMeMaxLimit != MaxPageSize {
		t.Fatalf("listSharedWithMeMaxLimit (%d) != MaxPageSize (%d)",
			listSharedWithMeMaxLimit, MaxPageSize)
	}
}

func TestClampPageSize(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   int
		want int
	}{
		{"under cap is unchanged", 50, 50},
		{"exactly at cap is unchanged", MaxPageSize, MaxPageSize},
		{"one over cap is clamped", MaxPageSize + 1, MaxPageSize},
		{"absurd DoS limit is clamped", 10_000_000, MaxPageSize},
		{"max int is clamped", int(^uint(0) >> 1), MaxPageSize},
		// Non-positive inputs are passed through untouched: callers
		// coerce the "<=0 => default" case BEFORE calling this, so the
		// helper must not turn a sentinel into 1000.
		{"zero passes through", 0, 0},
		{"negative passes through", -1, -1},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := clampPageSize(tc.in); got != tc.want {
				t.Fatalf("clampPageSize(%d) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}
