// SPDX-License-Identifier: AGPL-3.0-only

package audit

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

// liftObject is one object in the in-memory S3 fake. RetainUntil +
// LockMode model the immutable COMPLIANCE retention; the test asserts
// they are NEVER mutated by a legal-hold lift.
type liftObject struct {
	body        []byte
	legalHoldOn bool
	lockMode    types.ObjectLockMode
	retainUntil time.Time
}

// liftFakeS3 is an in-memory S3 supporting the four lift calls plus the
// archive write path. It records every PutObjectLegalHold so a test can
// prove COMPLIANCE retention was never touched (only the legal-hold
// flag flips).
type liftFakeS3 struct {
	objects map[string]*liftObject

	listCalls       int
	getLHCalls      int
	putLHCalls      int
	pageSize        int // 0 => single page
	listErr         error
	putLHErr        error
	putLHKeyMutated []string // keys we issued a legal-hold PUT for
}

func newLiftFakeS3() *liftFakeS3 {
	return &liftFakeS3{objects: map[string]*liftObject{}}
}

func (f *liftFakeS3) seed(key string, body []byte, legalHoldOn bool) {
	f.objects[key] = &liftObject{
		body:        body,
		legalHoldOn: legalHoldOn,
		lockMode:    types.ObjectLockModeCompliance,
		retainUntil: time.Date(2033, 1, 2, 3, 4, 5, 0, time.UTC),
	}
}

func (f *liftFakeS3) GetObjectLockConfiguration(context.Context, *s3.GetObjectLockConfigurationInput, ...func(*s3.Options)) (*s3.GetObjectLockConfigurationOutput, error) {
	return &s3.GetObjectLockConfigurationOutput{ObjectLockConfiguration: &types.ObjectLockConfiguration{
		ObjectLockEnabled: types.ObjectLockEnabledEnabled,
		Rule: &types.ObjectLockRule{DefaultRetention: &types.DefaultRetention{
			Mode: types.ObjectLockRetentionModeCompliance, Days: aws.Int32(1),
		}},
	}}, nil
}

func (f *liftFakeS3) PutObject(_ context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	body, _ := io.ReadAll(in.Body)
	f.objects[aws.ToString(in.Key)] = &liftObject{
		body:        body,
		legalHoldOn: in.ObjectLockLegalHoldStatus == types.ObjectLockLegalHoldStatusOn,
		lockMode:    in.ObjectLockMode,
		retainUntil: aws.ToTime(in.ObjectLockRetainUntilDate),
	}
	return &s3.PutObjectOutput{}, nil
}

func (f *liftFakeS3) ListObjectsV2(_ context.Context, in *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	f.listCalls++
	if f.listErr != nil {
		return nil, f.listErr
	}
	keys := make([]string, 0, len(f.objects))
	for k := range f.objects {
		if strings.HasPrefix(k, aws.ToString(in.Prefix)) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	start := 0
	if in.ContinuationToken != nil {
		for i, k := range keys {
			if k == aws.ToString(in.ContinuationToken) {
				start = i
				break
			}
		}
	}
	end := len(keys)
	truncated := false
	if f.pageSize > 0 && start+f.pageSize < len(keys) {
		end = start + f.pageSize
		truncated = true
	}

	out := &s3.ListObjectsV2Output{}
	for _, k := range keys[start:end] {
		out.Contents = append(out.Contents, types.Object{Key: aws.String(k)})
	}
	if truncated {
		out.IsTruncated = aws.Bool(true)
		out.NextContinuationToken = aws.String(keys[end])
	}
	return out, nil
}

func (f *liftFakeS3) GetObject(_ context.Context, in *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	o, ok := f.objects[aws.ToString(in.Key)]
	if !ok {
		return nil, errors.New("NoSuchKey")
	}
	return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(o.body))}, nil
}

func (f *liftFakeS3) GetObjectLegalHold(_ context.Context, in *s3.GetObjectLegalHoldInput, _ ...func(*s3.Options)) (*s3.GetObjectLegalHoldOutput, error) {
	f.getLHCalls++
	o, ok := f.objects[aws.ToString(in.Key)]
	if !ok {
		return nil, errors.New("NoSuchKey")
	}
	status := types.ObjectLockLegalHoldStatusOff
	if o.legalHoldOn {
		status = types.ObjectLockLegalHoldStatusOn
	}
	return &s3.GetObjectLegalHoldOutput{LegalHold: &types.ObjectLockLegalHold{Status: status}}, nil
}

func (f *liftFakeS3) PutObjectLegalHold(_ context.Context, in *s3.PutObjectLegalHoldInput, _ ...func(*s3.Options)) (*s3.PutObjectLegalHoldOutput, error) {
	f.putLHCalls++
	if f.putLHErr != nil {
		return nil, f.putLHErr
	}
	o, ok := f.objects[aws.ToString(in.Key)]
	if !ok {
		return nil, errors.New("NoSuchKey")
	}
	// The lift MUST only flip the legal-hold flag. COMPLIANCE retention
	// stays exactly as written — assert it here and never let the fake
	// mutate it.
	o.legalHoldOn = in.LegalHold.Status == types.ObjectLockLegalHoldStatusOn
	f.putLHKeyMutated = append(f.putLHKeyMutated, aws.ToString(in.Key))
	return &s3.PutObjectLegalHoldOutput{}, nil
}

// gzJSONL builds a gzip-JSONL archive body carrying one WAL Event per
// tenant id — the exact shape archiver.buildObject writes.
func gzJSONL(t *testing.T, tenantIDs ...string) []byte {
	t.Helper()
	var raw bytes.Buffer
	for i, tid := range tenantIDs {
		ev := wal.Event{
			TenantID:       tid,
			Actor:          "actor",
			IdempotencyKey: tid + "-idem",
			TsMs:           int64(1000 + i),
			Ops:            []map[string]any{{"op": "noop"}},
		}
		val, err := ev.Encode()
		if err != nil {
			t.Fatalf("encode event: %v", err)
		}
		line := archiveLine{
			Topic:     "entdb-wal",
			Partition: 0,
			Offset:    int64(i),
			Key:       tid,
			Value:     val,
		}
		enc, err := json.Marshal(line)
		if err != nil {
			t.Fatalf("marshal line: %v", err)
		}
		raw.Write(enc)
		raw.WriteByte('\n')
	}
	var gzbuf bytes.Buffer
	gw := gzip.NewWriter(&gzbuf)
	if _, err := gw.Write(raw.Bytes()); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return gzbuf.Bytes()
}

// assertRetentionIntact fails if any object's COMPLIANCE lock mode or
// retain-until changed — the core ADR-015 invariant the lift must honour.
func assertRetentionIntact(t *testing.T, f *liftFakeS3) {
	t.Helper()
	for k, o := range f.objects {
		if o.lockMode != types.ObjectLockModeCompliance {
			t.Fatalf("object %q lock mode = %q, want COMPLIANCE (retention must be immutable)", k, o.lockMode)
		}
		if o.retainUntil.IsZero() {
			t.Fatalf("object %q retain-until was cleared (retention must be immutable)", k)
		}
	}
}

func heldNone(context.Context, string) (bool, error) { return false, nil }

func TestLiftReleasesAllArchivedObjectsForTenant(t *testing.T) {
	ctx := context.Background()
	f := newLiftFakeS3()
	f.seed("wal/entdb-wal/0/0-1.jsonl.gz", gzJSONL(t, "tenant-a"), true)
	f.seed("wal/entdb-wal/0/2-3.jsonl.gz", gzJSONL(t, "tenant-a"), true)
	f.seed("wal/entdb-wal/1/0-0.jsonl.gz", gzJSONL(t, "tenant-a"), true)

	store := NewS3ObjectLockStore(f, "audit-bucket", "")
	sum, err := store.LiftLegalHoldForReleasedTenant(ctx, "tenant-a", heldNone)
	if err != nil {
		t.Fatalf("lift: %v", err)
	}
	if sum.Scanned != 3 || sum.HeldScanned != 3 || sum.Lifted != 3 || sum.SkippedStillHeld != 0 {
		t.Fatalf("summary = %+v, want all 3 lifted", sum)
	}
	for k, o := range f.objects {
		if o.legalHoldOn {
			t.Fatalf("object %q still has legal hold ON after release", k)
		}
	}
	assertRetentionIntact(t, f)
}

func TestLiftIsIdempotentOnReRun(t *testing.T) {
	ctx := context.Background()
	f := newLiftFakeS3()
	f.seed("wal/entdb-wal/0/0-1.jsonl.gz", gzJSONL(t, "tenant-a"), true)
	f.seed("wal/entdb-wal/0/2-3.jsonl.gz", gzJSONL(t, "tenant-a"), true)

	store := NewS3ObjectLockStore(f, "audit-bucket", "")
	if _, err := store.LiftLegalHoldForReleasedTenant(ctx, "tenant-a", heldNone); err != nil {
		t.Fatalf("first lift: %v", err)
	}
	putsAfterFirst := f.putLHCalls
	if putsAfterFirst != 2 {
		t.Fatalf("first run issued %d legal-hold PUTs, want 2", putsAfterFirst)
	}

	// Re-run: every object is already OFF, so the sweep is a pure no-op
	// for the mutating call (partial-completion-safe / idempotent).
	sum, err := store.LiftLegalHoldForReleasedTenant(ctx, "tenant-a", heldNone)
	if err != nil {
		t.Fatalf("re-run lift: %v", err)
	}
	if sum.Lifted != 0 || sum.HeldScanned != 0 {
		t.Fatalf("re-run summary = %+v, want zero lifted/held", sum)
	}
	if f.putLHCalls != putsAfterFirst {
		t.Fatalf("re-run issued extra legal-hold PUTs: %d -> %d", putsAfterFirst, f.putLHCalls)
	}
	assertRetentionIntact(t, f)
}

func TestLiftIsNoOpWhenNothingHeld(t *testing.T) {
	ctx := context.Background()
	f := newLiftFakeS3()
	// Objects exist but were never legal-hold-locked.
	f.seed("wal/entdb-wal/0/0-1.jsonl.gz", gzJSONL(t, "tenant-a"), false)
	f.seed("wal/entdb-wal/0/2-3.jsonl.gz", gzJSONL(t, "tenant-b"), false)

	store := NewS3ObjectLockStore(f, "audit-bucket", "")
	sum, err := store.LiftLegalHoldForReleasedTenant(ctx, "tenant-a", heldNone)
	if err != nil {
		t.Fatalf("lift: %v", err)
	}
	if sum.HeldScanned != 0 || sum.Lifted != 0 {
		t.Fatalf("summary = %+v, want no-op", sum)
	}
	if f.putLHCalls != 0 {
		t.Fatalf("no-op sweep issued %d legal-hold PUTs, want 0", f.putLHCalls)
	}

	// And a tenant with zero archived objects: also a clean no-op.
	empty := NewS3ObjectLockStore(newLiftFakeS3(), "audit-bucket", "")
	if sum2, err := empty.LiftLegalHoldForReleasedTenant(ctx, "ghost", heldNone); err != nil || sum2.Scanned != 0 {
		t.Fatalf("empty-bucket lift: sum=%+v err=%v", sum2, err)
	}
	assertRetentionIntact(t, f)
}

func TestLiftKeepsHoldWhenCoTenantStillHeld(t *testing.T) {
	ctx := context.Background()
	f := newLiftFakeS3()
	// Shared object: tenant-a (released) + tenant-b (still held).
	f.seed("wal/entdb-wal/0/0-1.jsonl.gz", gzJSONL(t, "tenant-a", "tenant-b"), true)
	// tenant-a-only object: must be released.
	f.seed("wal/entdb-wal/0/2-3.jsonl.gz", gzJSONL(t, "tenant-a"), true)

	stillHeld := func(_ context.Context, tid string) (bool, error) {
		return tid == "tenant-b", nil
	}
	store := NewS3ObjectLockStore(f, "audit-bucket", "")
	sum, err := store.LiftLegalHoldForReleasedTenant(ctx, "tenant-a", stillHeld)
	if err != nil {
		t.Fatalf("lift: %v", err)
	}
	if sum.Lifted != 1 || sum.SkippedStillHeld != 1 {
		t.Fatalf("summary = %+v, want 1 lifted + 1 skipped", sum)
	}
	if !f.objects["wal/entdb-wal/0/0-1.jsonl.gz"].legalHoldOn {
		t.Fatal("shared object lost its hold while tenant-b is still held")
	}
	if f.objects["wal/entdb-wal/0/2-3.jsonl.gz"].legalHoldOn {
		t.Fatal("tenant-a-only object should have been released")
	}
	assertRetentionIntact(t, f)
}

func TestLiftDoesNotTouchOtherTenantsObjects(t *testing.T) {
	ctx := context.Background()
	f := newLiftFakeS3()
	// tenant-b is held and is NOT being released; its object must be
	// left strictly untouched (no legal-hold PUT against its key).
	f.seed("wal/entdb-wal/0/0-1.jsonl.gz", gzJSONL(t, "tenant-a"), true)
	f.seed("wal/entdb-wal/0/2-3.jsonl.gz", gzJSONL(t, "tenant-b"), true)

	stillHeld := func(_ context.Context, tid string) (bool, error) {
		return tid == "tenant-b", nil
	}
	store := NewS3ObjectLockStore(f, "audit-bucket", "")
	if _, err := store.LiftLegalHoldForReleasedTenant(ctx, "tenant-a", stillHeld); err != nil {
		t.Fatalf("lift: %v", err)
	}
	if !f.objects["wal/entdb-wal/0/2-3.jsonl.gz"].legalHoldOn {
		t.Fatal("tenant-b's object lost its hold (other tenant unaffected invariant)")
	}
	for _, k := range f.putLHKeyMutated {
		if k == "wal/entdb-wal/0/2-3.jsonl.gz" {
			t.Fatalf("lift issued a legal-hold PUT against another tenant's object %q", k)
		}
	}
	assertRetentionIntact(t, f)
}

func TestLiftPaginatesAcrossPages(t *testing.T) {
	ctx := context.Background()
	f := newLiftFakeS3()
	f.pageSize = 2
	keys := []string{
		"wal/entdb-wal/0/0-0.jsonl.gz",
		"wal/entdb-wal/0/1-1.jsonl.gz",
		"wal/entdb-wal/0/2-2.jsonl.gz",
		"wal/entdb-wal/0/3-3.jsonl.gz",
		"wal/entdb-wal/0/4-4.jsonl.gz",
	}
	for _, k := range keys {
		f.seed(k, gzJSONL(t, "tenant-a"), true)
	}

	store := NewS3ObjectLockStore(f, "audit-bucket", "")
	sum, err := store.LiftLegalHoldForReleasedTenant(ctx, "tenant-a", heldNone)
	if err != nil {
		t.Fatalf("lift: %v", err)
	}
	if sum.Scanned != 5 || sum.Lifted != 5 {
		t.Fatalf("summary = %+v, want 5 scanned/lifted across pages", sum)
	}
	if f.listCalls < 3 {
		t.Fatalf("listCalls = %d, want >= 3 (pagination not exercised)", f.listCalls)
	}
	assertRetentionIntact(t, f)
}

func TestLiftPropagatesListError(t *testing.T) {
	ctx := context.Background()
	f := newLiftFakeS3()
	f.listErr = errors.New("s3 throttled")
	store := NewS3ObjectLockStore(f, "audit-bucket", "")
	if _, err := store.LiftLegalHoldForReleasedTenant(ctx, "tenant-a", heldNone); err == nil {
		t.Fatal("expected list error to propagate")
	}
}
