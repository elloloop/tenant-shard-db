// SPDX-License-Identifier: AGPL-3.0-only

package audit

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/elloloop/tenant-shard-db/server/go/internal/metrics"
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

const (
	defaultArchiveGroupID       = "entdb-wal-archive"
	defaultArchiveBatchSize     = 128
	defaultArchiveBatchBytes    = 10 << 20
	defaultArchivePollTimeout   = time.Second
	defaultArchiveRetryBackoff  = 5 * time.Second
	defaultArchiveRetentionDays = 2557
	archiveFormatVersion        = "wal-jsonl-gzip-v1"
)

// LegalHoldFunc reports whether records for tenantID should be archived
// with an S3 legal hold flag.
type LegalHoldFunc func(ctx context.Context, tenantID string) (bool, error)

type Options struct {
	Consumer wal.Consumer
	Store    ObjectLockStore
	Topic    string
	GroupID  string

	RetentionDays int
	BatchSize     int
	BatchBytes    int
	PollTimeout   time.Duration
	RetryBackoff  time.Duration

	LegalHoldFunc   LegalHoldFunc
	NowFn           func() time.Time
	SkipVerifyOnRun bool
}

// Archiver consumes the WAL with an independent consumer group and
// writes immutable gzip JSONL archive objects to ObjectLockStore.
type Archiver struct {
	consumer wal.Consumer
	store    ObjectLockStore
	topic    string
	groupID  string

	retentionDays int
	batchSize     int
	batchBytes    int
	pollTimeout   time.Duration
	retryBackoff  time.Duration

	legalHoldFunc   LegalHoldFunc
	nowFn           func() time.Time
	skipVerifyOnRun bool
}

func NewArchiver(opts Options) (*Archiver, error) {
	if opts.Consumer == nil {
		return nil, errors.New("audit: Options.Consumer is required")
	}
	if opts.Store == nil {
		return nil, errors.New("audit: Options.Store is required")
	}
	if strings.TrimSpace(opts.Topic) == "" {
		return nil, errors.New("audit: Options.Topic is required")
	}
	groupID := strings.TrimSpace(opts.GroupID)
	if groupID == "" {
		groupID = defaultArchiveGroupID
	}
	retentionDays := opts.RetentionDays
	if retentionDays <= 0 {
		retentionDays = defaultArchiveRetentionDays
	}
	batchSize := opts.BatchSize
	if batchSize <= 0 {
		batchSize = defaultArchiveBatchSize
	}
	batchBytes := opts.BatchBytes
	if batchBytes <= 0 {
		batchBytes = defaultArchiveBatchBytes
	}
	pollTimeout := opts.PollTimeout
	if pollTimeout <= 0 {
		pollTimeout = defaultArchivePollTimeout
	}
	retryBackoff := opts.RetryBackoff
	if retryBackoff <= 0 {
		retryBackoff = defaultArchiveRetryBackoff
	}
	legalHold := opts.LegalHoldFunc
	if legalHold == nil {
		legalHold = func(context.Context, string) (bool, error) { return false, nil }
	}
	nowFn := opts.NowFn
	if nowFn == nil {
		nowFn = func() time.Time { return time.Now().UTC() }
	}

	return &Archiver{
		consumer:        opts.Consumer,
		store:           opts.Store,
		topic:           strings.TrimSpace(opts.Topic),
		groupID:         groupID,
		retentionDays:   retentionDays,
		batchSize:       batchSize,
		batchBytes:      batchBytes,
		pollTimeout:     pollTimeout,
		retryBackoff:    retryBackoff,
		legalHoldFunc:   legalHold,
		nowFn:           nowFn,
		skipVerifyOnRun: opts.SkipVerifyOnRun,
	}, nil
}

func (a *Archiver) Verify(ctx context.Context) error {
	if err := a.store.VerifyObjectLock(ctx); err != nil {
		return fmt.Errorf("audit: verify object lock: %w", err)
	}
	return nil
}

// Run archives WAL records until ctx is cancelled. Transient poll or S3
// failures are retried without committing the failed archive records.
func (a *Archiver) Run(ctx context.Context) error {
	if !a.skipVerifyOnRun {
		if err := a.Verify(ctx); err != nil {
			return err
		}
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, err := a.RunOnce(ctx); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			metrics.IncArchiveErrors()
			slog.WarnContext(ctx, "audit: archive retrying", "error", err)
			if err := sleepContext(ctx, a.retryBackoff); err != nil {
				return err
			}
		}
	}
}

// RunOnce polls one WAL batch and archives whatever records are
// available. It is exposed for tests and for supervisors that prefer to
// own retry policy.
func (a *Archiver) RunOnce(ctx context.Context) (int, error) {
	records, err := a.consumer.PollBatch(ctx, a.topic, a.groupID, a.batchSize, a.pollTimeout)
	if err != nil {
		return 0, fmt.Errorf("audit: poll WAL: %w", err)
	}
	if len(records) == 0 {
		return 0, nil
	}
	return a.archiveAndCommit(ctx, records)
}

// partitionKey identifies a WAL partition for the lag gauge.
type partitionKey struct {
	topic     string
	partition int32
}

func (a *Archiver) partitionKeyFor(rec wal.Record) partitionKey {
	topic := rec.Position.Topic
	if topic == "" {
		topic = a.topic
	}
	return partitionKey{topic: topic, partition: rec.Position.Partition}
}

// archiveAndCommit archives the polled records and publishes per-partition
// lag. Lag = records polled from a partition but not yet durably written
// to S3 Object Lock + committed. A successful round-trip drives lag to 0
// for that partition; a put/commit failure leaves the unarchived tail as
// lag, which is the operator alert signal in ADR-015.
func (a *Archiver) archiveAndCommit(ctx context.Context, records []wal.Record) (int, error) {
	pending := map[partitionKey]int{}
	for _, rec := range records {
		pending[a.partitionKeyFor(rec)]++
	}
	for pk, n := range pending {
		metrics.SetArchiveLag(pk.topic, strconv.FormatInt(int64(pk.partition), 10), float64(n))
	}

	total := 0
	for _, batch := range a.archiveBatches(records) {
		obj, err := a.buildObject(ctx, batch)
		if err != nil {
			return total, err
		}
		if err := a.store.PutLockedObject(ctx, obj); err != nil {
			return total, err
		}
		for _, rec := range batch {
			if err := a.consumer.Commit(ctx, a.groupID, rec); err != nil {
				return total, fmt.Errorf("audit: commit archive offset: %w", err)
			}
			pk := a.partitionKeyFor(rec)
			pending[pk]--
			metrics.SetArchiveLag(pk.topic, strconv.FormatInt(int64(pk.partition), 10), float64(pending[pk]))
			total++
		}
	}
	metrics.AddArchiveWrites(total)
	return total, nil
}

func (a *Archiver) archiveBatches(records []wal.Record) [][]wal.Record {
	sorted := append([]wal.Record(nil), records...)
	sort.SliceStable(sorted, func(i, j int) bool {
		li, lj := sorted[i].Position, sorted[j].Position
		if li.Topic != lj.Topic {
			return li.Topic < lj.Topic
		}
		if li.Partition != lj.Partition {
			return li.Partition < lj.Partition
		}
		return li.Offset < lj.Offset
	})

	var out [][]wal.Record
	var cur []wal.Record
	var curBytes int
	var curTopic string
	var curPartition int32
	flush := func() {
		if len(cur) == 0 {
			return
		}
		out = append(out, cur)
		cur = nil
		curBytes = 0
	}
	for _, rec := range sorted {
		size := recordArchiveSize(rec)
		samePartition := len(cur) > 0 && rec.Position.Topic == curTopic && rec.Position.Partition == curPartition
		tooLarge := len(cur) > 0 && curBytes+size > a.batchBytes
		if len(cur) > 0 && (!samePartition || tooLarge) {
			flush()
		}
		if len(cur) == 0 {
			curTopic = rec.Position.Topic
			curPartition = rec.Position.Partition
		}
		cur = append(cur, rec)
		curBytes += size
	}
	flush()
	return out
}

func (a *Archiver) buildObject(ctx context.Context, records []wal.Record) (ArchiveObject, error) {
	if len(records) == 0 {
		return ArchiveObject{}, errors.New("audit: archive batch is empty")
	}
	first := records[0].Position
	last := records[len(records)-1].Position
	if first.Topic == "" {
		first.Topic = a.topic
	}
	if last.Topic == "" {
		last.Topic = first.Topic
	}

	var raw bytes.Buffer
	writer := bufio.NewWriter(&raw)
	tenantIDs := map[string]struct{}{}
	for _, rec := range records {
		if err := writeArchiveLine(writer, rec); err != nil {
			return ArchiveObject{}, err
		}
		for _, tenantID := range tenantIDsForRecord(rec) {
			tenantIDs[tenantID] = struct{}{}
		}
	}
	if err := writer.Flush(); err != nil {
		return ArchiveObject{}, fmt.Errorf("audit: flush jsonl: %w", err)
	}

	var compressed bytes.Buffer
	gz := gzip.NewWriter(&compressed)
	if _, err := gz.Write(raw.Bytes()); err != nil {
		return ArchiveObject{}, fmt.Errorf("audit: gzip archive: %w", err)
	}
	if err := gz.Close(); err != nil {
		return ArchiveObject{}, fmt.Errorf("audit: close gzip archive: %w", err)
	}
	body := compressed.Bytes()
	sum := sha256.Sum256(body)

	legalHold, err := a.legalHold(ctx, tenantIDs)
	if err != nil {
		return ArchiveObject{}, err
	}
	createdAt := a.nowFn().UTC()
	retainUntil := createdAt.Add(time.Duration(a.retentionDays) * 24 * time.Hour)
	tenantList := sortedKeys(tenantIDs)
	metadata := map[string]string{
		"entdb-format":        archiveFormatVersion,
		"entdb-topic":         first.Topic,
		"entdb-partition":     strconv.FormatInt(int64(first.Partition), 10),
		"entdb-start-offset":  strconv.FormatInt(first.Offset, 10),
		"entdb-end-offset":    strconv.FormatInt(last.Offset, 10),
		"entdb-record-count":  strconv.Itoa(len(records)),
		"entdb-sha256":        hex.EncodeToString(sum[:]),
		"entdb-created-at":    createdAt.Format(time.RFC3339Nano),
		"entdb-retain-until":  retainUntil.Format(time.RFC3339Nano),
		"entdb-legal-hold":    strconv.FormatBool(legalHold),
		"entdb-tenant-count":  strconv.Itoa(len(tenantList)),
		"entdb-tenant-sample": boundedMetadataList(tenantList, 512),
	}
	return ArchiveObject{
		Key:             fmt.Sprintf("wal/%s/%d/%d-%d.jsonl.gz", first.Topic, first.Partition, first.Offset, last.Offset),
		Body:            body,
		ContentType:     "application/jsonl",
		ContentEncoding: "gzip",
		Metadata:        metadata,
		RetainUntil:     retainUntil,
		LegalHold:       legalHold,
	}, nil
}

func (a *Archiver) legalHold(ctx context.Context, tenantIDs map[string]struct{}) (bool, error) {
	for _, tenantID := range sortedKeys(tenantIDs) {
		held, err := a.legalHoldFunc(ctx, tenantID)
		if err != nil {
			return false, fmt.Errorf("audit: legal hold lookup tenant %q: %w", tenantID, err)
		}
		if held {
			return true, nil
		}
	}
	return false, nil
}

type archiveLine struct {
	Topic       string            `json:"topic"`
	Partition   int32             `json:"partition"`
	Offset      int64             `json:"offset"`
	TimestampMs int64             `json:"timestamp_ms"`
	Key         string            `json:"key"`
	Headers     map[string]string `json:"headers,omitempty"`
	Value       json.RawMessage   `json:"value,omitempty"`
	ValueBase64 string            `json:"value_base64,omitempty"`
}

func writeArchiveLine(w *bufio.Writer, rec wal.Record) error {
	line := archiveLine{
		Topic:       rec.Position.Topic,
		Partition:   rec.Position.Partition,
		Offset:      rec.Position.Offset,
		TimestampMs: rec.Position.TimestampMs,
		Key:         rec.Key,
		Headers:     encodeHeaders(rec.Headers),
	}
	if json.Valid(rec.Value) {
		line.Value = append(json.RawMessage(nil), rec.Value...)
	} else {
		line.ValueBase64 = base64.StdEncoding.EncodeToString(rec.Value)
	}
	encoded, err := json.Marshal(line)
	if err != nil {
		return fmt.Errorf("audit: encode archive line: %w", err)
	}
	if _, err := w.Write(encoded); err != nil {
		return fmt.Errorf("audit: write archive line: %w", err)
	}
	if err := w.WriteByte('\n'); err != nil {
		return fmt.Errorf("audit: write archive newline: %w", err)
	}
	return nil
}

func tenantIDsFromEventValue(value []byte) []string {
	ev, err := wal.DecodeEvent(value)
	if err != nil || ev.TenantID == "" {
		return nil
	}
	return []string{ev.TenantID}
}

func tenantIDsForRecord(rec wal.Record) []string {
	if ids := tenantIDsFromEventValue(rec.Value); len(ids) > 0 {
		return ids
	}
	if rec.Key == "" {
		return nil
	}
	return []string{rec.Key}
}

func encodeHeaders(headers map[string][]byte) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	out := make(map[string]string, len(headers))
	for k, v := range headers {
		out[k] = base64.StdEncoding.EncodeToString(v)
	}
	return out
}

func sortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func boundedMetadataList(values []string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	var b strings.Builder
	for _, value := range values {
		if value == "" {
			continue
		}
		nextLen := len(value)
		if b.Len() > 0 {
			nextLen++
		}
		if b.Len()+nextLen > maxLen {
			break
		}
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(value)
	}
	return b.String()
}

func recordArchiveSize(rec wal.Record) int {
	size := len(rec.Key) + len(rec.Value) + 128
	for k, v := range rec.Headers {
		size += len(k) + len(v)
	}
	return size
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
