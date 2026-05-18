// SPDX-License-Identifier: AGPL-3.0-only

package audit

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

func TestS3ObjectLockStoreVerifyRequiresComplianceMode(t *testing.T) {
	ctx := context.Background()
	client := &fakeS3Client{
		objectLock: &types.ObjectLockConfiguration{
			ObjectLockEnabled: types.ObjectLockEnabledEnabled,
			Rule: &types.ObjectLockRule{
				DefaultRetention: &types.DefaultRetention{
					Mode: types.ObjectLockRetentionModeGovernance,
					Days: aws.Int32(30),
				},
			},
		},
	}
	store := NewS3ObjectLockStore(client, "audit-bucket", "")
	if err := store.VerifyObjectLock(ctx); !errors.Is(err, ErrObjectLockNotCompliance) {
		t.Fatalf("VerifyObjectLock error=%v, want ErrObjectLockNotCompliance", err)
	}

	client.objectLock.Rule.DefaultRetention.Mode = types.ObjectLockRetentionModeCompliance
	if err := store.VerifyObjectLock(ctx); err != nil {
		t.Fatalf("VerifyObjectLock compliance: %v", err)
	}
}

func TestS3ObjectLockStorePutLockedObjectSetsRetentionAndLegalHold(t *testing.T) {
	ctx := context.Background()
	client := &fakeS3Client{}
	store := NewS3ObjectLockStore(client, "audit-bucket", "kms-key")
	retainUntil := time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)
	err := store.PutLockedObject(ctx, ArchiveObject{
		Key:             "wal/entdb-wal/0/1-2.jsonl.gz",
		Body:            []byte("body"),
		ContentType:     "application/jsonl",
		ContentEncoding: "gzip",
		Metadata:        map[string]string{"entdb-format": archiveFormatVersion},
		RetainUntil:     retainUntil,
		LegalHold:       true,
	})
	if err != nil {
		t.Fatalf("PutLockedObject: %v", err)
	}
	in := client.put
	if in == nil {
		t.Fatal("PutObject was not called")
	}
	if aws.ToString(in.Bucket) != "audit-bucket" || aws.ToString(in.Key) != "wal/entdb-wal/0/1-2.jsonl.gz" {
		t.Fatalf("unexpected destination bucket/key: %s %s", aws.ToString(in.Bucket), aws.ToString(in.Key))
	}
	if in.ObjectLockMode != types.ObjectLockModeCompliance {
		t.Fatalf("ObjectLockMode=%s", in.ObjectLockMode)
	}
	if aws.ToString(in.IfNoneMatch) != "*" {
		t.Fatalf("IfNoneMatch=%q, want *", aws.ToString(in.IfNoneMatch))
	}
	if in.ObjectLockLegalHoldStatus != types.ObjectLockLegalHoldStatusOn {
		t.Fatalf("ObjectLockLegalHoldStatus=%s", in.ObjectLockLegalHoldStatus)
	}
	if in.ObjectLockRetainUntilDate == nil || !in.ObjectLockRetainUntilDate.Equal(retainUntil) {
		t.Fatalf("RetainUntil=%v", in.ObjectLockRetainUntilDate)
	}
	if in.ServerSideEncryption != types.ServerSideEncryptionAwsKms || aws.ToString(in.SSEKMSKeyId) != "kms-key" {
		t.Fatalf("SSE-KMS not set: %s %s", in.ServerSideEncryption, aws.ToString(in.SSEKMSKeyId))
	}
	body, err := io.ReadAll(in.Body)
	if err != nil {
		t.Fatalf("ReadAll body: %v", err)
	}
	if string(body) != "body" {
		t.Fatalf("body=%q", string(body))
	}
}

func TestS3ObjectLockStorePutLockedObjectTreatsExistingKeyAsIdempotentSuccess(t *testing.T) {
	ctx := context.Background()
	client := &fakeS3Client{
		putErr: &smithy.GenericAPIError{Code: "PreconditionFailed", Message: "object already exists"},
	}
	store := NewS3ObjectLockStore(client, "audit-bucket", "")
	err := store.PutLockedObject(ctx, ArchiveObject{
		Key:         "wal/entdb-wal/0/1-2.jsonl.gz",
		Body:        []byte("body"),
		RetainUntil: time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("PutLockedObject existing key: %v", err)
	}
}

type fakeS3Client struct {
	objectLock *types.ObjectLockConfiguration
	put        *s3.PutObjectInput
	putErr     error
}

func (f *fakeS3Client) GetObjectLockConfiguration(context.Context, *s3.GetObjectLockConfigurationInput, ...func(*s3.Options)) (*s3.GetObjectLockConfigurationOutput, error) {
	return &s3.GetObjectLockConfigurationOutput{ObjectLockConfiguration: f.objectLock}, nil
}

func (f *fakeS3Client) PutObject(_ context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	f.put = in
	if f.putErr != nil {
		return nil, f.putErr
	}
	return &s3.PutObjectOutput{}, nil
}

// The plain fakeS3Client only exercises the archive-write path; the
// legal-hold-lift sweep is covered by liftFakeS3 in
// legalhold_lift_test.go. These stubs just satisfy the S3API interface.
func (f *fakeS3Client) ListObjectsV2(context.Context, *s3.ListObjectsV2Input, ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	return &s3.ListObjectsV2Output{}, nil
}

func (f *fakeS3Client) GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeS3Client) GetObjectLegalHold(context.Context, *s3.GetObjectLegalHoldInput, ...func(*s3.Options)) (*s3.GetObjectLegalHoldOutput, error) {
	return &s3.GetObjectLegalHoldOutput{}, nil
}

func (f *fakeS3Client) PutObjectLegalHold(context.Context, *s3.PutObjectLegalHoldInput, ...func(*s3.Options)) (*s3.PutObjectLegalHoldOutput, error) {
	return &s3.PutObjectLegalHoldOutput{}, nil
}
