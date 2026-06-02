// SPDX-License-Identifier: AGPL-3.0-only

package apply

import (
	"sync/atomic"
	"time"
)

// progress is the applier's in-memory apply-progress signal (issue #653).
//
// It exists to make a STALLED applier observable. The applier appends to
// the WAL and ACKs writes independently of materialisation (ADR-016); if
// the applier is blocked mid-apply (e.g. SQLite lock contention on an
// existing shard — the prod failure behind #650) it keeps no externally
// visible signal apart from reads timing out. A stall is also
// indistinguishable from a healthy IDLE (caught-up) applier unless we
// track whether a batch is currently in-flight.
//
// Concurrency: the two timestamps are written ONLY by the serial
// Run/processBatch/finalizeBatch goroutine (never by the parallel apply
// workers — see ADR-027 invariant 3 in applier.go) and read concurrently
// by readiness probes, so both are plain atomics. No lock is taken on the
// read path, which keeps health checks non-blocking even when the applier
// is wedged.
type progress struct {
	// lastProgressMilli is the wall-clock (Unix millis) of the most
	// recent committed record. Updated at boot and on every commit.
	lastProgressMilli atomic.Int64
	// applyingSinceMilli is the wall-clock (Unix millis) the in-flight
	// batch began applying, or 0 when the applier is not currently
	// applying a batch (idle / caught up).
	applyingSinceMilli atomic.Int64
}

func (p *progress) markApplyingSince(nowMilli int64) { p.applyingSinceMilli.Store(nowMilli) }
func (p *progress) markIdle()                        { p.applyingSinceMilli.Store(0) }
func (p *progress) markProgress(nowMilli int64)      { p.lastProgressMilli.Store(nowMilli) }

// ApplierState is a lock-free snapshot of the applier's apply-progress
// signal, consumed by readiness checks.
type ApplierState struct {
	// LastProgressMilli is the Unix-millis of the most recent committed
	// record, or 0 if the applier has never made progress.
	LastProgressMilli int64
	// ApplyingSinceMilli is the Unix-millis the in-flight batch began
	// applying, or 0 when the applier is idle (caught up).
	ApplyingSinceMilli int64
}

// StalledFor reports how long the in-flight batch has gone WITHOUT FORWARD
// PROGRESS, as of nowMilli. It is the elapsed time since the later of (a)
// the batch start and (b) the most recent committed record — so a batch
// that is genuinely wedged (no commits) ages into a stall, while a slow
// but PROGRESSING batch (committing records, each advancing
// LastProgressMilli) resets the clock on every commit and is never
// flagged. It returns 0 when the applier is not currently applying a batch
// — an idle (caught-up) applier is NEVER stalled, which is the whole point
// of tracking ApplyingSinceMilli separately from LastProgressMilli.
//
// Using last-progress (not raw batch duration) is what prevents a
// legitimately long-but-healthy batch from tripping the readiness gate;
// only an absence of commits for `threshold` does.
func (s ApplierState) StalledFor(nowMilli int64) time.Duration {
	if s.ApplyingSinceMilli <= 0 {
		return 0
	}
	// Forward-progress reference: the batch hasn't progressed since the
	// later of its start and its last committed record.
	ref := s.ApplyingSinceMilli
	if s.LastProgressMilli > ref {
		ref = s.LastProgressMilli
	}
	if nowMilli <= ref {
		return 0
	}
	return time.Duration(nowMilli-ref) * time.Millisecond
}

// State returns a snapshot of the applier's apply-progress signal.
func (a *Applier) State() ApplierState {
	return ApplierState{
		LastProgressMilli:  a.prog.lastProgressMilli.Load(),
		ApplyingSinceMilli: a.prog.applyingSinceMilli.Load(),
	}
}

// Readiness is the applier's health view for readiness/liveness gating.
type Readiness struct {
	// State is one of "applying", "idle", "stalled", or "exited".
	State string
	// Stalled is true when the applier is wedged: either Run has exited,
	// or a batch has been applying longer than the (opt-in) stall
	// threshold. A readiness gate flips NOT_SERVING when this is true.
	Stalled bool
	// StalledFor is how long the in-flight batch has been applying when
	// State == "stalled" (0 otherwise).
	StalledFor time.Duration
}

const (
	applierStateApplying = "applying"
	applierStateIdle     = "idle"
	applierStateStalled  = "stalled"
	applierStateExited   = "exited"
)

// Readiness reports the applier's health as of nowMilli. threshold is the
// stall budget: when > 0, an in-flight batch older than threshold is
// reported as stalled (Stalled=true). threshold <= 0 disables stall
// gating (the apply-progress metrics still emit; only the gate is off) —
// the land-dark default, so existing deployments are not surprised. An
// EXITED applier is always reported stalled regardless of threshold,
// because a dead applier is unambiguously broken, not a tunable.
func (a *Applier) Readiness(nowMilli int64, threshold time.Duration) Readiness {
	if a.exited.Load() {
		return Readiness{State: applierStateExited, Stalled: true}
	}
	st := a.State()
	stalledFor := st.StalledFor(nowMilli)
	if threshold > 0 && stalledFor >= threshold {
		return Readiness{State: applierStateStalled, Stalled: true, StalledFor: stalledFor}
	}
	if st.ApplyingSinceMilli > 0 {
		return Readiness{State: applierStateApplying}
	}
	return Readiness{State: applierStateIdle}
}
