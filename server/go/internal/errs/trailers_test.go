package errs

import (
	"context"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// trailerFunc is the per-test handler signature: takes the server-side ctx
// and returns whatever it wants. Trailers set on ctx end up on the wire.
type trailerFunc func(ctx context.Context) error

// runTrailerTest spins up an in-memory grpc.Server with a single
// no-codec method, invokes the provided handler with the server-side
// context, and returns the trailing metadata observed by the client plus
// the status returned by the call.
//
// We use the empty proto-less codec by passing nil decoders; the server
// answers a single unary call. The lifecycle keeps no goroutines after
// return.
func runTrailerTest(t *testing.T, h trailerFunc) (metadata.MD, *status.Status) {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := grpc.NewServer(grpc.ForceServerCodec(rawCodec{}))
	// Register a single bidi-less unary method via the generic API.
	desc := &grpc.ServiceDesc{
		ServiceName: "errs.test.TrailerSvc",
		HandlerType: (*any)(nil),
		Methods: []grpc.MethodDesc{
			{
				MethodName: "Call",
				Handler: func(_ any, ctx context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
					var unused []byte
					if err := dec(&unused); err != nil {
						return nil, err
					}
					if err := h(ctx); err != nil {
						return nil, err
					}
					return []byte{}, nil
				},
			},
		},
		Metadata: "errs/test.proto",
	}
	srv.RegisterService(desc, struct{}{})

	go func() { _ = srv.Serve(lis) }()
	defer srv.Stop()

	conn, err := grpc.NewClient(
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.ForceCodec(rawCodec{})),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var trailer metadata.MD
	var reply []byte
	callErr := conn.Invoke(
		ctx,
		"/errs.test.TrailerSvc/Call",
		[]byte{},
		&reply,
		grpc.Trailer(&trailer),
	)
	st, _ := status.FromError(callErr)
	return trailer, st
}

// rawCodec is a do-nothing codec that lets us send/receive raw byte slices
// without pulling in a proto type. Marshal/Unmarshal are identity on
// []byte, which is all the trailer tests need.
type rawCodec struct{}

func (rawCodec) Marshal(v any) ([]byte, error) {
	if b, ok := v.([]byte); ok {
		return b, nil
	}
	return nil, nil
}
func (rawCodec) Unmarshal(data []byte, v any) error {
	if p, ok := v.(*[]byte); ok {
		*p = data
	}
	return nil
}
func (rawCodec) Name() string { return "errs-test-raw" }

func TestSetRedirectTrailer_RoundTrip(t *testing.T) {
	trailer, st := runTrailerTest(t, func(ctx context.Context) error {
		if err := SetRedirectTrailer(ctx, "node-7"); err != nil {
			return err
		}
		return Errorf(codes.Unavailable, "tenant pinned to node-7")
	})
	if st.Code() != codes.Unavailable {
		t.Fatalf("code: %v", st.Code())
	}
	got := trailer.Get(TrailerRedirectNode)
	if len(got) != 1 || got[0] != "node-7" {
		t.Fatalf("trailer %s = %v, want [node-7]", TrailerRedirectNode, got)
	}
}

func TestSetRedirectTrailer_RejectsEmpty(t *testing.T) {
	// We can call this directly with a background ctx because the
	// validation runs before grpc.SetTrailer.
	err := SetRedirectTrailer(context.Background(), "")
	if err == nil {
		t.Fatalf("empty owner: expected error")
	}
}

func TestSetRetryAfter_RoundsUp(t *testing.T) {
	cases := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"zero clamps to 1", 0, "1"},
		{"negative clamps to 1", -3 * time.Second, "1"},
		{"sub-second rounds up", 250 * time.Millisecond, "1"},
		{"exact second", 5 * time.Second, "5"},
		{"fraction rounds up", 5500 * time.Millisecond, "6"},
		{"large", 90 * time.Second, "90"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			trailer, st := runTrailerTest(t, func(ctx context.Context) error {
				if err := SetRetryAfter(ctx, c.d); err != nil {
					return err
				}
				return Errorf(codes.ResourceExhausted, "quota")
			})
			if st.Code() != codes.ResourceExhausted {
				t.Fatalf("code: %v", st.Code())
			}
			got := trailer.Get(TrailerRetryAfter)
			if len(got) != 1 || got[0] != c.want {
				t.Fatalf("retry-after: got %v, want [%s]", got, c.want)
			}
		})
	}
}

func TestSetRetryAfterSeconds(t *testing.T) {
	trailer, st := runTrailerTest(t, func(ctx context.Context) error {
		if err := SetRetryAfterSeconds(ctx, 12); err != nil {
			return err
		}
		return Errorf(codes.ResourceExhausted, "quota")
	})
	if st.Code() != codes.ResourceExhausted {
		t.Fatalf("code: %v", st.Code())
	}
	if got := trailer.Get(TrailerRetryAfter); len(got) != 1 || got[0] != "12" {
		t.Fatalf("retry-after seconds=12: got %v", got)
	}
}

func TestSetRetryAfterSeconds_ClampsBelowOne(t *testing.T) {
	trailer, _ := runTrailerTest(t, func(ctx context.Context) error {
		_ = SetRetryAfterSeconds(ctx, 0)
		return Errorf(codes.ResourceExhausted, "quota")
	})
	if got := trailer.Get(TrailerRetryAfter); len(got) != 1 || got[0] != "1" {
		t.Fatalf("clamp: got %v", got)
	}
}

// TrailerKeysAreLowercase guards against accidental introduction of
// upper-case header names. HTTP/2 normalizes to lower case anyway, but
// contract tests compare bytes after normalization and any drift here
// would surface as a contract-test failure.
func TestTrailerKeysAreLowercase(t *testing.T) {
	for _, k := range []string{TrailerRedirectNode, TrailerRetryAfter} {
		for _, r := range k {
			if r >= 'A' && r <= 'Z' {
				t.Fatalf("trailer key %q contains uppercase", k)
			}
		}
	}
}
