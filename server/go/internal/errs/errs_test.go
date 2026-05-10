package errs

import (
	"errors"
	"fmt"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Each row asserts (sentinel) -> code round-trip via status.FromError +
// status.Code, plus errors.Is reflexivity.
var sentinelTable = []struct {
	name string
	err  *codeError
	code codes.Code
}{
	{"InvalidArgument", ErrInvalidArgument, codes.InvalidArgument},
	{"NotFound", ErrNotFound, codes.NotFound},
	{"AlreadyExists", ErrAlreadyExists, codes.AlreadyExists},
	{"Permission", ErrPermission, codes.PermissionDenied},
	{"FailedPrecondition", ErrFailedPrecondition, codes.FailedPrecondition},
	{"ResourceExhausted", ErrResourceExhausted, codes.ResourceExhausted},
	{"Unauthenticated", ErrUnauthenticated, codes.Unauthenticated},
	{"Unavailable", ErrUnavailable, codes.Unavailable},
	{"Unimplemented", ErrUnimplemented, codes.Unimplemented},
	{"Internal", ErrInternal, codes.Internal},
	{"DeadlineExceeded", ErrDeadlineExceeded, codes.DeadlineExceeded},
}

func TestSentinelGRPCStatusRoundTrip(t *testing.T) {
	for _, row := range sentinelTable {
		t.Run(row.name, func(t *testing.T) {
			st, ok := status.FromError(row.err)
			if !ok {
				t.Fatalf("status.FromError(%v): expected ok=true", row.err)
			}
			if st.Code() != row.code {
				t.Fatalf("got code %v, want %v", st.Code(), row.code)
			}
			// Empty-message sentinel degrades to the lowercase code name.
			if st.Message() != row.code.String() {
				t.Fatalf("got msg %q, want %q", st.Message(), row.code.String())
			}
			// status.Code is the most common path callers use.
			if status.Code(row.err) != row.code {
				t.Fatalf("status.Code returned %v", status.Code(row.err))
			}
			// errors.Is reflexivity.
			if !errors.Is(row.err, row.err) {
				t.Fatalf("errors.Is(sentinel, itself) = false")
			}
		})
	}
}

func TestErrorf_FormatsMessage(t *testing.T) {
	err := Errorf(codes.PermissionDenied, "actor %q is not admin", "alice")
	if got, want := status.Code(err), codes.PermissionDenied; got != want {
		t.Fatalf("code: got %v want %v", got, want)
	}
	st, _ := status.FromError(err)
	want := `actor "alice" is not admin`
	if st.Message() != want {
		t.Fatalf("msg: got %q want %q", st.Message(), want)
	}
}

func TestErrorf_NoArgsTreatsFormatAsLiteral(t *testing.T) {
	err := Errorf(codes.InvalidArgument, "missing tenant_id")
	st, _ := status.FromError(err)
	if st.Message() != "missing tenant_id" {
		t.Fatalf("got %q", st.Message())
	}
}

func TestErrorf_EmptyMessageDegradesToCodeName(t *testing.T) {
	err := Errorf(codes.NotFound, "")
	st, _ := status.FromError(err)
	if st.Message() != codes.NotFound.String() {
		t.Fatalf("got %q want %q", st.Message(), codes.NotFound.String())
	}
}

func TestErrorf_IsSentinel(t *testing.T) {
	// errors.Is must match by code so handlers can wrap a sentinel with
	// a custom message and tests can still assert against the sentinel.
	err := Errorf(codes.NotFound, "tenant %q does not exist", "t-42")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("errors.Is(Errorf-NotFound, ErrNotFound) = false")
	}
	if errors.Is(err, ErrPermission) {
		t.Fatalf("errors.Is across distinct codes = true (should be false)")
	}
}

func TestCode_NilIsOK(t *testing.T) {
	if Code(nil) != codes.OK {
		t.Fatalf("Code(nil) = %v", Code(nil))
	}
}

func TestCode_NakedErrorIsUnknown(t *testing.T) {
	if Code(errors.New("boom")) != codes.Unknown {
		t.Fatalf("Code(plain error) = %v", Code(errors.New("boom")))
	}
}

func TestCode_StatusError(t *testing.T) {
	err := status.Error(codes.AlreadyExists, "dup")
	if Code(err) != codes.AlreadyExists {
		t.Fatalf("Code(status) = %v", Code(err))
	}
}

func TestAs_ExtractsCode(t *testing.T) {
	c, ok := As(Errorf(codes.Unimplemented, "no global store"))
	if !ok || c != codes.Unimplemented {
		t.Fatalf("As: got (%v,%v) want (Unimplemented,true)", c, ok)
	}
	if _, ok := As(errors.New("naked")); ok {
		t.Fatalf("As(plain error) returned ok=true")
	}
	if _, ok := As(nil); ok {
		t.Fatalf("As(nil) returned ok=true")
	}
}

// fakeResponse implements the SetError contract used by
// PreserveStatusOrSwallow. Real proto messages with an `error` string field
// generate this method via go-protobuf.
type fakeResponse struct {
	err string
}

func (f *fakeResponse) SetError(s string) { f.err = s }

func TestPreserveStatusOrSwallow_NilReturnsNil(t *testing.T) {
	dst := &fakeResponse{}
	if err := PreserveStatusOrSwallow(nil, dst); err != nil {
		t.Fatalf("nil err: got %v", err)
	}
	if dst.err != "" {
		t.Fatalf("dst written for nil err: %q", dst.err)
	}
}

func TestPreserveStatusOrSwallow_StatusErrorPassesThrough(t *testing.T) {
	dst := &fakeResponse{}
	in := Errorf(codes.PermissionDenied, "no")
	out := PreserveStatusOrSwallow(in, dst)
	if out != in {
		t.Fatalf("status err not passed through: got %v want %v", out, in)
	}
	if dst.err != "" {
		t.Fatalf("dst written on pass-through: %q", dst.err)
	}
}

func TestPreserveStatusOrSwallow_NakedErrorIsSwallowed(t *testing.T) {
	dst := &fakeResponse{}
	out := PreserveStatusOrSwallow(errors.New("kaboom"), dst)
	if out != nil {
		t.Fatalf("naked err not swallowed: %v", out)
	}
	if dst.err != "kaboom" {
		t.Fatalf("dst not written: %q", dst.err)
	}
}

func TestPreserveStatusOrSwallow_NilDstSilentlyDrops(t *testing.T) {
	// Documented behavior: nil dst is permitted but the message is dropped.
	out := PreserveStatusOrSwallow(errors.New("kaboom"), nil)
	if out != nil {
		t.Fatalf("nil dst should still swallow: %v", out)
	}
}

func TestPreserveStatusOrSwallow_WrappedSentinelPassesThrough(t *testing.T) {
	// fmt.Errorf with %w of a sentinel must still report the sentinel's
	// code via status.Code -- otherwise PreserveStatusOrSwallow would
	// wrongly swallow a wrapped sentinel.
	in := fmt.Errorf("admin guard: %w", ErrPermission)
	dst := &fakeResponse{}
	out := PreserveStatusOrSwallow(in, dst)
	// The status pkg unwraps via GRPCStatus on the wrapped sentinel.
	// Verify the chain works.
	if status.Code(in) != codes.PermissionDenied {
		t.Fatalf("status.Code through %%w wrap: got %v", status.Code(in))
	}
	if out != in {
		t.Fatalf("wrapped sentinel not passed through")
	}
	if dst.err != "" {
		t.Fatalf("dst written for wrapped sentinel: %q", dst.err)
	}
}

func TestFromPythonException_KnownClasses(t *testing.T) {
	cases := []struct {
		name string
		want codes.Code
	}{
		{"AccessDeniedError", codes.PermissionDenied},
		{"TenantNotFoundError", codes.NotFound},
		{"IdempotencyViolationError", codes.AlreadyExists},
		{"UniqueConstraintError", codes.AlreadyExists},
		{"CompositeUniqueConstraintError", codes.AlreadyExists},
		{"QueryFilterError", codes.InvalidArgument},
		{"CompatibilityError", codes.FailedPrecondition},
		{"RegistryFrozenError", codes.FailedPrecondition},
		{"DuplicateRegistrationError", codes.FailedPrecondition},
		{"AuthenticationError", codes.Unauthenticated},
		{"SessionError", codes.Unauthenticated},
		{"ApiKeyError", codes.Unauthenticated},
		{"AuthError", codes.Unauthenticated},
		{"CryptoShreddedError", codes.FailedPrecondition},
		{"TenantShreddedError", codes.FailedPrecondition},
		{"TenantKeyAlreadyProvisionedError", codes.AlreadyExists},
		{"WalConnectionError", codes.Unavailable},
		{"WalTimeoutError", codes.Unavailable},
		{"WalSerializationError", codes.Internal},
		{"PermissionError", codes.PermissionDenied},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := FromPythonException(c.name); got != c.want {
				t.Fatalf("FromPythonException(%q) = %v, want %v", c.name, got, c.want)
			}
		})
	}
}

func TestFromPythonException_UnknownReturnsUnknown(t *testing.T) {
	if got := FromPythonException("NonExistentError"); got != codes.Unknown {
		t.Fatalf("got %v, want Unknown", got)
	}
}

func TestFromGoError(t *testing.T) {
	if FromGoError(nil) != codes.OK {
		t.Fatalf("nil -> %v", FromGoError(nil))
	}
	if FromGoError(ErrAlreadyExists) != codes.AlreadyExists {
		t.Fatalf("sentinel -> %v", FromGoError(ErrAlreadyExists))
	}
	if FromGoError(status.Error(codes.Unavailable, "x")) != codes.Unavailable {
		t.Fatalf("status err -> %v", FromGoError(status.Error(codes.Unavailable, "x")))
	}
	if FromGoError(errors.New("naked")) != codes.Unknown {
		t.Fatalf("naked -> %v", FromGoError(errors.New("naked")))
	}
}
