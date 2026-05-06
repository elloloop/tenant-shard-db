package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/pb"
)

// dialOverride lets tests replace the dialer with a bufconn-backed
// one. nil in production code paths.
var dialOverride func(addr string) (*grpc.ClientConn, error)

// dial opens a gRPC connection to the configured address.
// The CLI uses insecure transport because the server is
// usually reached over a local port-forward or a sidecar;
// TLS termination is handled out-of-band.
func dial(addr string) (*grpc.ClientConn, error) {
	if dialOverride != nil {
		return dialOverride(addr)
	}
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}
	return conn, nil
}

// newClient is the one-stop helper the subcommands use.
// It returns the typed pb client, a cancel func that closes
// the connection, and a context already populated with the
// per-call timeout and the auth metadata header.
func newClient() (pb.EntDBServiceClient, context.Context, context.CancelFunc, error) {
	conn, err := dial(flags.addr)
	if err != nil {
		return nil, nil, nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), flags.timeout)
	if flags.apiKey != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+flags.apiKey)
	}
	cli := pb.NewEntDBServiceClient(conn)
	// Wrap cancel so the connection is closed when the
	// caller is done, regardless of how the call returned.
	wrapped := func() {
		cancel()
		_ = conn.Close()
	}
	return cli, ctx, wrapped, nil
}

// reqContext builds the standard RequestContext used by
// most read RPCs. It does NOT default the actor — handlers
// reject empty actors with InvalidArgument and the CLI
// surfaces that error verbatim.
func reqContext(tenantID string) *pb.RequestContext {
	return &pb.RequestContext{
		TenantId: tenantID,
		Actor:    flags.actor,
	}
}

// friendlyError turns a gRPC status into a single human line.
// Anything not a gRPC status falls through as-is. The
// transport-layer mapper in the SDK does the same job for
// SDK callers; the CLI duplicates a tiny version of it
// because it talks to the raw pb client and never enters
// the SDK's error translation path.
func friendlyError(err error) string {
	if err == nil {
		return ""
	}
	st, ok := status.FromError(err)
	if !ok {
		// Includes context.DeadlineExceeded thrown before
		// the call ever hit the wire.
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Sprintf("request timed out after %s — increase --timeout or check --addr", flags.timeout)
		}
		return err.Error()
	}
	switch st.Code() {
	case codes.Unauthenticated:
		return "authentication failed — set --api-key or $ENTDB_API_KEY"
	case codes.PermissionDenied:
		return "permission denied — actor " + quote(flags.actor) + " is not allowed: " + st.Message()
	case codes.NotFound:
		return "not found: " + st.Message()
	case codes.InvalidArgument:
		return "invalid argument: " + st.Message()
	case codes.Unavailable:
		return "server unavailable at " + flags.addr + " — " + st.Message()
	case codes.DeadlineExceeded:
		return fmt.Sprintf("request exceeded --timeout=%s", flags.timeout)
	case codes.FailedPrecondition:
		// Common case: tenant pinned to a different region.
		return "failed precondition: " + st.Message()
	default:
		return fmt.Sprintf("%s: %s", st.Code(), st.Message())
	}
}

func quote(s string) string {
	if s == "" {
		return "<unset>"
	}
	if strings.ContainsAny(s, " \t\"") {
		return fmt.Sprintf("%q", s)
	}
	return s
}

// formatTime turns a unix-millis int64 into an RFC3339 string.
// Zero is rendered as a dash so empty cells stand out in tables.
func formatTime(unixMs int64) string {
	if unixMs == 0 {
		return "-"
	}
	return time.UnixMilli(unixMs).UTC().Format(time.RFC3339)
}
