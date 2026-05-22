// SEC-5 / issue #136: internal store/DB/driver errors must NOT leak to
// the client, but MUST be logged server-side.
//
// Before the errs.Internal chokepoint, handlers wrapped the underlying
// error with %v into a client-visible codes.Internal status, e.g.
//
//	errs.Errorf(codes.Internal, "get user: %v", err)
//
// which shipped strings like `globalstore: get user "alice": sql:
// database is closed` back to the caller -- enough to fingerprint the
// schema/driver and confirm record existence.
//
// This test drives a real handler (CreateUser) into a real driver
// failure by closing the backing globalstore, then asserts:
//
//   - the gRPC status code stays codes.Internal (SDK behavior unchanged),
//   - the client-visible message is the fixed generic string and
//     contains NONE of the SQLite/driver/internal substrings,
//   - the full underlying detail IS captured by the server-side slog
//     logger.

package api_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

// syncBuffer is a goroutine-safe io.Writer so the slog handler can be
// read back after the handler call without a data race (slog writes may
// happen on a different goroutine in other handlers; CreateUser is
// synchronous but we stay defensive).
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// captureSlog redirects slog.Default to a buffer for the duration of the
// test and restores the original logger on cleanup.
func captureSlog(t *testing.T) *syncBuffer {
	t.Helper()
	orig := slog.Default()
	t.Cleanup(func() { slog.SetDefault(orig) })
	sb := &syncBuffer{}
	slog.SetDefault(slog.New(slog.NewTextHandler(sb, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))
	return sb
}

// leakSubstrings are fragments of the underlying driver/store error that
// MUST NOT appear in any client-visible message.
var leakSubstrings = []string{
	"sql:", // database/sql sentinel text
	"database is closed",
	"sqlite",       // driver name / table-name fingerprinting
	"globalstore:", // internal package context
	"get user",     // internal op label (logged, never sent)
}

// TestCreateUser_InternalErrorIsSanitized is the SEC-5 regression pin.
func TestCreateUser_InternalErrorIsSanitized(t *testing.T) {
	logBuf := captureSlog(t)

	gs, err := globalstore.New(globalstore.Options{DataDir: t.TempDir(), WALMode: true})
	if err != nil {
		t.Fatalf("globalstore.New: %v", err)
	}
	// Close the store so the very next driver call inside the handler
	// (GetUser) fails with a real `sql: database is closed` error.
	if cerr := gs.Close(); cerr != nil {
		t.Fatalf("gs.Close: %v", cerr)
	}

	srv := api.New(api.WithGlobalStore(gs))

	// admin:root passes the privilege gate (no interceptor on ctx, so
	// the claimed actor is authoritative), so the handler proceeds to
	// the GetUser existence check and trips the closed-DB error.
	_, herr := srv.CreateUser(context.Background(), &pb.CreateUserRequest{
		Actor:  "admin:root",
		UserId: "alice",
		Email:  "alice@example.com",
		Name:   "Alice",
	})
	if herr == nil {
		t.Fatalf("CreateUser: expected an internal error, got nil")
	}

	// 1. Code is still Internal — observability/retry contract unchanged.
	if got := errs.Code(herr); got != codes.Internal {
		t.Fatalf("code: got %v, want Internal (err=%v)", got, herr)
	}

	// 2. The client-visible message is the fixed generic string and
	//    leaks none of the underlying detail.
	st, _ := status.FromError(herr)
	clientMsg := st.Message()
	if clientMsg != "internal error" {
		t.Errorf("client message: got %q, want %q", clientMsg, "internal error")
	}
	lowMsg := strings.ToLower(clientMsg)
	for _, frag := range leakSubstrings {
		if strings.Contains(lowMsg, strings.ToLower(frag)) {
			t.Errorf("client message %q leaks internal substring %q", clientMsg, frag)
		}
	}

	// 3. The full detail WAS logged server-side. The slog record must
	//    carry both the op label and the raw driver text so an operator
	//    can still diagnose the failure.
	logged := logBuf.String()
	if logged == "" {
		t.Fatalf("expected a server-side slog record, got empty buffer")
	}
	if !strings.Contains(logged, "internal error sanitized for client") {
		t.Errorf("slog record missing sanitizer marker; got: %s", logged)
	}
	if !strings.Contains(logged, "get user") {
		t.Errorf("slog record missing op label %q; got: %s", "get user", logged)
	}
	if !strings.Contains(logged, "database is closed") {
		t.Errorf("slog record missing underlying driver detail; got: %s", logged)
	}
}

// TestErrsInternal_Unit is a focused unit test of the chokepoint itself:
// it returns codes.Internal with the fixed message and logs the detail,
// independent of any handler wiring.
func TestErrsInternal_Unit(t *testing.T) {
	logBuf := captureSlog(t)

	underlying := status.Error(codes.Unknown, "raw sqlite: no such table: secret_nodes")
	got := errs.Internal(context.Background(), "probe op", underlying)

	if code := errs.Code(got); code != codes.Internal {
		t.Fatalf("code: got %v, want Internal", code)
	}
	st, _ := status.FromError(got)
	if st.Message() != "internal error" {
		t.Fatalf("message: got %q, want %q", st.Message(), "internal error")
	}
	if strings.Contains(st.Message(), "secret_nodes") || strings.Contains(st.Message(), "sqlite") {
		t.Fatalf("client message leaked underlying detail: %q", st.Message())
	}
	logged := logBuf.String()
	if !strings.Contains(logged, "probe op") || !strings.Contains(logged, "secret_nodes") {
		t.Fatalf("slog record missing op label or detail; got: %s", logged)
	}

	// InternalNoCtx behaves identically without a context.
	got2 := errs.InternalNoCtx("noctx op", underlying)
	if errs.Code(got2) != codes.Internal {
		t.Fatalf("InternalNoCtx code: got %v, want Internal", errs.Code(got2))
	}
	st2, _ := status.FromError(got2)
	if st2.Message() != "internal error" {
		t.Fatalf("InternalNoCtx message: got %q, want %q", st2.Message(), "internal error")
	}
}
