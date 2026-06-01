// SPDX-License-Identifier: AGPL-3.0-only

package api

import (
	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
)

// authorizeMailboxScope enforces the USER_MAILBOX privacy boundary (#639).
//
// A USER_MAILBOX node is private to exactly one user. The mailbox-scoped read
// RPCs — GetNode / GetNodes / QueryNodes / SearchNodes — take target_user off
// the wire and confine the read to that user's mailbox. But target_user is
// caller-supplied, so without this gate any tenant member could read another
// user's private mailbox simply by naming them. Membership in the tenant is
// NOT sufficient: the caller must BE that user, or be an admin/system actor.
//
// Rules:
//   - empty target_user — no mailbox scope; the ordinary tenant/ACL gates apply.
//   - admin / system actor — allowed (operator / service access).
//   - a user reading their OWN mailbox (trusted.ID() == target_user) — allowed.
//   - anyone else (incl. an ordinary member naming a different user) — denied.
//
// The denial depends only on the caller-vs-target relationship, never on
// whether the target's node exists, so it is not an existence oracle.
func authorizeMailboxScope(trusted auth.Actor, targetUser string) error {
	if targetUser == "" {
		return nil
	}
	if trusted.IsAdmin() || trusted.IsSystem() {
		return nil
	}
	if trusted.IsUser() && trusted.ID() == targetUser {
		return nil
	}
	return errs.Errorf(codes.PermissionDenied,
		"mailbox scope: caller may only read their own mailbox")
}
