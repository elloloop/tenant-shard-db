// SPDX-License-Identifier: AGPL-3.0-only

package audit

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

func TestRunOnceArchivesGzipJSONLWithLockMetadata(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC)
	rec := archiveTestRecord(t, "tenant-1", "idem-1", 2, 41)
	consumer := &fakeArchiveConsumer{records: []wal.Record{rec}}
	store := &fakeObjectLockStore{}
	archiver, err := NewArchiver(Options{
		Consumer:      consumer,
		Store:         store,
		Topic:         "entdb-wal",
		GroupID:       "archive",
		RetentionDays: 30,
		NowFn:         func() time.Time { return now },
		LegalHoldFunc: func(_ context.Context, tenantID string) (bool, error) {
			return tenantID == "tenant-1", nil
		},
	})
	if err != nil {
		t.Fatalf("NewArchiver: %v", err)
	}

	archived, err := archiver.RunOnce(ctx)
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if archived != 1 {
		t.Fatalf("archived=%d, want 1", archived)
	}
	if len(store.objects) != 1 {
		t.Fatalf("objects=%d, want 1", len(store.objects))
	}
	if len(consumer.commits) != 1 {
		t.Fatalf("commits=%d, want 1", len(consumer.commits))
	}

	obj := store.objects[0]
	if obj.Key != "wal/entdb-wal/2/41-41.jsonl.gz" {
		t.Fatalf("key=%q", obj.Key)
	}
	if obj.ContentType != "application/jsonl" || obj.ContentEncoding != "gzip" {
		t.Fatalf("content headers=(%q,%q)", obj.ContentType, obj.ContentEncoding)
	}
	if !obj.LegalHold || obj.Metadata["entdb-legal-hold"] != "true" {
		t.Fatalf("legal hold not set: obj=%v metadata=%v", obj.LegalHold, obj.Metadata)
	}
	if !obj.RetainUntil.Equal(now.Add(30 * 24 * time.Hour)) {
		t.Fatalf("retain until=%s", obj.RetainUntil)
	}
	if obj.Metadata["entdb-format"] != archiveFormatVersion ||
		obj.Metadata["entdb-start-offset"] != "41" ||
		obj.Metadata["entdb-end-offset"] != "41" ||
		obj.Metadata["entdb-record-count"] != "1" ||
		obj.Metadata["entdb-tenant-sample"] != "tenant-1" ||
		obj.Metadata["entdb-sha256"] == "" {
		t.Fatalf("unexpected metadata: %#v", obj.Metadata)
	}

	lines := gunzipLines(t, obj.Body)
	if len(lines) != 1 {
		t.Fatalf("lines=%d, want 1", len(lines))
	}
	var line archiveLine
	if err := json.Unmarshal([]byte(lines[0]), &line); err != nil {
		t.Fatalf("unmarshal line: %v", err)
	}
	if line.Topic != "entdb-wal" || line.Partition != 2 || line.Offset != 41 || line.Key != "tenant-1" {
		t.Fatalf("unexpected line metadata: %#v", line)
	}
	if line.Headers[wal.HeaderIdempotencyKey] != base64.StdEncoding.EncodeToString([]byte("idem-1")) {
		t.Fatalf("unexpected headers: %#v", line.Headers)
	}
	var ev wal.Event
	if err := json.Unmarshal(line.Value, &ev); err != nil {
		t.Fatalf("unmarshal value: %v", err)
	}
	if ev.TenantID != "tenant-1" || ev.IdempotencyKey != "idem-1" {
		t.Fatalf("unexpected archived event: %#v", ev)
	}
}

func TestRunOnceDoesNotCommitWhenArchiveWriteFails(t *testing.T) {
	ctx := context.Background()
	consumer := &fakeArchiveConsumer{
		records: []wal.Record{archiveTestRecord(t, "tenant-1", "idem-1", 0, 7)},
	}
	store := &fakeObjectLockStore{putErr: errors.New("s3 unavailable")}
	archiver, err := NewArchiver(Options{
		Consumer: consumer,
		Store:    store,
		Topic:    "entdb-wal",
		GroupID:  "archive",
	})
	if err != nil {
		t.Fatalf("NewArchiver: %v", err)
	}

	archived, err := archiver.RunOnce(ctx)
	if err == nil {
		t.Fatal("RunOnce error = nil, want write failure")
	}
	if archived != 0 {
		t.Fatalf("archived=%d, want 0", archived)
	}
	if len(consumer.commits) != 0 {
		t.Fatalf("commits=%d, want 0", len(consumer.commits))
	}
}

func archiveTestRecord(t *testing.T, tenantID, idem string, partition int32, offset int64) wal.Record {
	t.Helper()
	ev := wal.Event{
		TenantID:       tenantID,
		Actor:          "actor-1",
		IdempotencyKey: idem,
		TsMs:           1000 + offset,
		Ops:            []map[string]any{{"op": "noop"}},
	}
	value, err := ev.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	return wal.Record{
		Key:   tenantID,
		Value: value,
		Position: wal.StreamPos{
			Topic:       "entdb-wal",
			Partition:   partition,
			Offset:      offset,
			TimestampMs: 2000 + offset,
		},
		Headers: map[string][]byte{wal.HeaderIdempotencyKey: []byte(idem)},
	}
}

type fakeArchiveConsumer struct {
	records []wal.Record
	commits []wal.Record
	pollErr error
}

func (f *fakeArchiveConsumer) Subscribe(context.Context, string, string) (<-chan wal.Record, <-chan error, error) {
	return nil, nil, errors.New("not implemented")
}

func (f *fakeArchiveConsumer) PollBatch(context.Context, string, string, int, time.Duration) ([]wal.Record, error) {
	if f.pollErr != nil {
		return nil, f.pollErr
	}
	return append([]wal.Record(nil), f.records...), nil
}

func (f *fakeArchiveConsumer) Commit(_ context.Context, _ string, rec wal.Record) error {
	f.commits = append(f.commits, rec)
	return nil
}

type fakeObjectLockStore struct {
	objects []ArchiveObject
	putErr  error
}

func (f *fakeObjectLockStore) VerifyObjectLock(context.Context) error { return nil }

func (f *fakeObjectLockStore) PutLockedObject(_ context.Context, obj ArchiveObject) error {
	if f.putErr != nil {
		return f.putErr
	}
	f.objects = append(f.objects, obj)
	return nil
}

func gunzipLines(t *testing.T, body []byte) []string {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	defer func() { _ = gz.Close() }()
	raw, err := io.ReadAll(gz)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan: %v", err)
	}
	return lines
}
