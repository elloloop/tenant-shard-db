// SPDX-License-Identifier: AGPL-3.0-only

package apply

import (
	"testing"
	"time"
)

// TestApplierState_StalledFor pins the idle-vs-stalled distinction (#653):
// an applier that is not currently applying a batch (ApplyingSinceMilli==0)
// is NEVER stalled, no matter how old its last progress is — that is the
// whole reason ApplyingSinceMilli is tracked separately.
func TestApplierState_StalledFor(t *testing.T) {
	const now = int64(1_700_000_000_000)
	cases := []struct {
		name          string
		lastProgress  int64
		applyingSince int64
		want          time.Duration
	}{
		// LastProgress older than the batch start => the wedge clock runs
		// from batch start (no commits yet this batch).
		{"idle (not applying)", now - 1_000, 0, 0},
		{"just started, no commit yet", now - 60_000, now, 0},
		{"blocked 5s on first record", now - 60_000, now - 5_000, 5 * time.Second},
		{"blocked 30s on first record", now - 60_000, now - 30_000, 30 * time.Second},
		// Slow but PROGRESSING: batch started 30s ago but a record committed
		// 0.5s ago -> only 0.5s without forward progress, NOT a stall. This
		// is the false-positive the per-record progress signal prevents.
		{"slow but progressing (commit 0.5s ago)", now - 500, now - 30_000, 500 * time.Millisecond},
		{"progressing (commit just now)", now, now - 30_000, 0},
		{"clock skew (since in future)", now - 1_000, now + 1_000, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st := ApplierState{LastProgressMilli: tc.lastProgress, ApplyingSinceMilli: tc.applyingSince}
			if got := st.StalledFor(now); got != tc.want {
				t.Fatalf("StalledFor = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestApplier_Readiness_SlowBatchNotStalled pins that a long-running batch
// that keeps committing records is NOT flagged stalled even past the
// threshold (#653 review): the readiness gate fires on absence of forward
// progress, not on raw batch duration.
func TestApplier_Readiness_SlowBatchNotStalled(t *testing.T) {
	const now = int64(1_700_000_000_000)
	a := &Applier{}
	a.prog.applyingSinceMilli.Store(now - 5*60_000) // batch in-flight 5 minutes
	a.prog.lastProgressMilli.Store(now - 200)       // but committed a record 200ms ago

	rd := a.Readiness(now, time.Minute)
	if rd.Stalled || rd.State != applierStateApplying {
		t.Fatalf("slow-but-progressing batch readiness = %+v, want applying/not-stalled", rd)
	}
}

// TestApplier_Readiness pins the readiness state machine: exited > stalled
// (only when threshold>0) > applying > idle, and that threshold<=0
// disables stall gating while an exited applier always gates.
func TestApplier_Readiness(t *testing.T) {
	const now = int64(1_700_000_000_000)

	// lastProgress is set well in the past (older than every applyingSince
	// below), modelling the realistic blocked case: no record has committed
	// in the current batch, so the wedge clock runs from the batch start.
	// (Slow-but-progressing — lastProgress newer than applyingSince — is
	// covered by TestApplier_Readiness_SlowBatchNotStalled.)
	newApplier := func(applyingSince int64, exited bool) *Applier {
		a := &Applier{}
		a.prog.lastProgressMilli.Store(now - 600_000)
		a.prog.applyingSinceMilli.Store(applyingSince)
		a.exited.Store(exited)
		return a
	}

	t.Run("idle when not applying", func(t *testing.T) {
		rd := newApplier(0, false).Readiness(now, time.Minute)
		if rd.State != applierStateIdle || rd.Stalled {
			t.Fatalf("got %+v, want idle/not-stalled", rd)
		}
	})

	t.Run("applying within threshold", func(t *testing.T) {
		rd := newApplier(now-10_000, false).Readiness(now, time.Minute)
		if rd.State != applierStateApplying || rd.Stalled {
			t.Fatalf("got %+v, want applying/not-stalled", rd)
		}
	})

	t.Run("stalled past threshold", func(t *testing.T) {
		rd := newApplier(now-90_000, false).Readiness(now, time.Minute)
		if rd.State != applierStateStalled || !rd.Stalled {
			t.Fatalf("got %+v, want stalled", rd)
		}
		if rd.StalledFor != 90*time.Second {
			t.Fatalf("StalledFor = %v, want 90s", rd.StalledFor)
		}
	})

	t.Run("threshold 0 disables stall gating (still reports applying)", func(t *testing.T) {
		rd := newApplier(now-3_600_000, false).Readiness(now, 0)
		if rd.State != applierStateApplying || rd.Stalled {
			t.Fatalf("got %+v, want applying/not-stalled (gating off)", rd)
		}
	})

	t.Run("exited always gates regardless of threshold", func(t *testing.T) {
		for _, th := range []time.Duration{0, time.Minute} {
			rd := newApplier(0, true).Readiness(now, th)
			if rd.State != applierStateExited || !rd.Stalled {
				t.Fatalf("threshold=%v: got %+v, want exited/stalled", th, rd)
			}
		}
	})
}
