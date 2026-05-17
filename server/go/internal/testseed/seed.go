// Package testseed wires the deterministic test-only fixtures the
// cross-implementation suites (tests/python/integration/test_grpc_contract.py
// and tests/python/e2e/) expect to find on a freshly-booted server.
//
// The harness contract is documented in docs/go-port/shared/test-harness.md
// (the `--seed-tenant` and `--seed-profile` rows). The Python in-process
// fixture for the contract suite lives at tests/python/integration/conftest.py
// and seeds the same shape via direct globalstore/canonicalstore writes +
// a follow-up ExecuteAtomic call. This package reproduces that state for
// the Go subprocess harness.
//
// Profiles:
//   - "contract": User/Task/AssignedTo schema + alice/bob users +
//     seed node + seed-1 receipt. Mirrors the Python contract fixture.
//   - "e2e": User/Product/Order schema (typeIDs 8001/8002/8003) + edge
//     types (Purchased/PlacedOrder/OrderContains, 8101/8102/8103) +
//     a single `e2e-runner` user as tenant owner. No seed node — the
//     e2e suite creates all its own state.
//
// CLAUDE.md invariant #1 says all writes go through the WAL, and as of
// GitHub issue #505 this seed honours it: the seed node is written by
// appending a real WAL event through the producer and letting the
// already-running applier materialise it (idempotency + offset rows
// included). The seed path and the runtime ExecuteAtomic path are now
// byte-identical, so there is no longer a parallel-universe state and
// no in-memory offset pre-bump to poison WaitForOffset.
package testseed

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
	"github.com/elloloop/tenant-shard-db/server/go/internal/schema"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
	"google.golang.org/grpc/codes"
)

// Contract-suite fixture constants — keep in lock-step with
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

// E2E-suite fixture constants — keep in lock-step with
// tests/python/e2e/e2e_schema.proto and tests/python/e2e/test_e2e.py.
const (
	E2ERunnerUserID = "e2e-runner"
	E2ERunnerActor  = "user:e2e-runner"

	E2EUserTypeID    = 8001
	E2EProductTypeID = 8002
	E2EOrderTypeID   = 8003

	E2EPurchasedEdgeID     = 8101
	E2EPlacedOrderEdgeID   = 8102
	E2EOrderContainsEdgeID = 8103
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

// RegisterE2ESchema populates reg with the User/Product/Order types +
// Purchased/PlacedOrder/OrderContains edges the e2e suite asserts
// against. Type IDs (8001/8002/8003) and edge IDs (8101/8102/8103)
// must match tests/python/e2e/e2e_schema.proto verbatim — the e2e
// SDK loads its registry from the proto descriptor at runtime and
// the wire payloads will fail to route otherwise.
//
// Field IDs are derived from the proto field numbers (CLAUDE.md
// invariant #6 — field IDs, not names, are the storage key).
func RegisterE2ESchema(reg *schema.Registry) error {
	if reg == nil {
		return errors.New("testseed: nil registry")
	}

	user := &schema.NodeTypeDef{
		TypeID: E2EUserTypeID,
		Name:   "User",
		Fields: []schema.FieldDef{
			{FieldID: 1, Name: "email", Kind: schema.KindString, Required: true, Indexed: true},
			{FieldID: 2, Name: "name", Kind: schema.KindString, Required: true, Searchable: true},
			{FieldID: 3, Name: "age", Kind: schema.KindInteger},
			{FieldID: 4, Name: "active", Kind: schema.KindBoolean},
			{FieldID: 5, Name: "created_at", Kind: schema.KindInteger},
		},
	}
	if err := reg.RegisterNode(user); err != nil {
		return fmt.Errorf("testseed: register E2E User: %w", err)
	}

	product := &schema.NodeTypeDef{
		TypeID: E2EProductTypeID,
		Name:   "Product",
		Fields: []schema.FieldDef{
			{FieldID: 1, Name: "sku", Kind: schema.KindString, Required: true, Indexed: true, Unique: true},
			{FieldID: 2, Name: "name", Kind: schema.KindString, Required: true, Searchable: true},
			{FieldID: 3, Name: "price", Kind: schema.KindFloat, Required: true},
			{FieldID: 4, Name: "category", Kind: schema.KindEnum, EnumValues: []string{"electronics", "clothing", "food", "other"}},
			{FieldID: 5, Name: "in_stock", Kind: schema.KindBoolean},
		},
	}
	if err := reg.RegisterNode(product); err != nil {
		return fmt.Errorf("testseed: register E2E Product: %w", err)
	}

	order := &schema.NodeTypeDef{
		TypeID: E2EOrderTypeID,
		Name:   "Order",
		Fields: []schema.FieldDef{
			{FieldID: 1, Name: "order_number", Kind: schema.KindString, Required: true, Indexed: true, Unique: true},
			{FieldID: 2, Name: "total", Kind: schema.KindFloat, Required: true},
			{FieldID: 3, Name: "status", Kind: schema.KindEnum, EnumValues: []string{"pending", "paid", "shipped", "delivered", "cancelled"}},
			{FieldID: 4, Name: "created_at", Kind: schema.KindInteger},
		},
	}
	if err := reg.RegisterNode(order); err != nil {
		return fmt.Errorf("testseed: register E2E Order: %w", err)
	}

	purchased := &schema.EdgeTypeDef{
		EdgeID:     E2EPurchasedEdgeID,
		Name:       "purchased",
		FromTypeID: E2EUserTypeID,
		ToTypeID:   E2EProductTypeID,
		Props: []schema.FieldDef{
			{FieldID: 1, Name: "quantity", Kind: schema.KindInteger, Required: true},
			{FieldID: 2, Name: "price_paid", Kind: schema.KindFloat},
		},
	}
	if err := reg.RegisterEdge(purchased); err != nil {
		return fmt.Errorf("testseed: register E2E purchased: %w", err)
	}

	placedOrder := &schema.EdgeTypeDef{
		EdgeID:     E2EPlacedOrderEdgeID,
		Name:       "placed_order",
		FromTypeID: E2EUserTypeID,
		ToTypeID:   E2EOrderTypeID,
	}
	if err := reg.RegisterEdge(placedOrder); err != nil {
		return fmt.Errorf("testseed: register E2E placed_order: %w", err)
	}

	orderContains := &schema.EdgeTypeDef{
		EdgeID:     E2EOrderContainsEdgeID,
		Name:       "contains",
		FromTypeID: E2EOrderTypeID,
		ToTypeID:   E2EProductTypeID,
		Props: []schema.FieldDef{
			{FieldID: 1, Name: "quantity", Kind: schema.KindInteger, Required: true},
		},
	}
	if err := reg.RegisterEdge(orderContains); err != nil {
		return fmt.Errorf("testseed: register E2E contains: %w", err)
	}

	return nil
}

// seedWaitTimeout bounds how long SeedTenantContract waits for the
// running applier to materialise the seed event. Generous enough to
// cover a slow CI box; the applier normally catches up in single-digit
// milliseconds on the in-memory WAL.
const seedWaitTimeout = 30 * time.Second

// SeedTenantContract applies the contract-fixture seed for tenantID.
// Order mirrors the Python conftest:
//
//  1. tenant_registry row (tenantID, "Acme Corp", region default).
//  2. user_registry rows: alice + bob.
//  3. tenant_members: alice=owner, bob=member.
//  4. canonical store: OpenTenant, then the seeded-node + seed-1
//     receipt + applied_offsets row are produced by appending a real
//     WAL event and letting the running applier materialise it.
//
// The producer/topic MUST be the same producer the gRPC server and the
// running applier share, so the appended event is consumed by the
// applier that is already running (see cmd/entdb-server/main.go: the
// applier goroutine is started before the seed runs). This makes the
// seed path identical to the runtime ExecuteAtomic path — GitHub issue
// #505.
//
// All steps are best-effort idempotent: an already-existing globalstore
// row is treated as success, and the WAL event carries idempotency key
// "seed-1" so a repeated harness boot against the same data-dir dedupes
// at both the producer (same key → same StreamPos) and the applier
// (in-txn idempotency probe → StatusSkipped) layers.
func SeedTenantContract(ctx context.Context, g *globalstore.GlobalStore, s *store.CanonicalStore, producer wal.Producer, topic, tenantID string) error {
	if g == nil {
		return errors.New("testseed: nil globalstore")
	}
	if s == nil {
		return errors.New("testseed: nil canonical store")
	}
	if producer == nil {
		return errors.New("testseed: nil wal producer")
	}
	if topic == "" {
		return errors.New("testseed: empty wal topic")
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

	// Seed node — written through the real producer→applier path so
	// the applier itself records the seed-1 idempotency row and the
	// applied_offsets row (CLAUDE.md invariant #1; GitHub issue #505).
	// Payload is field-id-keyed (CLAUDE.md invariant #6): field 1 =
	// email, field 2 = name (per RegisterContractSchema). The event
	// actor IS the materialised owner_actor (see apply/ops_create_node.go),
	// so it must be AliceActor to satisfy the contract fixture.
	event := wal.Event{
		TenantID:       tenantID,
		Actor:          AliceActor,
		IdempotencyKey: SeedReceipt,
		TsMs:           time.Now().UnixMilli(),
		Ops: []map[string]any{
			{
				"op":      "create_node",
				"id":      SeedNodeID,
				"type_id": UserTypeID,
				"data":    map[string]any{"1": "seeded@example.com", "2": "Seeded"},
			},
		},
	}
	payloadBytes, err := event.Encode()
	if err != nil {
		return fmt.Errorf("testseed: encode seed event: %w", err)
	}
	headers := map[string][]byte{
		wal.HeaderIdempotencyKey: []byte(SeedReceipt),
	}
	if _, err := producer.Append(ctx, topic, tenantID, payloadBytes, headers); err != nil {
		return fmt.Errorf("testseed: append seed event: %w", err)
	}

	// Block until the running applier has finalised the seed event.
	// We poll the idempotency record (not WaitForOffset) because the
	// receipt row is the precise post-condition the contract suite
	// asserts on (GetReceiptStatus(seed-1) == APPLIED) and it is
	// written in the same applier transaction as the node + offset.
	if err := waitForSeedApplied(ctx, s, tenantID, SeedReceipt); err != nil {
		return fmt.Errorf("testseed: wait for seed event to apply: %w", err)
	}

	return nil
}

// waitForSeedApplied blocks until (tenantID, idem) is present in
// applied_events, the seed-wait budget elapses, or ctx is cancelled.
// The seed event is consumed by the applier goroutine that main.go
// starts before the seed runs, so this typically returns within a few
// milliseconds.
func waitForSeedApplied(ctx context.Context, s *store.CanonicalStore, tenantID, idem string) error {
	waitCtx, cancel := context.WithTimeout(ctx, seedWaitTimeout)
	defer cancel()

	delay := 1 * time.Millisecond
	for {
		rec, err := s.CheckIdempotencyStatus(waitCtx, tenantID, idem)
		if err != nil {
			return err
		}
		if rec.Present {
			return nil
		}
		select {
		case <-waitCtx.Done():
			return fmt.Errorf("timed out after %s waiting for seed receipt %q", seedWaitTimeout, idem)
		case <-time.After(delay):
		}
		if delay < 25*time.Millisecond {
			delay *= 2
		}
	}
}

// SeedTenant is a deprecated alias for SeedTenantContract kept until
// every caller has been migrated. New callers MUST use the explicit
// profile-aware variant.
//
// Deprecated: use SeedTenantContract.
func SeedTenant(ctx context.Context, g *globalstore.GlobalStore, s *store.CanonicalStore, producer wal.Producer, topic, tenantID string) error {
	return SeedTenantContract(ctx, g, s, producer, topic, tenantID)
}

// SeedTenantE2E applies the e2e-fixture seed for tenantID. Order:
//
//  1. tenant_registry row (tenantID, "E2E Test Tenant", region default).
//  2. user_registry row: e2e-runner.
//  3. tenant_members: e2e-runner=owner.
//  4. canonical store: OpenTenant (so the per-tenant SQLite file exists).
//
// No seed node — the e2e suite creates and asserts every node itself.
// All steps are best-effort idempotent (matches SeedTenantContract).
//
// The schema registration is the caller's responsibility (typically
// via RegisterE2ESchema before Freeze) so the registry can be frozen
// once before the canonical store is opened.
func SeedTenantE2E(ctx context.Context, g *globalstore.GlobalStore, s *store.CanonicalStore, tenantID string) error {
	if g == nil {
		return errors.New("testseed: nil globalstore")
	}
	if s == nil {
		return errors.New("testseed: nil canonical store")
	}
	if tenantID == "" {
		return errors.New("testseed: empty tenantID")
	}

	if _, err := g.CreateTenant(ctx, tenantID, "E2E Test Tenant", ""); err != nil && !isAlreadyExists(err) {
		return fmt.Errorf("testseed: create tenant: %w", err)
	}
	if _, err := g.CreateUser(ctx, E2ERunnerUserID, "e2e-runner@example.com", "E2E Runner"); err != nil && !isAlreadyExists(err) {
		return fmt.Errorf("testseed: create user e2e-runner: %w", err)
	}
	if err := g.AddTenantMember(ctx, tenantID, E2ERunnerUserID, "owner"); err != nil && !isAlreadyExists(err) {
		return fmt.Errorf("testseed: add e2e-runner as owner: %w", err)
	}

	if err := s.OpenTenant(ctx, tenantID); err != nil {
		return fmt.Errorf("testseed: open tenant: %w", err)
	}

	return nil
}

func isAlreadyExists(err error) bool {
	return errs.Code(err) == codes.AlreadyExists
}
