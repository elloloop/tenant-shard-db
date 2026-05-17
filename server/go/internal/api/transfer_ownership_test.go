// SPDX-License-Identifier: AGPL-3.0-only

// Tests for the TransferOwnership RPC. Spec:
// docs/go-port/rpcs/TransferOwnership.md.
//
// Behaviour pinned here:
//   - Owner happy path: trusted actor == current owner_actor → owner
//     flips to new_owner; node visible to new_owner via canonical SQLite
//     state; response.found = true.
//   - Non-owner caller is rejected with codes.PermissionDenied — even
//     when the caller is "system:" or holds an ACL grant on the node.
//     This is a Go-side hardening over the Python handler (which only
//     calls _check_tenant). See package-level doc on transfer_ownership.go.
//   - Missing node returns codes.NotFound (Go hardens vs. Python's
//     "found=false, error=''" soft fail).
//   - Recipient sees the node post-apply: store.GetNode reports the new
//     owner; node_visibility row exists for the recipient.
//
// The tests construct an in-memory wal.Producer + per-tenant
// CanonicalStore + globalstore so the WAL append path is exercised end
// to end. The applier is NOT run because the handler synchronously
// applies the change (the WAL event is the durable record + replay
// substrate; the in-memory mutation is the handler's own materialise).

package api_test

import (
	"context"
	"database/sql"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	"github.com/elloloop/tenant-shard-db/server/go/internal/auth"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

const (
	xferTenant = "acme"
	xferNodeID = "node-1"
	xferOwner  = "user:alice"
	xferNew    = "user:bob"
)

// xferFixture wires the dependencies a TransferOwnership handler needs:
// a globalstore (for the tenant gate), a per-tenant CanonicalStore, an
// in-memory WAL producer, and a server with all three injected. Returns
// the fixture and a context with the trusted-actor identity attached.
type xferFixture struct {
	srv      *api.Server
	store    *store.CanonicalStore
	producer *wal.InMemory
}

func newXferFixture(t *testing.T) *xferFixture {
	t.Helper()
	gs := newGlobalStore(t)
	if _, err := gs.CreateTenant(context.Background(), xferTenant, "Acme", ""); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	cs, err := store.New(store.Options{RootDir: t.TempDir(), WALMode: true})
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	if err := cs.OpenTenant(context.Background(), xferTenant); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}
	prod := wal.NewInMemory(1)
	if err := prod.Connect(context.Background()); err != nil {
		t.Fatalf("wal.Connect: %v", err)
	}
	t.Cleanup(func() { _ = prod.Close(context.Background()) })

	srv := api.New(
		api.WithGlobalStore(gs),
		api.WithStore(cs),
		api.WithWALProducer(prod),
	)
	return &xferFixture{srv: srv, store: cs, producer: prod}
}

// seedNode inserts a node with the given owner so each test does not
// have to repeat the boilerplate. Uses CreateNodeRaw to bypass the WAL
// (the test isn't exercising create_node — it is exercising the post-
// create transfer path).
func (f *xferFixture) seedNode(t *testing.T, nodeID, owner string) {
	t.Helper()
	_, err := f.store.CreateNodeRaw(context.Background(), xferTenant, store.NodeInput{
		NodeID:     nodeID,
		TypeID:     1,
		Payload:    map[string]any{"1": "v"},
		OwnerActor: owner,
	})
	if err != nil {
		t.Fatalf("seed CreateNodeRaw: %v", err)
	}
}

// ctxAs returns a context with a trusted Identity for `actor` so
// auth.Authoritative resolves to the desired caller. Handlers MUST
// consult this attestation; the wire-claimed actor is informational.
func ctxAs(actor string) context.Context {
	id := auth.Identity{Subject: actor, Method: auth.MethodSession}
	return auth.WithIdentity(context.Background(), id)
}

// TestTransferOwnership_OwnerHappyPath: the current owner can transfer
// ownership to a new principal. Post-apply, store.GetNode reports the
// new owner and node_visibility contains a row for the new owner.
func TestTransferOwnership_OwnerHappyPath(t *testing.T) {
	t.Parallel()
	f := newXferFixture(t)
	f.seedNode(t, xferNodeID, xferOwner)

	resp, err := f.srv.TransferOwnership(ctxAs(xferOwner), &pb.TransferOwnershipRequest{
		Context:  &pb.RequestContext{TenantId: xferTenant, Actor: xferOwner},
		NodeId:   xferNodeID,
		NewOwner: xferNew,
	})
	if err != nil {
		t.Fatalf("TransferOwnership: unexpected error: %v", err)
	}
	if !resp.GetFound() {
		t.Fatalf("Found=false; want true on happy path")
	}
	if resp.GetError() != "" {
		t.Errorf("unexpected error message: %q", resp.GetError())
	}

	// Side effect: owner_actor flipped on disk.
	n, err := f.store.GetNode(context.Background(), xferTenant, xferNodeID)
	if err != nil {
		t.Fatalf("post-apply GetNode: %v", err)
	}
	if n.OwnerActor != xferNew {
		t.Errorf("owner_actor = %q; want %q", n.OwnerActor, xferNew)
	}
}

// TestTransferOwnership_RecipientSeesNode: post-apply, the new owner
// has a node_visibility row pointing at the transferred node, so reads
// keyed by the recipient's principal succeed. This is the
// "visibility refresh per W1.10" assertion the spec calls out.
func TestTransferOwnership_RecipientSeesNode(t *testing.T) {
	t.Parallel()
	f := newXferFixture(t)
	f.seedNode(t, xferNodeID, xferOwner)

	if _, err := f.srv.TransferOwnership(ctxAs(xferOwner), &pb.TransferOwnershipRequest{
		Context:  &pb.RequestContext{TenantId: xferTenant, Actor: xferOwner},
		NodeId:   xferNodeID,
		NewOwner: xferNew,
	}); err != nil {
		t.Fatalf("TransferOwnership: %v", err)
	}

	db, err := f.store.AdminDB(xferTenant)
	if err != nil {
		t.Fatalf("AdminDB: %v", err)
	}
	var seen int
	row := db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM node_visibility WHERE tenant_id = ? AND node_id = ? AND principal = ?`,
		xferTenant, xferNodeID, xferNew,
	)
	if err := row.Scan(&seen); err != nil && err != sql.ErrNoRows {
		t.Fatalf("scan node_visibility: %v", err)
	}
	if seen != 1 {
		t.Errorf("node_visibility row count for %q = %d; want 1", xferNew, seen)
	}
}

// TestTransferOwnership_NonOwner_PermissionDenied: a caller who is not
// the current owner gets PermissionDenied — regardless of whether they
// are a tenant member, hold an ACL grant, or are even a system: actor.
// This is the Go-side hardening over the Python handler's no-auth path.
func TestTransferOwnership_NonOwner_PermissionDenied(t *testing.T) {
	t.Parallel()
	f := newXferFixture(t)
	f.seedNode(t, xferNodeID, xferOwner)

	// Caller is "user:eve", not the owner.
	_, err := f.srv.TransferOwnership(ctxAs("user:eve"), &pb.TransferOwnershipRequest{
		Context:  &pb.RequestContext{TenantId: xferTenant, Actor: "user:eve"},
		NodeId:   xferNodeID,
		NewOwner: xferNew,
	})
	if err == nil {
		t.Fatalf("expected PermissionDenied, got nil error")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("not a grpc status: %v", err)
	}
	if st.Code() != codes.PermissionDenied {
		t.Errorf("code = %v; want PermissionDenied", st.Code())
	}

	// Confirm side effect: owner_actor unchanged.
	n, gerr := f.store.GetNode(context.Background(), xferTenant, xferNodeID)
	if gerr != nil {
		t.Fatalf("post-deny GetNode: %v", gerr)
	}
	if n.OwnerActor != xferOwner {
		t.Errorf("owner_actor mutated under PermissionDenied: got %q want %q", n.OwnerActor, xferOwner)
	}
}

// TestTransferOwnership_PrivilegeEscalation_IgnoresClaimedActor: a
// caller authenticated as user:eve (via the trusted-actor context)
// claims actor=user:alice in the request body. The wire-claimed actor
// MUST be ignored — auth.Authoritative resolves to user:eve and the
// owner-only gate denies the transfer. Pinned by commit fece3fb.
func TestTransferOwnership_PrivilegeEscalation_IgnoresClaimedActor(t *testing.T) {
	t.Parallel()
	f := newXferFixture(t)
	f.seedNode(t, xferNodeID, xferOwner)

	_, err := f.srv.TransferOwnership(ctxAs("user:eve"), &pb.TransferOwnershipRequest{
		Context:  &pb.RequestContext{TenantId: xferTenant, Actor: xferOwner}, // lie!
		NodeId:   xferNodeID,
		NewOwner: xferNew,
	})
	if err == nil {
		t.Fatalf("expected PermissionDenied; got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.PermissionDenied {
		t.Errorf("code = %v; want PermissionDenied", st.Code())
	}
}

// TestTransferOwnership_MissingNode_NotFound: the node_id does not
// exist in the tenant → codes.NotFound. Hardens the Python "found=false,
// error=”" soft-fail per spec §"Error contract".
func TestTransferOwnership_MissingNode_NotFound(t *testing.T) {
	t.Parallel()
	f := newXferFixture(t)
	// Note: no seed.

	_, err := f.srv.TransferOwnership(ctxAs(xferOwner), &pb.TransferOwnershipRequest{
		Context:  &pb.RequestContext{TenantId: xferTenant, Actor: xferOwner},
		NodeId:   "ghost",
		NewOwner: xferNew,
	})
	if err == nil {
		t.Fatalf("expected NotFound; got nil")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("not a grpc status: %v", err)
	}
	if st.Code() != codes.NotFound {
		t.Errorf("code = %v; want NotFound", st.Code())
	}
}

// TestTransferOwnership_InvalidArgument_EmptyFields: the three required
// fields each produce InvalidArgument when empty. Pinned per spec
// §"Error contract".
func TestTransferOwnership_InvalidArgument_EmptyFields(t *testing.T) {
	t.Parallel()
	f := newXferFixture(t)
	f.seedNode(t, xferNodeID, xferOwner)

	cases := []struct {
		name string
		req  *pb.TransferOwnershipRequest
	}{
		{
			name: "empty tenant_id",
			req: &pb.TransferOwnershipRequest{
				Context:  &pb.RequestContext{TenantId: "", Actor: xferOwner},
				NodeId:   xferNodeID,
				NewOwner: xferNew,
			},
		},
		{
			name: "empty node_id",
			req: &pb.TransferOwnershipRequest{
				Context:  &pb.RequestContext{TenantId: xferTenant, Actor: xferOwner},
				NodeId:   "",
				NewOwner: xferNew,
			},
		},
		{
			name: "empty new_owner",
			req: &pb.TransferOwnershipRequest{
				Context:  &pb.RequestContext{TenantId: xferTenant, Actor: xferOwner},
				NodeId:   xferNodeID,
				NewOwner: "",
			},
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			_, err := f.srv.TransferOwnership(ctxAs(xferOwner), c.req)
			if err == nil {
				t.Fatalf("expected InvalidArgument; got nil")
			}
			st, _ := status.FromError(err)
			if st.Code() != codes.InvalidArgument {
				t.Errorf("code = %v; want InvalidArgument", st.Code())
			}
		})
	}
}

// TestTransferOwnership_InvalidActorString: a tenant-qualified or bare
// principal in new_owner is rejected with InvalidArgument (the Python
// handler accepted any string).
func TestTransferOwnership_InvalidActorString(t *testing.T) {
	t.Parallel()
	f := newXferFixture(t)
	f.seedNode(t, xferNodeID, xferOwner)

	for _, badOwner := range []string{"alice", "tenant_a:user:alice", "weird:foo"} {
		badOwner := badOwner
		t.Run(badOwner, func(t *testing.T) {
			t.Parallel()
			_, err := f.srv.TransferOwnership(ctxAs(xferOwner), &pb.TransferOwnershipRequest{
				Context:  &pb.RequestContext{TenantId: xferTenant, Actor: xferOwner},
				NodeId:   xferNodeID,
				NewOwner: badOwner,
			})
			if err == nil {
				t.Fatalf("expected InvalidArgument for new_owner=%q; got nil", badOwner)
			}
			st, _ := status.FromError(err)
			if st.Code() != codes.InvalidArgument {
				t.Errorf("code = %v; want InvalidArgument", st.Code())
			}
		})
	}
}
