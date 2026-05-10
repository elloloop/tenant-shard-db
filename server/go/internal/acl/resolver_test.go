// SPDX-License-Identifier: AGPL-3.0-only

package acl_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/elloloop/tenant-shard-db/server/go/internal/acl"
	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
)

// fakeGroups is a tiny in-memory GroupMembershipReader. The map key is
// the canonical "kind:id" form of the member (e.g. "user:alice"); the
// value is the list of bare group ids that contain it.
type fakeGroups struct {
	parents map[string][]string
}

func (f *fakeGroups) GroupsContaining(_ context.Context, _, member string) ([]string, error) {
	return f.parents[member], nil
}

func TestResolverUserNoGroups(t *testing.T) {
	r := acl.NewResolver(&fakeGroups{})
	out, err := r.Expand(context.Background(), "t1", acl.User("alice"))
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if len(out) != 1 || out[0] != acl.User("alice") {
		t.Fatalf("Expand returned %+v", out)
	}
}

func TestResolverNonGroupActorPassthrough(t *testing.T) {
	r := acl.NewResolver(&fakeGroups{})
	for _, a := range []acl.Actor{
		acl.Service("svc"), acl.Role("admin"), acl.Tenant("acme"), acl.System("applier"),
	} {
		out, err := r.Expand(context.Background(), "t1", a)
		if err != nil {
			t.Fatalf("Expand(%v): %v", a, err)
		}
		if len(out) != 1 || out[0] != a {
			t.Fatalf("Expand(%v) = %+v, want [%v]", a, out, a)
		}
	}
}

func TestResolverNilReaderTrivial(t *testing.T) {
	r := acl.NewResolver(nil)
	out, err := r.Expand(context.Background(), "t1", acl.User("alice"))
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("nil reader expansion = %+v, want [user:alice]", out)
	}
}

func TestResolverGroupRecursive(t *testing.T) {
	// alice ∈ eng, eng ∈ all-employees, bob ∈ eng (not used here).
	groups := &fakeGroups{parents: map[string][]string{
		"user:alice": {"eng"},
		"group:eng":  {"all-employees"},
		"user:bob":   {"eng"},
	}}
	r := acl.NewResolver(groups)
	out, err := r.Expand(context.Background(), "t1", acl.User("alice"))
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	got := actorStrings(out)
	want := map[string]struct{}{
		"user:alice":          {},
		"group:eng":           {},
		"group:all-employees": {},
	}
	if len(got) != len(want) {
		t.Fatalf("Expand alice = %v, want %v", got, want)
	}
	for _, s := range got {
		if _, ok := want[s]; !ok {
			t.Fatalf("unexpected %s in expansion %v", s, got)
		}
	}
	// Input first.
	if out[0] != acl.User("alice") {
		t.Fatalf("first element should be input actor, got %v", out[0])
	}
}

func TestResolverCycleSafe(t *testing.T) {
	// group:a ∈ b, group:b ∈ a (cycle). Should not loop.
	groups := &fakeGroups{parents: map[string][]string{
		"group:a": {"b"},
		"group:b": {"a"},
	}}
	r := acl.NewResolver(groups)
	out, err := r.Expand(context.Background(), "t1", acl.Group("a"))
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	// Should contain a and b, exactly once each.
	if len(out) != 2 {
		t.Fatalf("cycle expansion = %v, want 2 unique groups", out)
	}
}

func TestResolverDepthExceeded(t *testing.T) {
	// Build a chain longer than MaxGroupResolutionDepth (10).
	parents := map[string][]string{}
	parents["user:alice"] = []string{"g0"}
	for i := 0; i < acl.MaxGroupResolutionDepth+5; i++ {
		parents["group:g"+itoa(i)] = []string{"g" + itoa(i+1)}
	}
	r := acl.NewResolver(&fakeGroups{parents: parents})
	_, err := r.Expand(context.Background(), "t1", acl.User("alice"))
	if err == nil {
		t.Fatalf("Expand should have errored on depth")
	}
	if !errors.Is(err, errs.ErrInvalidArgument) {
		t.Fatalf("err = %v, want errs.ErrInvalidArgument", err)
	}
	if !strings.Contains(err.Error(), "depth") {
		t.Fatalf("err = %v, expected depth message", err)
	}
}

func TestResolverEmptyActor(t *testing.T) {
	r := acl.NewResolver(nil)
	_, err := r.Expand(context.Background(), "t1", acl.Actor{})
	if !errors.Is(err, errs.ErrInvalidArgument) {
		t.Fatalf("empty actor expansion err = %v, want ErrInvalidArgument", err)
	}
}

func TestResolverExpandIDs(t *testing.T) {
	groups := &fakeGroups{parents: map[string][]string{
		"user:alice": {"eng"},
	}}
	r := acl.NewResolver(groups)
	ids, err := r.ExpandIDs(context.Background(), "t1", acl.User("alice"))
	if err != nil {
		t.Fatalf("ExpandIDs: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("ExpandIDs = %v, want 2", ids)
	}
}

func actorStrings(actors []acl.Actor) []string {
	out := make([]string, len(actors))
	for i, a := range actors {
		out[i] = a.String()
	}
	return out
}

// tiny itoa to avoid importing strconv just for tests.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	var buf [20]byte
	n := len(buf)
	for i > 0 {
		n--
		buf[n] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		n--
		buf[n] = '-'
	}
	return string(buf[n:])
}
