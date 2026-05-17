package errs

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// withCapturedSlog swaps slog.Default for a buffer-backed text handler
// and restores it on cleanup.
func withCapturedSlog(t *testing.T) *bytes.Buffer {
	t.Helper()
	orig := slog.Default()
	t.Cleanup(func() { slog.SetDefault(orig) })
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))
	return &buf
}

func TestInternal_ReturnsGenericInternalAndLogsDetail(t *testing.T) {
	buf := withCapturedSlog(t)

	underlying := errors.New("sqlite: no such table: tenant_secrets")
	err := Internal(context.Background(), "get user", underlying)

	if status.Code(err) != codes.Internal {
		t.Fatalf("code: got %v, want Internal", status.Code(err))
	}
	st, _ := status.FromError(err)
	if st.Message() != genericInternalMessage {
		t.Fatalf("client message: got %q, want %q", st.Message(), genericInternalMessage)
	}
	// errors.Is must still classify it as the Internal sentinel so
	// handler-side assertions keep working.
	if !errors.Is(err, ErrInternal) {
		t.Fatalf("errors.Is(Internal(...), ErrInternal) = false")
	}

	for _, leak := range []string{"sqlite", "tenant_secrets", "no such table"} {
		if strings.Contains(st.Message(), leak) {
			t.Fatalf("client message %q leaked %q", st.Message(), leak)
		}
	}

	logged := buf.String()
	if !strings.Contains(logged, "get user") {
		t.Fatalf("log missing op label; got: %s", logged)
	}
	if !strings.Contains(logged, "tenant_secrets") {
		t.Fatalf("log missing underlying detail; got: %s", logged)
	}
}

func TestInternalNoCtx_SameContract(t *testing.T) {
	buf := withCapturedSlog(t)

	err := InternalNoCtx("parse node payload", errors.New("driver crash 0xdeadbeef"))
	if status.Code(err) != codes.Internal {
		t.Fatalf("code: got %v, want Internal", status.Code(err))
	}
	st, _ := status.FromError(err)
	if st.Message() != genericInternalMessage {
		t.Fatalf("message: got %q, want %q", st.Message(), genericInternalMessage)
	}
	if strings.Contains(st.Message(), "0xdeadbeef") {
		t.Fatalf("client message leaked detail: %q", st.Message())
	}
	if !strings.Contains(buf.String(), "0xdeadbeef") {
		t.Fatalf("log missing underlying detail; got: %s", buf.String())
	}
}

func TestInternal_NilUnderlyingStillSanitizes(t *testing.T) {
	withCapturedSlog(t)
	err := Internal(context.Background(), "op", nil)
	if err == nil {
		t.Fatalf("Internal(nil) returned nil; must always yield a non-nil status")
	}
	if status.Code(err) != codes.Internal {
		t.Fatalf("code: got %v, want Internal", status.Code(err))
	}
	st, _ := status.FromError(err)
	if st.Message() != genericInternalMessage {
		t.Fatalf("message: got %q, want %q", st.Message(), genericInternalMessage)
	}
}
