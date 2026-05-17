// SPDX-License-Identifier: MIT
package entdb

import (
	"context"
	"math/rand"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Retry tuning constants. These mirror the Python SDK
// (sdk/python/entdb_sdk/_grpc_client.py) so both clients behave
// identically under an outage.
const (
	// retryBaseDelay is the base of the exponential backoff:
	// the unjittered delay for attempt n is base * 2**n.
	retryBaseDelay = 100 * time.Millisecond

	// retryMaxDelay caps a single backoff sleep so a high
	// maxRetries doesn't produce multi-minute waits between
	// attempts.
	retryMaxDelay = 5 * time.Second

	// defaultRetryWallClockBudget bounds the *total* wall-clock
	// time the SDK will spend retrying a single call (including
	// backoff sleeps). Once the elapsed time since the first
	// attempt would exceed this, the last error is returned even if
	// attempts remain. This stops a slow-failing endpoint from
	// blocking a caller far past their own deadline. Override via
	// [WithRetryBudget].
	defaultRetryWallClockBudget = 30 * time.Second
)

// readMethodAllowlist is the set of RPCs that are safe to retry on
// codes.DeadlineExceeded. Every entry is a read-only / side-effect-free
// RPC: re-issuing it after a timeout cannot duplicate a write. The
// short method name (the segment after the final '/' in the gRPC
// path) is used as the key.
//
// DEADLINE_EXCEEDED is deliberately NOT retried for mutating RPCs
// (ExecuteAtomic, ShareNode, the admin/GDPR surface, ...): a timeout
// means the deadline elapsed while the request was in flight, so the
// write may already have committed server-side. Retrying would risk a
// duplicate mutation. UNAVAILABLE is still retried for every method —
// it is a connection-level failure where the request almost certainly
// never reached the server.
var readMethodAllowlist = map[string]struct{}{
	"GetConnectedNodes": {},
	"GetEdgesFrom":      {},
	"GetEdgesTo":        {},
	"GetMailbox":        {},
	"GetNode":           {},
	"GetNodeByKey":      {},
	"GetNodes":          {},
	"GetReceiptStatus":  {},
	"GetSchema":         {},
	"GetTenant":         {},
	"GetTenantMembers":  {},
	"GetTenantQuota":    {},
	"GetUser":           {},
	"GetUserTenants":    {},
	"Health":            {},
	"ListMailboxUsers":  {},
	"ListSharedWithMe":  {},
	"ListTenants":       {},
	"ListUsers":         {},
	"QueryNodes":        {},
	"SearchMailbox":     {},
	"SearchNodes":       {},
	"WaitForOffset":     {},
	"ExportUserData":    {},
}

// shortMethod returns the trailing RPC name from a full gRPC method
// path (e.g. "/entdb.v1.EntDBService/GetNode" -> "GetNode").
func shortMethod(fullMethod string) string {
	if i := strings.LastIndex(fullMethod, "/"); i >= 0 {
		return fullMethod[i+1:]
	}
	return fullMethod
}

// isRetryable decides whether a failed RPC may be retried.
//
//   - codes.Unavailable: always retryable (the request likely never
//     reached the server).
//   - codes.DeadlineExceeded: retryable only when the method is in
//     the read allowlist (retrying a write after a timeout could
//     duplicate a committed mutation).
//
// Any other status code is terminal.
func isRetryable(err error, shortName string) bool {
	st, ok := status.FromError(err)
	if !ok {
		return false
	}
	switch st.Code() {
	case codes.Unavailable:
		return true
	case codes.DeadlineExceeded:
		_, allowed := readMethodAllowlist[shortName]
		return allowed
	default:
		return false
	}
}

// jitterSource is the minimal slice of *rand.Rand the backoff needs.
// Tests inject a deterministic implementation; production uses a
// mutex-guarded *rand.Rand seeded from the clock.
type jitterSource interface {
	// Float64 returns a pseudo-random number in [0.0, 1.0).
	Float64() float64
}

// lockedRand is the default jitterSource. math/rand's top-level
// functions are already goroutine-safe, but a client may issue
// concurrent RPCs through one transport, so we guard an explicit
// source to keep the jitter reproducible under -race when a seed is
// pinned in tests.
type lockedRand struct {
	mu sync.Mutex
	r  *rand.Rand
}

func newLockedRand() *lockedRand {
	return &lockedRand{r: rand.New(rand.NewSource(time.Now().UnixNano()))}
}

func (l *lockedRand) Float64() float64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.r.Float64()
}

// fullJitterBackoff returns the sleep duration before the given
// attempt (0-indexed). It implements the "full jitter" strategy from
// AWS's exponential-backoff guidance: pick a uniformly random delay
// in [0, min(cap, base*2**attempt)). Full jitter de-correlates the
// retry schedules of many clients recovering from the same outage,
// avoiding a thundering herd.
func fullJitterBackoff(attempt int, src jitterSource) time.Duration {
	ceiling := retryBaseDelay << uint(attempt) // base * 2**attempt
	if ceiling <= 0 || ceiling > retryMaxDelay {
		ceiling = retryMaxDelay
	}
	return time.Duration(src.Float64() * float64(ceiling))
}

// retryInterceptor returns a unary client interceptor that retries
// transient failures with full-jitter exponential backoff, bounded by
// both maxRetries and a wall-clock budget.
//
// It is installed *outside* the redirect interceptor so each retry
// attempt still gets the full redirect-follow behaviour. When
// maxRetries <= 0 the interceptor is a no-op pass-through (callers can
// also just skip installing it). A budget <= 0 falls back to
// [defaultRetryWallClockBudget].
func retryInterceptor(maxRetries int, budget time.Duration, src jitterSource) grpc.UnaryClientInterceptor {
	if src == nil {
		src = newLockedRand()
	}
	if budget <= 0 {
		budget = defaultRetryWallClockBudget
	}
	return func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		short := shortMethod(method)
		start := time.Now()
		var err error
		for attempt := 0; ; attempt++ {
			err = invoker(ctx, method, req, reply, cc, opts...)
			if err == nil {
				return nil
			}
			if attempt >= maxRetries || !isRetryable(err, short) {
				return err
			}

			sleep := fullJitterBackoff(attempt, src)

			// Wall-clock budget: stop if the next sleep would push
			// total elapsed time past the budget. Returning the
			// last error here (rather than sleeping anyway) keeps
			// the SDK from blocking a caller well past their own
			// deadline on a persistently slow endpoint.
			if time.Since(start)+sleep > budget {
				return err
			}

			// Respect a caller-supplied context deadline: never
			// sleep past it.
			select {
			case <-ctx.Done():
				return err
			case <-time.After(sleep):
			}
		}
	}
}
