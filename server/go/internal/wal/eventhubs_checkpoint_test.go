// SPDX-License-Identifier: AGPL-3.0-only

package wal

import (
	"context"
	"sync"
	"testing"
	"time"
)

// fakeBlobClient is an in-memory blobClient: download returns (nil, nil)
// until something is uploaded (the first-run / missing-blob case).
type fakeBlobClient struct {
	mu   sync.Mutex
	data []byte
	set  bool
}

func (f *fakeBlobClient) upload(_ context.Context, data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data = append([]byte(nil), data...)
	f.set = true
	return nil
}

func (f *fakeBlobClient) download(_ context.Context) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.set {
		return nil, nil
	}
	return append([]byte(nil), f.data...), nil
}

func TestBlobCheckpointStore_RoundTrip(t *testing.T) {
	t.Parallel()
	store := &blobCheckpointStore{client: &fakeBlobClient{}}
	ctx := context.Background()

	// Missing blob ⇒ empty map, not an error.
	got, err := store.Load(ctx)
	if err != nil {
		t.Fatalf("Load (empty): %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("Load (empty) = %v, want empty", got)
	}

	want := map[string]int64{"0": 42, "1": 9007199254740993} // incl. an int64 > 2^53
	if err := store.Save(ctx, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err = store.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got) != 2 || got["0"] != 42 || got["1"] != 9007199254740993 {
		t.Fatalf("round-trip = %v, want %v", got, want)
	}
}

// fakeCheckpointStore is an in-memory CheckpointStore that records saves.
type fakeCheckpointStore struct {
	mu    sync.Mutex
	data  map[string]int64
	saves int
}

func (f *fakeCheckpointStore) Load(_ context.Context) (map[string]int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[string]int64{}
	for k, v := range f.data {
		out[k] = v
	}
	return out, nil
}

func (f *fakeCheckpointStore) Save(_ context.Context, cp map[string]int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data = map[string]int64{}
	for k, v := range cp {
		f.data[k] = v
	}
	f.saves++
	return nil
}

func TestEventHubs_CheckpointStore_LoadOnConnect(t *testing.T) {
	t.Parallel()
	fakeStore := &fakeCheckpointStore{data: map[string]int64{"0": 99}}
	fake := newFakeEventHubs()

	e := NewEventHubs(DefaultEventHubsConfig("Endpoint=sb://fake/;SharedAccessKeyName=x;SharedAccessKey=y", "entdb-wal", ""))
	e.config.CheckpointStore = fakeStore
	e.newClient = func(ctx context.Context) (EventHubsAPI, error) { return fake, nil }
	if err := e.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { _ = e.Close(context.Background()) })

	if e.checkpoints["0"] != 99 {
		t.Fatalf("checkpoints not restored on Connect: got %v, want {0:99}", e.checkpoints)
	}
}

func TestEventHubs_CheckpointStore_SaveOnCommit(t *testing.T) {
	t.Parallel()
	fakeStore := &fakeCheckpointStore{}
	fake := newFakeEventHubs()

	e := NewEventHubs(DefaultEventHubsConfig("Endpoint=sb://fake/;SharedAccessKeyName=x;SharedAccessKey=y", "entdb-wal", ""))
	e.config.MaxWaitTime = 100 * time.Millisecond
	e.config.CheckpointStore = fakeStore
	e.newClient = func(ctx context.Context) (EventHubsAPI, error) { return fake, nil }
	if err := e.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(func() { _ = e.Close(context.Background()) })
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if _, err := e.Append(ctx, "entdb-wal", "tenant_a", []byte("v"), nil); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	got, err := e.PollBatch(ctx, "entdb-wal", "$Default", 10, 300*time.Millisecond)
	if err != nil || len(got) == 0 {
		t.Fatalf("PollBatch: got %d records, err=%v", len(got), err)
	}
	if err := e.Commit(ctx, "$Default", got[0]); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// The commit must have been persisted to the durable store.
	fakeStore.mu.Lock()
	saves, persisted := fakeStore.saves, len(fakeStore.data)
	fakeStore.mu.Unlock()
	if saves == 0 {
		t.Fatal("Commit did not persist to the checkpoint store")
	}
	if persisted == 0 {
		t.Fatal("persisted checkpoint map is empty after Commit")
	}
}
