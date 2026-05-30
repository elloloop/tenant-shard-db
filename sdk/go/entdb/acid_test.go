//go:build integration

package entdb

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/elloloop/tenant-shard-db/sdk/go/entdb/v2/internal/testpb"
)

// =============================================================================
// Go SDK E2E ACID Tests
// =============================================================================

// TestIntegration_Acid_Atomicity_RollbackOnPreconditionFailure proves that
// when an operation inside a plan fails a precondition check (CAS mismatch),
// the entire transaction rolls back and no partial changes are committed.
func TestIntegration_Acid_Atomicity_RollbackOnPreconditionFailure(t *testing.T) {
	ctx := context.Background()
	c := newSchemaClientForIINEWithComposite(t, ctx)
	scope := c.Tenant(itTenant).Actor(UserActor("e2e-runner"))

	uid := fmt.Sprintf("atom-pre-%d", time.Now().UnixNano())
	nodeID := fmt.Sprintf("u-atom-pre-%s", uid)

	plan := scope.Plan()

	// Op 1: Create a valid Product
	p1 := testpb.NewProductMsg()
	p1.SetFields(fName(nodeID), "Atomicity Pre Product", 999)
	plan.Create(p1, WithID(nodeID))

	// Op 2: Update a nonexistent node with an impossible precondition
	patch := testpb.NewProductMsg()
	patch.SetFields("", "Failed Update", 1999)
	plan.UpdateIf("nonexistent-node-id", patch, "sku", "impossible-precondition-value")

	// Commit must fail
	_, err := plan.Commit(ctx)
	if err == nil {
		t.Fatal("expected Commit to fail due to mismatching precondition, got nil error")
	}

	if !errors.Is(err, ErrPreconditionFailed) {
		t.Errorf("expected ErrPreconditionFailed error, got: %v", err)
	}

	// Verify that the Product from Op 1 was NOT created (rolled back)
	node, err := entdbGetProduct(ctx, scope, nodeID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if node != nil {
		t.Errorf("Atomicity violation: Product node %s was committed despite plan failure!", nodeID)
	}
}

// TestIntegration_Acid_Atomicity_RollbackOnUniqueViolation proves that
// when an operation inside a plan violates a unique key constraint, the
// entire transaction rolls back and no partial changes are committed.
func TestIntegration_Acid_Atomicity_RollbackOnUniqueViolation(t *testing.T) {
	ctx := context.Background()
	c := newSchemaClientForIINEWithComposite(t, ctx)
	scope := c.Tenant(itTenant).Actor(UserActor("e2e-runner"))

	uid := fmt.Sprintf("atom-uniq-%d", time.Now().UnixNano())
	sharedSKU := fmt.Sprintf("shared-sku-%s", uid)

	// First, seed a product with the shared SKU
	seedPlan := scope.Plan()
	pSeed := testpb.NewProductMsg()
	pSeed.SetFields(sharedSKU, "Original Product", 100)
	seedPlan.Create(pSeed, WithID(fmt.Sprintf("p-seed-%s", uid)))
	seedRes, err := seedPlan.Commit(ctx, WithWaitApplied(true))
	if err != nil || !seedRes.Success {
		t.Fatalf("seed Product: %v, %+v", err, seedRes)
	}

	// Now attempt to insert a valid product *and* a duplicate product in a single atomic transaction
	okNodeID := fmt.Sprintf("p-ok-%s", uid)
	plan := scope.Plan()

	// Op 1: Valid product
	pOK := testpb.NewProductMsg()
	pOK.SetFields(fmt.Sprintf("distinct-sku-%s", uid), "Valid Product", 200)
	plan.Create(pOK, WithID(okNodeID))

	// Op 2: Duplicate product violating unique constraint on SKU
	pDup := testpb.NewProductMsg()
	pDup.SetFields(sharedSKU, "Duplicate Product", 300)
	plan.Create(pDup, WithID(fmt.Sprintf("p-dup-%s", uid)))

	// Commit must fail
	_, err = plan.Commit(ctx, WithWaitApplied(true))
	if err == nil {
		t.Fatal("expected Commit to fail due to unique constraint violation, got nil error")
	}

	var uce *UniqueConstraintError
	if !errors.As(err, &uce) {
		t.Errorf("expected UniqueConstraintError, got: %v", err)
	}

	// Verify that the OK product node was NOT committed (rolled back)
	node, err := entdbGetProduct(ctx, scope, okNodeID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if node != nil {
		t.Errorf("Atomicity violation: Product node %s was committed despite unique violation in plan!", okNodeID)
	}
}

// TestIntegration_Acid_Consistency_UniqueField proves that the database
// preserves consistency by rejecting duplicate writes on unique fields.
func TestIntegration_Acid_Consistency_UniqueField(t *testing.T) {
	ctx := context.Background()
	c := newSchemaClientForIINEWithComposite(t, ctx)
	scope := c.Tenant(itTenant).Actor(UserActor("e2e-runner"))

	uid := fmt.Sprintf("consis-%d", time.Now().UnixNano())
	sku := fmt.Sprintf("uniq-sku-%s", uid)

	// First insert: succeeds
	p1 := scope.Plan()
	prod1 := testpb.NewProductMsg()
	prod1.SetFields(sku, "Product 1", 500)
	p1.Create(prod1)
	res1, err := p1.Commit(ctx, WithWaitApplied(true))
	if err != nil || !res1.Success {
		t.Fatalf("first commit: %v, %+v", err, res1)
	}

	// Second insert of same SKU: fails
	p2 := scope.Plan()
	prod2 := testpb.NewProductMsg()
	prod2.SetFields(sku, "Product 2", 1000)
	p2.Create(prod2)

	_, err = p2.Commit(ctx, WithWaitApplied(true))
	if err == nil {
		t.Fatal("expected second commit to fail, got nil error")
	}

	var uce *UniqueConstraintError
	if !errors.As(err, &uce) {
		t.Fatalf("expected UniqueConstraintError, got: %v", err)
	}

	// Verify details
	if uce.TypeID != 201 {
		t.Errorf("expected TypeID 201, got %d", uce.TypeID)
	}
	if uce.FieldID != 1 {
		t.Errorf("expected FieldID 1, got %d", uce.FieldID)
	}
	if uce.Value != sku {
		t.Errorf("expected Value %q, got %v", sku, uce.Value)
	}
}

// TestIntegration_Acid_Isolation_LostUpdatePrecondition proves that optimistic
// concurrency control (OCC) prevents lost updates under concurrent transitions.
func TestIntegration_Acid_Isolation_LostUpdatePrecondition(t *testing.T) {
	ctx := context.Background()
	c := newSchemaClientForIINEWithComposite(t, ctx)
	scope := c.Tenant(itTenant).Actor(UserActor("e2e-runner"))

	uid := fmt.Sprintf("iso-%d", time.Now().UnixNano())
	nodeID := fmt.Sprintf("p-iso-%s", uid)
	origSKU := fmt.Sprintf("orig-sku-%s", uid)

	// Seed the product
	seedPlan := scope.Plan()
	pSeed := testpb.NewProductMsg()
	pSeed.SetFields(origSKU, "Original Product", 100)
	seedPlan.Create(pSeed, WithID(nodeID))
	seedRes, err := seedPlan.Commit(ctx, WithWaitApplied(true))
	if err != nil || !seedRes.Success {
		t.Fatalf("seed Product: %v", err)
	}

	// Client A and Client B read the same state (SKU == origSKU)
	node, err := entdbGetProduct(ctx, scope, nodeID)
	if err != nil || node == nil {
		t.Fatalf("read seeded node: %v, %v", err, node)
	}
	if node.SKU() != origSKU {
		t.Fatalf("unexpected SKU: %s", node.SKU())
	}

	// Client A updates name to 'Product A' with precondition SKU == origSKU (succeeds)
	planA := scope.Plan()
	pA := testpb.NewProductMsg()
	pA.SetFields("changed-sku", "Product A", 100)
	planA.UpdateIf(nodeID, pA, "sku", origSKU)
	resA, err := planA.Commit(ctx, WithWaitApplied(true))
	if err != nil || !resA.Success {
		t.Fatalf("Client A commit: %v", err)
	}

	// Client B attempts to update price to 500 with precondition SKU == origSKU
	// It must fail because Client A changed the state (advanced the version/offset)
	planB := scope.Plan()
	pB := testpb.NewProductMsg()
	pB.SetFields(origSKU, "Product B", 500)
	planB.UpdateIf(nodeID, pB, "sku", origSKU)
	_, err = planB.Commit(ctx, WithWaitApplied(true))
	if err == nil {
		t.Fatal("expected Client B commit to fail, got nil error")
	}

	var pf *PreconditionFailure
	if !errors.As(err, &pf) {
		t.Fatalf("expected PreconditionFailure, got: %v", err)
	}

	if pf.Field != "1" {
		t.Errorf("expected Field '1' (sku), got %q", pf.Field)
	}
	if pf.ExpectedValue != origSKU {
		t.Errorf("expected ExpectedValue %v, got %v", origSKU, pf.ExpectedValue)
	}
}

// TestIntegration_Acid_Durability verifies that committed transactions are
// durable on disk and survive process restarts.
func TestIntegration_Acid_Durability(t *testing.T) {
	ctx := context.Background()

	// 1. Create a dedicated temporary directory for this test
	tmp, err := os.MkdirTemp("", "entdb-durability-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(tmp)

	// 2. Build the server binary
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	repoRoot := filepath.Clean(filepath.Join(wd, "..", "..", ".."))
	serverDir := filepath.Join(repoRoot, "server", "go")
	bin := filepath.Join(tmp, "entdb-server")
	build := exec.Command("go", "build", "-o", bin, "./cmd/entdb-server")
	build.Dir = serverDir
	build.Stdout, build.Stderr = os.Stderr, os.Stderr
	if err := build.Run(); err != nil {
		t.Fatalf("build server: %v", err)
	}

	port, err := freePort()
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	dataDir := filepath.Join(tmp, "data")

	// 3. Start the first server process
	srv1 := exec.Command(bin,
		"--addr", addr,
		"--data-dir", dataDir,
		"--wal-backend", "memory",
	)
	srv1.Stdout, srv1.Stderr = os.Stderr, os.Stderr
	if err := srv1.Start(); err != nil {
		t.Fatalf("start server 1: %v", err)
	}

	// Helper to ensure server is killed
	var killed bool
	killServer := func(srv *exec.Cmd) {
		if !killed {
			_ = srv.Process.Kill()
			_, _ = srv.Process.Wait()
		}
	}
	defer func() { killServer(srv1) }()

	// Wait for server 1 to be ready
	client1, err := NewClient(addr, WithSchema(testpb.NewProductMsg()))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if err := client1.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer client1.Close()

	// Wait up to 5s for health check
	deadline := time.Now().Add(5 * time.Second)
	ready := false
	for time.Now().Before(deadline) {
		if _, herr := client1.Health(ctx); herr == nil {
			ready = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !ready {
		t.Fatal("server 1 never became ready")
	}

	// Bootstrap tenant and user schema
	if err := bootstrapIntegrationContract(ctx, addr); err != nil {
		t.Fatalf("bootstrap server 1: %v", err)
	}

	// 4. Write a product node and wait for it to be applied durably
	scope1 := client1.Tenant(itTenant).Actor(UserActor("e2e-runner"))
	uid := fmt.Sprintf("dur-%d", time.Now().UnixNano())
	nodeID := fmt.Sprintf("p-dur-%s", uid)
	sku := fmt.Sprintf("sku-dur-%s", uid)

	plan1 := scope1.Plan()
	p1 := testpb.NewProductMsg()
	p1.SetFields(sku, "Durable Product", 4200)
	plan1.Create(p1, WithID(nodeID))
	res1, err := plan1.Commit(ctx, WithWaitApplied(true))
	if err != nil || !res1.Success {
		t.Fatalf("Commit product: %v, %+v", err, res1)
	}

	// Verify it's readable before restart
	prodPre, err := entdbGetProduct(ctx, scope1, nodeID)
	if err != nil || prodPre == nil {
		t.Fatalf("Get before restart: %v, %v", err, prodPre)
	}

	// 5. Shutdown server 1 cleanly
	_ = client1.Close()
	_ = srv1.Process.Kill()
	_, _ = srv1.Process.Wait()
	killed = true

	// 6. Start server 2 pointing to the SAME data directory
	killed = false
	srv2 := exec.Command(bin,
		"--addr", addr,
		"--data-dir", dataDir,
		"--wal-backend", "memory",
	)
	srv2.Stdout, srv2.Stderr = os.Stderr, os.Stderr
	if err := srv2.Start(); err != nil {
		t.Fatalf("start server 2: %v", err)
	}
	defer func() { killServer(srv2) }()

	// Wait for server 2 to be ready
	client2, err := NewClient(addr, WithSchema(testpb.NewProductMsg()))
	if err != nil {
		t.Fatalf("NewClient 2: %v", err)
	}
	if err := client2.Connect(ctx); err != nil {
		t.Fatalf("Connect 2: %v", err)
	}
	defer client2.Close()

	ready = false
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, herr := client2.Health(ctx); herr == nil {
			ready = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !ready {
		t.Fatal("server 2 never became ready")
	}

	// 7. Verify the node still exists and is correct (Durability!)
	scope2 := client2.Tenant(itTenant).Actor(UserActor("e2e-runner"))
	prodPost, err := entdbGetProduct(ctx, scope2, nodeID)
	if err != nil {
		t.Fatalf("Get after restart: %v", err)
	}
	if prodPost == nil {
		t.Fatal("Durability violation: product was lost after server restart!")
	}
	if prodPost.SKU() != sku || prodPost.Name() != "Durable Product" {
		t.Errorf("Durability violation: product data corrupted! Got: sku=%q, name=%q", prodPost.SKU(), prodPost.Name())
	}
}

// Helpers to get Product typed nodes in tests
func entdbGetProduct(ctx context.Context, scope *Scope, id string) (*testpb.Product, error) {
	node, err := Get[*testpb.Product](ctx, scope, id)
	return node, err
}

func fName(s string) string {
	return s
}
