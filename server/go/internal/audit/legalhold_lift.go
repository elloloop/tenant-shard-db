// SPDX-License-Identifier: AGPL-3.0-only

// Legal-hold lift on already-archived S3 objects (EPIC #511 Gap 1).
//
// When a tenant is placed under legal hold the archiver escalates every
// *subsequent* archive write to ObjectLockLegalHoldStatus=ON
// (archiver.go:buildObject -> s3_lock.go:PutLockedObject). Releasing the
// hold (SetLegalHold OFF) previously had no effect on objects already
// written to S3 — they stayed legal-hold-locked forever, requiring
// manual S3 surgery and blocking the GDPR-erasure-after-release story.
//
// This file closes that gap. On an explicit release the api layer
// triggers LiftLegalHoldForReleasedTenant as a background sweep (the RPC
// does not block on it). GDPR erasure NEVER triggers a lift: legal hold
// deliberately supersedes erasure, so only an explicit release lifts the
// hold and only THEN may queued erasure proceed (the DeleteUser
// precondition gate in internal/api/delete_user.go is unchanged).
//
// Object-key reality (important): archive objects are keyed
// "wal/<topic>/<partition>/<start>-<end>.jsonl.gz" — per WAL partition,
// NOT per tenant. A single archive object can carry records for many
// tenants and gets legal-hold=ON if ANY of those tenants was held when
// it was written. There is therefore no per-tenant key prefix to scan;
// the sweep walks the whole archive prefix ("wal/") and, for each object
// whose legal hold is ON, decodes the object body to recover its
// authoritative tenant set (the bounded entdb-tenant-sample metadata may
// truncate and is not relied on) and clears the hold only once NONE of
// the tenants the object carries is still under legal hold. That rule is
// what makes the sweep:
//
//   - idempotent          : an already-OFF / never-held object is skipped;
//   - partial-completion-safe : a re-run finishes an interrupted sweep
//     (each object is decided independently from current S3 + globalstore
//     state, no progress cursor to corrupt);
//   - safe for shared objects : another tenant still on hold keeps the
//     object's hold ON;
//   - a no-op for a tenant with no archived objects.
//
// COMPLIANCE-mode retention is NEVER touched — only PutObjectLegalHold is
// issued, which flips the legal-hold flag and leaves
// ObjectLockMode=COMPLIANCE + ObjectLockRetainUntilDate intact (ADR-015:
// retention is immutable; legal hold is liftable on release).

package audit

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithyhttp "github.com/aws/smithy-go/transport/http"

	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

// withContentMD5 registers the smithy Content-MD5 build-step middleware
// for a single S3 call. PutObjectLegalHold is a legacy
// Content-MD5-required operation; MinIO and other non-AWS S3 endpoints
// reject it with HTTP 400 MissingContentMD5 unless the header is
// present. AWS SDK Go v2's post-Jan-2025 default computes a CRC32
// trailer instead, so we add the legacy MD5 explicitly. Harmless on
// real AWS S3 (it accepts a correct Content-MD5).
func withContentMD5(o *s3.Options) {
	o.APIOptions = append(o.APIOptions, smithyhttp.AddContentChecksumMiddleware)
}

// archivePrefix is the single root prefix every archive object is
// written under (see archiver.buildObject key format). The lift sweep
// paginates this prefix; there is no narrower per-tenant prefix because
// the key is per WAL partition.
const archivePrefix = "wal/"

// TenantHeldFunc reports whether tenantID is *still* under legal hold.
// The sweep consults it per tenant referenced by a held object so a
// shared object keeps its hold while any co-tenant remains held. Backed
// by globalstore.IsLegalHoldSet in production.
type TenantHeldFunc func(ctx context.Context, tenantID string) (bool, error)

// LiftSummary reports one LiftLegalHoldForReleasedTenant pass. It is
// safe to log: counts only, no object bodies.
type LiftSummary struct {
	// Scanned is the number of archive objects enumerated.
	Scanned int
	// HeldScanned is the number of objects observed with legal hold ON.
	HeldScanned int
	// Lifted is the number of objects whose legal hold this pass cleared.
	Lifted int
	// SkippedStillHeld is the number of held objects left ON because
	// some tenant they carry is still under legal hold.
	SkippedStillHeld int
}

// LiftLegalHoldForReleasedTenant clears the S3 Object Lock legal hold on
// archive objects that no longer carry any still-held tenant, after
// tenantID's hold has been released. See the file header for the design
// (trigger, idempotency, retention untouched, shared-object safety).
//
// The sweep is resumable: it derives every decision from live S3 +
// stillHeld state, so a crashed / re-invoked run simply finishes the
// remaining objects. An object whose body cannot be decoded is treated
// conservatively (left ON) so a corrupt object never silently drops a
// hold; that surfaces as an error to the caller for visibility.
func (s *S3ObjectLockStore) LiftLegalHoldForReleasedTenant(
	ctx context.Context,
	tenantID string,
	stillHeld TenantHeldFunc,
) (LiftSummary, error) {
	var summary LiftSummary
	if s == nil || s.client == nil {
		return summary, errors.New("audit: s3 client is required")
	}
	if s.bucket == "" {
		return summary, errors.New("audit: s3 bucket is required")
	}
	if tenantID == "" {
		return summary, errors.New("audit: released tenant id is required")
	}
	if stillHeld == nil {
		return summary, errors.New("audit: still-held lookup is required")
	}

	var continuationToken *string
	for {
		if err := ctx.Err(); err != nil {
			return summary, err
		}
		page, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(s.bucket),
			Prefix:            aws.String(archivePrefix),
			ContinuationToken: continuationToken,
		})
		if err != nil {
			return summary, fmt.Errorf("audit: list archive objects: %w", err)
		}
		for _, obj := range page.Contents {
			key := aws.ToString(obj.Key)
			if key == "" {
				continue
			}
			summary.Scanned++
			if err := s.liftOneObject(ctx, key, tenantID, stillHeld, &summary); err != nil {
				return summary, err
			}
		}
		if page.IsTruncated == nil || !*page.IsTruncated || aws.ToString(page.NextContinuationToken) == "" {
			break
		}
		continuationToken = page.NextContinuationToken
	}
	return summary, nil
}

// liftOneObject applies the lift rule to a single archive object. It is
// a no-op when the object's legal hold is already OFF (idempotency) and
// leaves the hold ON when any tenant the object carries is still held
// (shared-object safety). Only PutObjectLegalHold(OFF) is ever issued —
// COMPLIANCE retention is never touched.
func (s *S3ObjectLockStore) liftOneObject(
	ctx context.Context,
	key, releasedTenant string,
	stillHeld TenantHeldFunc,
	summary *LiftSummary,
) error {
	lhOut, err := s.client.GetObjectLegalHold(ctx, &s3.GetObjectLegalHoldInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("audit: get legal hold %q: %w", key, err)
	}
	if lhOut == nil || lhOut.LegalHold == nil ||
		lhOut.LegalHold.Status != types.ObjectLockLegalHoldStatusOn {
		// Already OFF / never held — nothing to do (idempotent).
		return nil
	}
	summary.HeldScanned++

	tenants, err := s.objectTenants(ctx, key)
	if err != nil {
		return fmt.Errorf("audit: decode held object %q: %w", key, err)
	}
	// Conservative: if the released tenant is not even in this object,
	// some *other* held tenant is keeping it ON — leave it.
	carriesReleased := false
	for _, t := range tenants {
		if t == releasedTenant {
			carriesReleased = true
			break
		}
	}
	if !carriesReleased {
		summary.SkippedStillHeld++
		return nil
	}
	for _, t := range tenants {
		held, herr := stillHeld(ctx, t)
		if herr != nil {
			return fmt.Errorf("audit: still-held lookup tenant %q: %w", t, herr)
		}
		if held {
			// A co-tenant is still under hold; the object stays ON.
			summary.SkippedStillHeld++
			return nil
		}
	}

	if _, err := s.client.PutObjectLegalHold(ctx, &s3.PutObjectLegalHoldInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		LegalHold: &types.ObjectLockLegalHold{
			Status: types.ObjectLockLegalHoldStatusOff,
		},
	}, withContentMD5); err != nil {
		return fmt.Errorf("audit: clear legal hold %q: %w", key, err)
	}
	summary.Lifted++
	return nil
}

// objectTenants downloads an archive object and returns the
// authoritative set of tenant ids it carries by decoding the gzip-JSONL
// body with the same line shape the archiver writes (archiver.go). The
// bounded entdb-tenant-sample metadata is deliberately not used here —
// it can truncate and is not authoritative.
func (s *S3ObjectLockStore) objectTenants(ctx context.Context, key string) ([]string, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("get object %q: %w", key, err)
	}
	defer func() {
		if out != nil && out.Body != nil {
			_ = out.Body.Close()
		}
	}()

	gz, err := gzip.NewReader(out.Body)
	if err != nil {
		return nil, fmt.Errorf("gunzip %q: %w", key, err)
	}
	defer func() { _ = gz.Close() }()

	seen := map[string]struct{}{}
	scanner := bufio.NewScanner(gz)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		raw := bytes.TrimSpace(scanner.Bytes())
		if len(raw) == 0 {
			continue
		}
		var line archiveLine
		if err := json.Unmarshal(raw, &line); err != nil {
			return nil, fmt.Errorf("decode archive line in %q: %w", key, err)
		}
		value := line.Value
		if len(value) == 0 && line.ValueBase64 != "" {
			// base64-encoded non-JSON value: the WAL Event is JSON, so a
			// base64 line is never an Event we can read a tenant from.
			// Fall back to the record key, which the archiver sets to
			// the tenant id for tenant-scoped writes.
			if line.Key != "" {
				seen[line.Key] = struct{}{}
			}
			continue
		}
		if ev, derr := wal.DecodeEvent(value); derr == nil && ev.TenantID != "" {
			seen[ev.TenantID] = struct{}{}
			continue
		}
		if line.Key != "" {
			seen[line.Key] = struct{}{}
		}
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("scan archive body %q: %w", key, err)
	}
	return sortedKeys(seen), nil
}
