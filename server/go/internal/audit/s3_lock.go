// SPDX-License-Identifier: AGPL-3.0-only

// Package audit contains durable audit-log archival helpers. The S3
// implementation writes WAL archive objects with Object Lock in
// COMPLIANCE mode so a copied WAL segment is immutable for its retention
// window.
package audit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

var ErrObjectLockNotCompliance = errors.New("audit: s3 bucket object lock is not COMPLIANCE")

// ArchiveObject is one immutable WAL archive object.
type ArchiveObject struct {
	Key             string
	Body            []byte
	ContentType     string
	ContentEncoding string
	Metadata        map[string]string
	RetainUntil     time.Time
	LegalHold       bool
}

// ObjectLockStore writes archive objects and verifies that the
// destination supports Object Lock in COMPLIANCE mode before archival
// starts.
type ObjectLockStore interface {
	VerifyObjectLock(ctx context.Context) error
	PutLockedObject(ctx context.Context, obj ArchiveObject) error
}

// S3API is the small subset of the AWS S3 client needed by this package.
type S3API interface {
	GetObjectLockConfiguration(ctx context.Context, params *s3.GetObjectLockConfigurationInput, optFns ...func(*s3.Options)) (*s3.GetObjectLockConfigurationOutput, error)
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

// S3ObjectLockStore stores WAL archive objects in a single S3 bucket.
type S3ObjectLockStore struct {
	client   S3API
	bucket   string
	kmsKeyID string
}

func NewS3ObjectLockStore(client S3API, bucket, kmsKeyID string) *S3ObjectLockStore {
	return &S3ObjectLockStore{client: client, bucket: bucket, kmsKeyID: kmsKeyID}
}

func (s *S3ObjectLockStore) VerifyObjectLock(ctx context.Context) error {
	if s == nil || s.client == nil {
		return errors.New("audit: s3 client is required")
	}
	if s.bucket == "" {
		return errors.New("audit: s3 bucket is required")
	}
	out, err := s.client.GetObjectLockConfiguration(ctx, &s3.GetObjectLockConfigurationInput{
		Bucket: aws.String(s.bucket),
	})
	if err != nil {
		return fmt.Errorf("audit: get object lock configuration: %w", err)
	}
	cfg := out.ObjectLockConfiguration
	if cfg == nil || cfg.ObjectLockEnabled != types.ObjectLockEnabledEnabled {
		return ErrObjectLockNotCompliance
	}
	if cfg.Rule == nil ||
		cfg.Rule.DefaultRetention == nil ||
		cfg.Rule.DefaultRetention.Mode != types.ObjectLockRetentionModeCompliance {
		return ErrObjectLockNotCompliance
	}
	return nil
}

func (s *S3ObjectLockStore) PutLockedObject(ctx context.Context, obj ArchiveObject) error {
	if s == nil || s.client == nil {
		return errors.New("audit: s3 client is required")
	}
	if s.bucket == "" {
		return errors.New("audit: s3 bucket is required")
	}
	if obj.Key == "" {
		return errors.New("audit: archive object key is required")
	}
	if len(obj.Body) == 0 {
		return errors.New("audit: archive object body is required")
	}
	if obj.RetainUntil.IsZero() {
		return errors.New("audit: archive retain-until time is required")
	}

	legalHold := types.ObjectLockLegalHoldStatusOff
	if obj.LegalHold {
		legalHold = types.ObjectLockLegalHoldStatusOn
	}
	input := &s3.PutObjectInput{
		Bucket:                    aws.String(s.bucket),
		Key:                       aws.String(obj.Key),
		Body:                      bytes.NewReader(obj.Body),
		ContentType:               aws.String(obj.ContentType),
		ContentEncoding:           aws.String(obj.ContentEncoding),
		IfNoneMatch:               aws.String("*"),
		Metadata:                  obj.Metadata,
		ObjectLockMode:            types.ObjectLockModeCompliance,
		ObjectLockRetainUntilDate: aws.Time(obj.RetainUntil),
		ObjectLockLegalHoldStatus: legalHold,
	}
	if s.kmsKeyID != "" {
		input.ServerSideEncryption = types.ServerSideEncryptionAwsKms
		input.SSEKMSKeyId = aws.String(s.kmsKeyID)
	}
	if _, err := s.client.PutObject(ctx, input); err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "PreconditionFailed" {
			return nil
		}
		return fmt.Errorf("audit: put locked object %q: %w", obj.Key, err)
	}
	return nil
}
