// SPDX-License-Identifier: AGPL-3.0-only

package auth

import (
	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
)

// unauthenticatedf wraps a formatted message in errs.ErrUnauthenticated.
// All credential-validation failures (bad JWT, unknown API key, expired
// session, etc.) funnel through this so the interceptor returns a
// consistent codes.Unauthenticated.
//
// Bad-JWT / unknown-API-key / expired-session paths all funnel
// through this wrapper.
func unauthenticatedf(format string, a ...any) error {
	return errs.Errorf(codes.Unauthenticated, format, a...)
}

// permissionDeniedf wraps a formatted message in errs.ErrPermission. Used
// when credentials validate but the caller lacks the privilege to
// perform the requested action -- this package emits PERMISSION_DENIED
// only for the "valid creds, insufficient scope" path; tenant/ACL checks
// live in their own packages.
func permissionDeniedf(format string, a ...any) error {
	return errs.Errorf(codes.PermissionDenied, format, a...)
}
