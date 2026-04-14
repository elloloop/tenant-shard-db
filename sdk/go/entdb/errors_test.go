package entdb

import (
	"errors"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestNewRateLimitError_Fields(t *testing.T) {
	err := NewRateLimitError("monthly quota exceeded", 123456)
	if err.Message != "monthly quota exceeded" {
		t.Fatalf("Message = %q, want %q", err.Message, "monthly quota exceeded")
	}
	if err.Code != "RATE_LIMITED" {
		t.Fatalf("Code = %q, want %q", err.Code, "RATE_LIMITED")
	}
	if err.RetryAfterMs != 123456 {
		t.Fatalf("RetryAfterMs = %d, want 123456", err.RetryAfterMs)
	}
	if got, ok := err.Details["retry_after_ms"].(int64); !ok || got != 123456 {
		t.Fatalf("Details[retry_after_ms] = %v, want 123456", err.Details["retry_after_ms"])
	}
}

func TestRateLimitError_Error_WithRetry(t *testing.T) {
	err := NewRateLimitError("tenant RPS bucket empty", 2500)
	msg := err.Error()
	if !strings.Contains(msg, "RATE_LIMITED") {
		t.Errorf("Error() = %q missing RATE_LIMITED code", msg)
	}
	if !strings.Contains(msg, "tenant RPS bucket empty") {
		t.Errorf("Error() = %q missing original message", msg)
	}
	if !strings.Contains(msg, "2500") {
		t.Errorf("Error() = %q missing retry window", msg)
	}
}

func TestRateLimitError_Error_WithoutRetry(t *testing.T) {
	err := NewRateLimitError("just throttled", 0)
	msg := err.Error()
	if strings.Contains(msg, "retry after") {
		t.Errorf("Error() = %q should not mention retry when RetryAfterMs==0", msg)
	}
}

func TestRateLimitError_Error_EmptyMessage(t *testing.T) {
	err := &RateLimitError{
		EntDBError: EntDBError{Code: "RATE_LIMITED"},
	}
	msg := err.Error()
	if !strings.Contains(msg, "rate limit exceeded") {
		t.Errorf("Error() = %q, expected a default phrase", msg)
	}
}

func TestRateLimitError_ImplementsErrorInterface(t *testing.T) {
	var err error = NewRateLimitError("x", 1)
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	// errors.As should recover the typed error.
	var rle *RateLimitError
	if !errors.As(err, &rle) {
		t.Fatalf("errors.As failed to extract *RateLimitError")
	}
	if rle.RetryAfterMs != 1 {
		t.Errorf("RetryAfterMs=%d, want 1", rle.RetryAfterMs)
	}
}

func TestParseRateLimitFromStatus_ResourceExhausted(t *testing.T) {
	st := status.New(codes.ResourceExhausted, "monthly write quota exceeded")
	trailer := metadata.MD{"retry-after": []string{"42"}}
	rle := parseRateLimitFromStatus(st.Err(), trailer)
	if rle == nil {
		t.Fatalf("expected RateLimitError, got nil")
	}
	if rle.RetryAfterMs != 42*1000 {
		t.Errorf("RetryAfterMs = %d, want %d", rle.RetryAfterMs, 42*1000)
	}
	if !strings.Contains(rle.Message, "monthly write quota exceeded") {
		t.Errorf("Message = %q", rle.Message)
	}
}

func TestParseRateLimitFromStatus_NoTrailer(t *testing.T) {
	st := status.New(codes.ResourceExhausted, "throttled")
	rle := parseRateLimitFromStatus(st.Err(), metadata.MD{})
	if rle == nil {
		t.Fatalf("expected RateLimitError, got nil")
	}
	if rle.RetryAfterMs != 0 {
		t.Errorf("RetryAfterMs = %d, want 0 when no trailer", rle.RetryAfterMs)
	}
}

func TestParseRateLimitFromStatus_OtherCode(t *testing.T) {
	st := status.New(codes.NotFound, "nope")
	rle := parseRateLimitFromStatus(st.Err(), metadata.MD{})
	if rle != nil {
		t.Fatalf("expected nil for non-ResourceExhausted code, got %v", rle)
	}
}

func TestParseRateLimitFromStatus_NilError(t *testing.T) {
	if rle := parseRateLimitFromStatus(nil, metadata.MD{}); rle != nil {
		t.Fatalf("expected nil for nil error, got %v", rle)
	}
}

func TestParseRateLimitFromStatus_MalformedRetryAfter(t *testing.T) {
	st := status.New(codes.ResourceExhausted, "throttled")
	trailer := metadata.MD{"retry-after": []string{"not-a-number"}}
	rle := parseRateLimitFromStatus(st.Err(), trailer)
	if rle == nil {
		t.Fatalf("expected RateLimitError, got nil")
	}
	if rle.RetryAfterMs != 0 {
		t.Errorf("RetryAfterMs should be 0 for malformed trailer, got %d", rle.RetryAfterMs)
	}
}

// ── UniqueConstraintError (2026-04-14 SDK v0.3 decision) ───────────

func TestNewUniqueConstraintError_Fields(t *testing.T) {
	uce := NewUniqueConstraintError("tenant-1", 101, 1, "alice@example.com")
	if uce.TenantID != "tenant-1" {
		t.Errorf("TenantID = %q, want tenant-1", uce.TenantID)
	}
	if uce.TypeID != 101 {
		t.Errorf("TypeID = %d, want 101", uce.TypeID)
	}
	if uce.FieldID != 1 {
		t.Errorf("FieldID = %d, want 1", uce.FieldID)
	}
	if v, ok := uce.Value.(string); !ok || v != "alice@example.com" {
		t.Errorf("Value = %v", uce.Value)
	}
	if uce.Code != "UNIQUE_CONSTRAINT" {
		t.Errorf("Code = %q", uce.Code)
	}
	if got, ok := uce.Details["field_id"].(int32); !ok || got != 1 {
		t.Errorf("Details[field_id] = %v", uce.Details["field_id"])
	}
}

func TestUniqueConstraintError_Error(t *testing.T) {
	uce := NewUniqueConstraintError("tenant-1", 101, 1, "alice@example.com")
	msg := uce.Error()
	if !strings.Contains(msg, "UNIQUE_CONSTRAINT") {
		t.Errorf("Error() = %q missing UNIQUE_CONSTRAINT", msg)
	}
	if !strings.Contains(msg, "alice@example.com") {
		t.Errorf("Error() = %q missing value", msg)
	}
}

func TestUniqueConstraintError_ErrorEmptyMessage(t *testing.T) {
	uce := &UniqueConstraintError{
		EntDBError: EntDBError{Code: "UNIQUE_CONSTRAINT"},
		TenantID:   "t1",
		TypeID:     7,
		FieldID:    2,
		Value:      "XYZ",
	}
	msg := uce.Error()
	if !strings.Contains(msg, "XYZ") || !strings.Contains(msg, "field_id=2") {
		t.Errorf("Error() = %q", msg)
	}
}

func TestParseUniqueConstraintFromStatus_AlreadyExists(t *testing.T) {
	st := status.New(codes.AlreadyExists, "unique key already exists: email=alice@example.com")
	uce := parseUniqueConstraintFromStatus(st.Err(), "tenant-1")
	if uce == nil {
		t.Fatalf("expected UniqueConstraintError, got nil")
	}
	if uce.TenantID != "tenant-1" {
		t.Errorf("TenantID = %q", uce.TenantID)
	}
	if !strings.Contains(uce.Message, "email=alice@example.com") {
		t.Errorf("Message = %q", uce.Message)
	}
}

func TestParseUniqueConstraintFromStatus_OtherCode(t *testing.T) {
	st := status.New(codes.NotFound, "nope")
	uce := parseUniqueConstraintFromStatus(st.Err(), "tenant-1")
	if uce != nil {
		t.Errorf("expected nil for non-AlreadyExists code, got %v", uce)
	}
}

func TestParseUniqueConstraintFromStatus_NilError(t *testing.T) {
	if uce := parseUniqueConstraintFromStatus(nil, "tenant-1"); uce != nil {
		t.Fatalf("expected nil for nil error, got %v", uce)
	}
}

func TestUniqueConstraintError_ImplementsErrorInterface(t *testing.T) {
	var err error = NewUniqueConstraintError("t", 1, 3, "x")
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	var uce *UniqueConstraintError
	if !errors.As(err, &uce) {
		t.Fatalf("errors.As failed to extract *UniqueConstraintError")
	}
	if uce.FieldID != 3 {
		t.Errorf("FieldID=%d, want 3", uce.FieldID)
	}
}
