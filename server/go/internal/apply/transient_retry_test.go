// SPDX-License-Identifier: AGPL-3.0-only

package apply_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/elloloop/tenant-shard-db/server/go/internal/apply"
	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

// scriptedConsumer is a minimal wal.Consumer whose PollBatch returns a
// scripted (records, error) per 1-based poll number and signals each
// poll on pollCh so a test can observe whether Run retried (issue #627).
type scriptedConsumer struct {
	mu     sync.Mutex
	n      int
	pollCh chan int
	poll   func(n int) ([]wal.Record, error)
}

func (c *scriptedConsumer) PollBatch(ctx context.Context, _ string, _ string, _ int, _ time.Duration) ([]wal.Record, error) {
	c.mu.Lock()
	c.n++
	n := c.n
	c.mu.Unlock()
	select {
	case c.pollCh <- n:
	default:
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return c.poll(n)
}

func (c *scriptedConsumer) Commit(context.Context, string, wal.Record) error { return nil }

func (c *scriptedConsumer) Subscribe(context.Context, string, string) (<-chan wal.Record, <-chan error, error) {
	return nil, nil, errors.New("scriptedConsumer.Subscribe: unused")
}

func (c *scriptedConsumer) polls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

// transientPollErr mirrors what kafka.go's wrapPollErr produces for an
// idle-connection EOF: tagged with both ErrWal and ErrTransient.
func transientPollErr() error {
	return fmt.Errorf("%w: %w: kafka poll: EOF", wal.ErrWal, wal.ErrTransient)
}

func newApplierWithConsumer(t *testing.T, c wal.Consumer, opts apply.Options) *apply.Applier {
	t.Helper()
	dir := t.TempDir()
	cs, err := store.New(store.Options{RootDir: dir, WALMode: true})
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	gs, err := globalstore.New(globalstore.Options{DataDir: dir, WALMode: true})
	if err != nil {
		t.Fatalf("globalstore.New: %v", err)
	}
	t.Cleanup(func() { _ = gs.Close() })

	opts.Store = cs
	opts.Global = gs
	opts.Consumer = c
	opts.Topic = "entdb-wal-transient-test"
	opts.GroupID = "applier-transient-test"
	a, err := apply.New(opts)
	if err != nil {
		t.Fatalf("apply.New: %v", err)
	}
	return a
}

func waitForPolls(t *testing.T, ch <-chan int, want int, timeout time.Duration) {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case n := <-ch:
			if n >= want {
				return
			}
		case <-deadline:
			t.Fatalf("applier did not reach poll #%d within %s — it exited on the transient WAL error instead of retrying (issue #627)", want, timeout)
		}
	}
}

// TestApplier_RetriesTransientPollError surfaces issue #627: a transient
// WAL poll error (e.g. Azure Event Hubs idle-connection EOF) must NOT
// crash the applier. The fake returns a transient error on the first
// poll, then drains empty; the applier must keep polling rather than
// return the error. Pre-fix, Run returns on poll #1 and never reaches
// poll #2, so waitForPolls fails.
func TestApplier_RetriesTransientPollError(t *testing.T) {
	c := &scriptedConsumer{
		pollCh: make(chan int, 128),
		poll: func(n int) ([]wal.Record, error) {
			if n == 1 {
				return nil, transientPollErr()
			}
			return nil, nil // drained
		},
	}
	a := newApplierWithConsumer(t, c, apply.Options{TransientPollBackoff: time.Millisecond})

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- a.Run(ctx) }()

	// Must poll at least twice: transient error on #1, retry on #2.
	waitForPolls(t, c.pollCh, 2, 3*time.Second)

	cancel()
	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run returned %v; want context.Canceled (the transient error must be retried, not surfaced)", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}

// TestApplier_FatalPollErrorExits ensures the retry path does not swallow
// genuinely-fatal poll errors: a non-transient error still stops Run.
func TestApplier_FatalPollErrorExits(t *testing.T) {
	fatal := fmt.Errorf("%w: kafka poll: SASL authentication failed", wal.ErrWal)
	c := &scriptedConsumer{
		pollCh: make(chan int, 128),
		poll:   func(int) ([]wal.Record, error) { return nil, fatal },
	}
	a := newApplierWithConsumer(t, c, apply.Options{TransientPollBackoff: time.Millisecond})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- a.Run(ctx) }()
	select {
	case err := <-errCh:
		if err == nil || errors.Is(err, context.Canceled) {
			t.Fatalf("Run returned %v; a fatal poll error must stop the applier", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit on a fatal poll error")
	}
}

// TestApplier_GivesUpAfterMaxTransientStreak verifies the opt-in safety
// valve: with a cap set, persistent transient errors eventually exit
// (after at least the cap's worth of retries, not on the first error).
func TestApplier_GivesUpAfterMaxTransientStreak(t *testing.T) {
	c := &scriptedConsumer{
		pollCh: make(chan int, 128),
		poll:   func(int) ([]wal.Record, error) { return nil, transientPollErr() },
	}
	a := newApplierWithConsumer(t, c, apply.Options{
		TransientPollBackoff:   time.Millisecond,
		MaxTransientPollStreak: 3,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- a.Run(ctx) }()
	select {
	case err := <-errCh:
		if err == nil || errors.Is(err, context.Canceled) {
			t.Fatalf("Run returned %v; want a give-up error after the transient streak cap", err)
		}
		if !errors.Is(err, wal.ErrTransient) {
			t.Errorf("give-up error should wrap the transient cause: %v", err)
		}
		if got := c.polls(); got < 3 {
			t.Errorf("expected >=3 retries before giving up, got %d polls (pre-fix it exits on the first error)", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not give up after MaxTransientPollStreak transient errors")
	}
}
