// Trailer helpers for the Go gRPC server.
//
// Spec: docs/go-port/shared/error-mapping.md "Detail trailers". The Python
// server attaches trailing metadata in two places only:
//
//   - entdb-redirect-node on UNAVAILABLE (api/grpc_server.py:387-389)
//   - retry-after on RESOURCE_EXHAUSTED (auth/quota_interceptor.py:233)
//
// Order matters: trailers MUST be set BEFORE the handler returns the status
// error, otherwise grpc-go flushes trailers along with the closing status
// and the late call is a no-op (same constraint Python documents at
// api/grpc_server.py:390).
//
// Both helpers wrap grpc.SetTrailer, which writes to the call's trailer set
// rather than mutating the response -- so the return idiom is
//
//	if err := errs.SetRedirectTrailer(ctx, ownerNode); err != nil {
//	        return nil, err
//	}
//	return nil, errs.Errorf(codes.Unavailable, "tenant pinned to %s", ownerNode)
//
// SetRedirectTrailer / SetRetryAfter return an error only if the context
// is not a server-side gRPC call context (grpc.SetTrailer returns
// "grpc: failed to fetch the stream from the context"), so handlers can
// treat the error as a bug-detector rather than a normal failure.

package errs

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// Trailer key constants. Lower-case per HTTP/2 header rules; grpc-go
// normalizes these on the wire but we keep the constants in the canonical
// form so contract tests can compare bytes directly.
const (
	// TrailerRedirectNode carries the owning node id the SDK should retry
	// against on a sharding miss. Python source:
	// api/grpc_server.py:387-389. Always paired with codes.Unavailable.
	TrailerRedirectNode = "entdb-redirect-node"

	// TrailerRetryAfter carries the integer-second delay before the SDK
	// should retry. Python source: auth/quota_interceptor.py:233. Always
	// paired with codes.ResourceExhausted from the quota interceptor.
	// Format: decimal integer seconds, no unit suffix (matches HTTP
	// Retry-After "delta-seconds" form, RFC 7231 §7.1.3).
	TrailerRetryAfter = "retry-after"
)

// SetRedirectTrailer attaches the entdb-redirect-node trailer with the
// given owner node id. Call this immediately before returning a
// codes.Unavailable error from a handler that detected a sharding mismatch.
//
// Empty ownerNode is a programming error -- the SDK redirect cache uses
// the value as a routing key, and an empty string would cause a redirect
// loop. We return an error rather than panicking so the caller can fall
// back to a non-routing failure.
func SetRedirectTrailer(ctx context.Context, ownerNode string) error {
	if ownerNode == "" {
		return fmt.Errorf("errs.SetRedirectTrailer: ownerNode must not be empty")
	}
	md := metadata.Pairs(TrailerRedirectNode, ownerNode)
	return grpc.SetTrailer(ctx, md)
}

// SetRetryAfter attaches the retry-after trailer with the given duration,
// rounded UP to the nearest second (zero rounds to 1; matches the Python
// quota interceptor's str(int(math.ceil(seconds))) shape -- the SDK uses
// the value as a sleep target, so rounding down would cause an immediate
// re-hit on the rate limiter).
//
// Negative durations are clamped to 1 second.
//
// Pair this with a codes.ResourceExhausted error.
func SetRetryAfter(ctx context.Context, d time.Duration) error {
	seconds := int64(d / time.Second)
	if d%time.Second != 0 && d > 0 {
		seconds++
	}
	if seconds < 1 {
		seconds = 1
	}
	md := metadata.Pairs(TrailerRetryAfter, fmt.Sprintf("%d", seconds))
	return grpc.SetTrailer(ctx, md)
}

// SetRetryAfterSeconds is the integer-seconds form, matching the Python
// quota interceptor's call site exactly. Provided so the auth/quota
// interceptor port can be a one-line change.
//
// seconds < 1 is clamped to 1 (see SetRetryAfter rationale).
func SetRetryAfterSeconds(ctx context.Context, seconds int) error {
	if seconds < 1 {
		seconds = 1
	}
	md := metadata.Pairs(TrailerRetryAfter, fmt.Sprintf("%d", seconds))
	return grpc.SetTrailer(ctx, md)
}
