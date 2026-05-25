// SPDX-License-Identifier: AGPL-3.0-only

// Durable checkpoint store for the Event Hubs WAL backend (#570).
//
// The backend tracks the last-committed per-partition sequence number so a
// re-Subscribe resumes after it. By default that state lives only in
// memory, so a process restart replays the hub from the configured start
// position. A CheckpointStore persists it (Azure Blob) so restarts resume
// where they left off.
//
// The blob store is split behind a tiny blobClient seam: the store LOGIC
// (marshal the checkpoint map, read-through on miss) is unit-tested with a
// fake, and only the thin azblob upload/download wrapper is untested I/O —
// consistent with how the other cloud backends isolate their SDK calls.

package wal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
)

// CheckpointStore persists the per-partition checkpoint map (partition id
// → last committed sequence number) across process restarts.
type CheckpointStore interface {
	// Load returns the persisted checkpoints, or an empty map if none have
	// been saved yet. A genuine read fault is returned as an error.
	Load(ctx context.Context) (map[string]int64, error)
	// Save persists the full checkpoint map. Callers hold the backend lock,
	// so saves are serialized.
	Save(ctx context.Context, checkpoints map[string]int64) error
}

// blobClient is the minimal blob I/O the checkpoint store needs. It exists
// so the store logic is testable without Azure: download returns
// (nil, nil) when the blob does not exist yet.
type blobClient interface {
	upload(ctx context.Context, data []byte) error
	download(ctx context.Context) ([]byte, error)
}

// blobCheckpointStore persists the checkpoint map as a single JSON blob.
type blobCheckpointStore struct {
	client blobClient
}

// Load reads + decodes the checkpoint blob. A missing blob (first run) is
// not an error — it yields an empty map.
func (s *blobCheckpointStore) Load(ctx context.Context) (map[string]int64, error) {
	data, err := s.client.download(ctx)
	if err != nil {
		return nil, fmt.Errorf("eventhubs checkpoint: load: %w", err)
	}
	out := map[string]int64{}
	if len(data) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("eventhubs checkpoint: decode: %w", err)
	}
	return out, nil
}

// Save encodes + writes the full checkpoint map.
func (s *blobCheckpointStore) Save(ctx context.Context, checkpoints map[string]int64) error {
	data, err := json.Marshal(checkpoints)
	if err != nil {
		return fmt.Errorf("eventhubs checkpoint: encode: %w", err)
	}
	if err := s.client.upload(ctx, data); err != nil {
		return fmt.Errorf("eventhubs checkpoint: save: %w", err)
	}
	return nil
}

// azblobClient is the azblob-backed blobClient (the thin untested I/O
// wrapper). It targets a single (container, blob) holding the JSON map.
type azblobClient struct {
	client    *azblob.Client
	container string
	blobName  string
}

func (a *azblobClient) upload(ctx context.Context, data []byte) error {
	_, err := a.client.UploadBuffer(ctx, a.container, a.blobName, data, nil)
	return err
}

func (a *azblobClient) download(ctx context.Context) ([]byte, error) {
	resp, err := a.client.DownloadStream(ctx, a.container, a.blobName, nil)
	if err != nil {
		// A not-yet-created checkpoint blob is the first-run case, not a
		// fault — surface it as "no data".
		if bloberror.HasCode(err, bloberror.BlobNotFound, bloberror.ContainerNotFound) {
			return nil, nil
		}
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, resp.Body); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// NewBlobCheckpointStore builds a CheckpointStore backed by an Azure Blob
// Storage blob. connStr is a storage-account connection string; container
// must already exist; blobName is the object holding the JSON checkpoint
// map (one per WAL hub + consumer group).
func NewBlobCheckpointStore(connStr, container, blobName string) (CheckpointStore, error) {
	if connStr == "" || container == "" || blobName == "" {
		return nil, errors.New("eventhubs checkpoint: connection string, container, and blob name are required")
	}
	client, err := azblob.NewClientFromConnectionString(connStr, nil)
	if err != nil {
		return nil, fmt.Errorf("eventhubs checkpoint: azblob client: %w", err)
	}
	return &blobCheckpointStore{client: &azblobClient{client: client, container: container, blobName: blobName}}, nil
}
