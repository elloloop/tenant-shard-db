// Package testseed wires the deterministic test-only fixture the
// cross-implementation contract suite (tests/python/integration/
// test_grpc_contract.py) expects to find on a freshly-booted server.
//
// The harness contract is documented in docs/go-port/shared/test-harness.md
// (the `--seed-tenant` row). The Python in-process fixture lives at
// tests/python/integration/conftest.py:275-348 and seeds the same shape
// via direct globalstore/canonicalstore writes + a follow-up ExecuteAtomic
// call. This package reproduces that state for the Go subprocess harness.
//
// CLAUDE.md invariant #1 says all writes go through the WAL. This
// package is the documented exception: it is a test-only seed for
// the contract harness, gated by `--seed-tenant`, and never reachable
// from a production code path. The seed writes the same applied_events
// + applied_offsets rows the applier would have written had the seed
// run through ExecuteAtomic, so post-seed reads behave identically.
package testseed

import (
	"context"
	"errors"
	"fmt"

	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
	"github.com/elloloop/tenant-shard-db/server/go/internal/schema"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
	"google.golang.org/grpc/codes"
)

// Fixture constants — keep in lock-step with
// tests/python/integration/conftest.py (TENANT, ALICE, SEED_NODE_ID)
// and tests/python/integration/test_grpc_contract.py.
const (
	AliceUserID  = "alice"
	BobUserID    = "bob"
	AliceActor   = "user:alice"
	SeedNodeID   = "seeded-node"
	SeedReceipt  = "seed-1"
	SeedTopic    = "entdb-wal"
	UserTypeID   = 1
	TaskTypeID   = 2
	AssignedToID = 100
)

// RegisterContractSchema populates reg with the User/Task/AssignedTo
// types the contract suite asserts against. Mirrors
// tests/python/integration/conftest.py:_build_python_registry.
//
// Idempotent on a fresh registry; a second call against an already-
// populated registry returns ErrDuplicateRegistration.
func RegisterContractSchema(reg *schema.Registry) error {
	if reg == nil {
		return errors.New("testseed: nil registry")
	}
	user := &schema.NodeTypeDef{
		TypeID: UserTypeID,
		Name:   "User",
		Fields: []schema.FieldDef{
			{FieldID: 1, Name: "email", Kind: schema.KindString},
			{FieldID: 2, Name: "name", Kind: schema.KindString},
		},
	}
	if err := reg.RegisterNode(user); err != nil {
		return fmt.Errorf("testseed: register User: %w", err)
	}
	task := &schema.NodeTypeDef{
		TypeID: TaskTypeID,
		Name:   "Task",
		Fields: []schema.FieldDef{
			{FieldID: 1, Name: "title", Kind: schema.KindString},
			{FieldID: 2, Name: "description", Kind: schema.KindString},
		},
	}
	if err := reg.RegisterNode(task); err != nil {
		return fmt.Errorf("testseed: register Task: %w", err)
	}
	edge := &schema.EdgeTypeDef{
		EdgeID:     AssignedToID,
		Name:       "AssignedTo",
		FromTypeID: TaskTypeID,
		ToTypeID:   UserTypeID,
	}
	if err := reg.RegisterEdge(edge); err != nil {
		return fmt.Errorf("testseed: register AssignedTo: %w", err)
	}
	return nil
}

// SeedTenant applies the contract-fixture seed for tenantID. Order
// mirrors the Python conftest:
//
//  1. tenant_registry row (tenantID, "Acme Corp", region default).
//  2. user_registry rows: alice + bob.
//  3. tenant_members: alice=owner, bob=member.
//  4. canonical store: OpenTenant + seeded-node + seed-1 receipt +
//     applied_offsets row.
//
// All steps are best-effort idempotent: an already-existing row is
// treated as success so repeated harness boots against the same
// data-dir don't fail.
func SeedTenant(ctx context.Context, g *globalstore.GlobalStore, s *store.CanonicalStore, tenantID string) error {
	if g == nil {
		return errors.New("testseed: nil globalstore")
	}
	if s == nil {
		return errors.New("testseed: nil canonical store")
	}
	if tenantID == "" {
		return errors.New("testseed: empty tenantID")
	}

	if _, err := g.CreateTenant(ctx, tenantID, "Acme Corp", ""); err != nil && !isAlreadyExists(err) {
		return fmt.Errorf("testseed: create tenant: %w", err)
	}
	if _, err := g.CreateUser(ctx, AliceUserID, "alice@example.com", "Alice"); err != nil && !isAlreadyExists(err) {
		return fmt.Errorf("testseed: create user alice: %w", err)
	}
	if _, err := g.CreateUser(ctx, BobUserID, "bob@example.com", "Bob"); err != nil && !isAlreadyExists(err) {
		return fmt.Errorf("testseed: create user bob: %w", err)
	}
	if err := g.AddTenantMember(ctx, tenantID, AliceUserID, "owner"); err != nil && !isAlreadyExists(err) {
		return fmt.Errorf("testseed: add alice as owner: %w", err)
	}
	if err := g.AddTenantMember(ctx, tenantID, BobUserID, "member"); err != nil && !isAlreadyExists(err) {
		return fmt.Errorf("testseed: add bob as member: %w", err)
	}

	if err := s.OpenTenant(ctx, tenantID); err != nil {
		return fmt.Errorf("testseed: open tenant: %w", err)
	}

	// Seed node — payload is field-id-keyed (CLAUDE.md invariant #6).
	// Field 1 = email, Field 2 = name (per RegisterContractSchema).
	if _, err := s.CreateNodeRaw(ctx, tenantID, store.NodeInput{
		NodeID:     SeedNodeID,
		TypeID:     UserTypeID,
		Payload:    map[string]any{"1": "seeded@example.com", "2": "Seeded"},
		OwnerActor: AliceActor,
	}); err != nil && !isUniqueOrAlreadyExists(err) {
		return fmt.Errorf("testseed: create seed node: %w", err)
	}

	// seed-1 receipt so GetReceiptStatus(seed-1) returns APPLIED.
	if err := s.RecordIdempotency(ctx, tenantID, SeedReceipt, "entdb-wal:0:0"); err != nil && !isIdempotencyViolation(err) {
		return fmt.Errorf("testseed: record receipt seed-1: %w", err)
	}

	// applied_offsets: mark partition 0 of entdb-wal as having processed
	// offset 1 (one event — the seed node). WaitForOffset(target=0)
	// returns immediately because the in-memory map now has tenantID >= 0.
	if err := s.UpdateAppliedOffset(ctx, tenantID, SeedTopic, 0, 1); err != nil {
		return fmt.Errorf("testseed: update applied offset: %w", err)
	}

	return nil
}

func isAlreadyExists(err error) bool {
	return errs.Code(err) == codes.AlreadyExists
}

func isUniqueOrAlreadyExists(err error) bool {
	if isAlreadyExists(err) {
		return true
	}
	return errors.Is(err, store.ErrUniqueConstraint)
}

func isIdempotencyViolation(err error) bool {
	return errors.Is(err, store.ErrIdempotencyViolation)
}
