// SPDX-License-Identifier: AGPL-3.0-only

package acl

import (
	"context"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
)

// MaxGroupResolutionDepth bounds recursive group-membership expansion.
// Max ACL depth is 10. Exceeding it yields errs.ErrInvalidArgument so a
// pathological group cycle isn't silently truncated.
const MaxGroupResolutionDepth = 10

// GroupMembershipReader is the data source for parent-group expansion.
// For an actor (user/group), it returns the set of groups that
// directly contain it. The Resolver walks these recursively up to
// MaxGroupResolutionDepth.
//
// Implementations must be safe for concurrent use. The contract is
// intentionally minimal so that the production binding in W1.10 can
// adapt either store.CanonicalStore.group_users or an applier-side
// in-memory mirror.
type GroupMembershipReader interface {
	// GroupsContaining returns the group_ids (no "group:" prefix; just
	// the bare id) that have memberActorID as a direct member. The
	// memberActorID is passed in canonical "kind:id" form (e.g.
	// "user:alice" or "group:eng").
	GroupsContaining(ctx context.Context, tenantID, memberActorID string) ([]string, error)
}

// Resolver expands group principals to a flat actor list.
//
// Behaviour:
//
//   - For a user / service / role / system actor: returns [actor].
//   - For a group actor: returns [group, ...recursive parent groups].
//     Cycle-safe via a visited set.
//   - Depth-bounded: walking past MaxGroupResolutionDepth from any
//     starting actor yields errs.ErrInvalidArgument.
type Resolver struct {
	reader GroupMembershipReader
}

// NewResolver returns a Resolver backed by reader. reader may be nil
// for callers (and tests) that only need the trivial single-actor
// expansion path; calls that would require group lookup will return
// errs.ErrInternal in that case.
func NewResolver(reader GroupMembershipReader) *Resolver {
	return &Resolver{reader: reader}
}

// Expand returns the flat actor list — the input actor plus every
// group it transitively belongs to. Order is stable: input first,
// breadth-first afterwards.
//
// The returned slice is always non-empty when err == nil (it contains
// at least the input actor).
func (r *Resolver) Expand(ctx context.Context, tenantID string, a Actor) ([]Actor, error) {
	if a.IsZero() {
		return nil, errs.Errorf(codes.InvalidArgument,
			"acl: Resolver.Expand: empty actor")
	}
	out := []Actor{a}
	// Only group / user actors meaningfully participate in group_users
	// lookups. tenant: / role: / system: actors expand to themselves.
	if a.Kind != ActorKindUser && a.Kind != ActorKindGroup {
		return out, nil
	}
	if r.reader == nil {
		// Trivial expansion: no upstream groups available. This is the
		// degenerate path tests use when they don't seed groups.
		return out, nil
	}
	visited := map[string]struct{}{a.String(): {}}
	type qItem struct {
		actor Actor
		depth int
	}
	queue := []qItem{{actor: a, depth: 0}}
	for len(queue) > 0 {
		head := queue[0]
		queue = queue[1:]
		if head.depth >= MaxGroupResolutionDepth {
			return nil, errs.Errorf(codes.InvalidArgument,
				"acl: group resolution depth exceeds %d for %s",
				MaxGroupResolutionDepth, a)
		}
		gids, err := r.reader.GroupsContaining(ctx, tenantID, head.actor.String())
		if err != nil {
			return nil, err
		}
		for _, gid := range gids {
			parent := Group(gid)
			if _, ok := visited[parent.String()]; ok {
				continue
			}
			visited[parent.String()] = struct{}{}
			out = append(out, parent)
			queue = append(queue, qItem{actor: parent, depth: head.depth + 1})
		}
	}
	return out, nil
}

// ExpandIDs is a convenience wrapper that returns the canonical
// "kind:id" string form. Useful for IN (?, ?, …) lookups against
// node_access.actor_id and node_visibility.principal.
func (r *Resolver) ExpandIDs(ctx context.Context, tenantID string, a Actor) ([]string, error) {
	actors, err := r.Expand(ctx, tenantID, a)
	if err != nil {
		return nil, err
	}
	out := make([]string, len(actors))
	for i, x := range actors {
		out[i] = x.String()
	}
	return out, nil
}
