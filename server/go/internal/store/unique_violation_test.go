// SPDX-License-Identifier: AGPL-3.0-only

package store_test

import (
	"context"
	"strings"
	"testing"

	"github.com/elloloop/tenant-shard-db/server/go/internal/schema"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

// newStoreWithRegistry builds a tmpdir-backed store wired to a frozen
// registry so the unique-violation translator can resolve composite
// constraint coordinates.
func newStoreWithRegistry(t *testing.T, reg *schema.Registry) *store.CanonicalStore {
	t.Helper()
	dir := t.TempDir()
	cs, err := store.New(store.Options{RootDir: dir, WALMode: true, Registry: reg})
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// oauthIdentityRegistry models the issue-#566 motivating type:
// OAuthIdentity(type_id=201) with a (provider, provider_user_id)
// composite unique constraint plus a single-field unique email.
func oauthIdentityRegistry(t *testing.T) *schema.Registry {
	t.Helper()
	reg := schema.NewRegistry()
	nt := &schema.NodeTypeDef{
		TypeID: 201,
		Name:   "OAuthIdentity",
		Fields: []schema.FieldDef{
			{FieldID: 1, Name: "provider", Kind: schema.KindString},
			{FieldID: 2, Name: "provider_user_id", Kind: schema.KindString},
			{FieldID: 3, Name: "email", Kind: schema.KindString, Unique: true},
			{FieldID: 4, Name: "external_serial", Kind: schema.KindInteger},
		},
		CompositeUnique: []schema.CompositeUniqueDef{
			{Name: "provider_user_id", FieldIDs: []uint32{1, 2}},
		},
	}
	if err := reg.RegisterNode(nt); err != nil {
		t.Fatalf("RegisterNode: %v", err)
	}
	if _, err := reg.Freeze(); err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	return reg
}

// insertViaTxn inserts a node row directly on a BatchTxn connection,
// ensuring indexes first, and returns the RAW SQLite error (with the
// "index '...'" clause intact) so BuildUniqueViolationDetail can be
// exercised against the on-the-wire driver message. The txn is committed
// on success and rolled back on a constraint trip — mirroring the
// applier's batch lifecycle.
func insertViaTxn(t *testing.T, cs *store.CanonicalStore, tenant, nodeID string, typeID int32, payloadJSON string) error {
	t.Helper()
	ctx := context.Background()
	tx, err := cs.BeginBatch(ctx, tenant)
	if err != nil {
		t.Fatalf("BeginBatch: %v", err)
	}
	if err := cs.EnsureFieldIndexesTx(ctx, tx, typeID); err != nil {
		_ = tx.Rollback()
		t.Fatalf("EnsureFieldIndexesTx: %v", err)
	}
	_, insErr := tx.Conn().ExecContext(ctx,
		`INSERT INTO nodes (tenant_id,node_id,type_id,payload_json,created_at,updated_at,owner_actor,acl_blob) VALUES (?,?,?,?,?,?,?,?)`,
		tenant, nodeID, typeID, payloadJSON, 1, 1, "u", "[]")
	if insErr != nil {
		_ = tx.Rollback()
		return insErr
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	return nil
}

func TestBuildUniqueViolationDetail_Composite(t *testing.T) {
	reg := oauthIdentityRegistry(t)
	cs := newStoreWithRegistry(t, reg)
	ctx := context.Background()
	if err := cs.OpenTenant(ctx, "t1"); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}

	// First insert seeds the (google, uid-1) tuple; the txn helper ensures
	// the composite index exists.
	if err := insertViaTxn(t, cs, "t1", "n1", 201, `{"1":"google","2":"uid-1","3":"a@x.com"}`); err != nil {
		t.Fatalf("seed insert: %v", err)
	}
	// Duplicate composite tuple (distinct email so only the composite
	// constraint trips).
	err := insertViaTxn(t, cs, "t1", "n2", 201, `{"1":"google","2":"uid-1","3":"b@x.com"}`)
	if err == nil {
		t.Fatalf("expected duplicate composite insert to fail")
	}

	detail, ok := cs.BuildUniqueViolationDetail("t1", map[string]any{"1": "google", "2": "uid-1"}, err)
	if !ok {
		t.Fatalf("BuildUniqueViolationDetail did not recognise the violation: %v", err)
	}
	want := "Composite unique constraint violation: tenant=t1 type_id=201 " +
		"constraint='provider_user_id' fields=[1, 2] values=['google', 'uid-1'] already exists"
	if detail != want {
		t.Fatalf("detail mismatch:\n got=%q\nwant=%q", detail, want)
	}
}

func TestBuildUniqueViolationDetail_SingleField(t *testing.T) {
	reg := oauthIdentityRegistry(t)
	cs := newStoreWithRegistry(t, reg)
	ctx := context.Background()
	if err := cs.OpenTenant(ctx, "t1"); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}
	if err := insertViaTxn(t, cs, "t1", "n1", 201, `{"1":"google","2":"uid-1","3":"dup@x.com"}`); err != nil {
		t.Fatalf("seed insert: %v", err)
	}
	// Same email (single-field unique on field 3) but distinct composite
	// tuple so ONLY the single-field constraint trips.
	err := insertViaTxn(t, cs, "t1", "n2", 201, `{"1":"github","2":"uid-9","3":"dup@x.com"}`)
	if err == nil {
		t.Fatalf("expected duplicate email insert to fail")
	}
	detail, ok := cs.BuildUniqueViolationDetail("t1", map[string]any{"1": "github", "2": "uid-9", "3": "dup@x.com"}, err)
	if !ok {
		t.Fatalf("BuildUniqueViolationDetail did not recognise the violation: %v", err)
	}
	want := "Unique constraint violation: tenant=t1 type_id=201 field_id=3 value='dup@x.com' already exists"
	if detail != want {
		t.Fatalf("detail mismatch:\n got=%q\nwant=%q", detail, want)
	}
}

// TestBuildUniqueViolationDetail_BigIntComposite pins ADR-028 lossless
// behaviour: an int64 above 2^53 must render as the exact integer
// literal, never float scientific notation, so the SDK round-trips it.
func TestBuildUniqueViolationDetail_BigIntComposite(t *testing.T) {
	reg := schema.NewRegistry()
	nt := &schema.NodeTypeDef{
		TypeID: 301,
		Name:   "BigKey",
		Fields: []schema.FieldDef{
			{FieldID: 1, Name: "provider", Kind: schema.KindString},
			{FieldID: 2, Name: "serial", Kind: schema.KindInteger},
		},
		CompositeUnique: []schema.CompositeUniqueDef{
			{Name: "provider_serial", FieldIDs: []uint32{1, 2}},
		},
	}
	if err := reg.RegisterNode(nt); err != nil {
		t.Fatalf("RegisterNode: %v", err)
	}
	if _, err := reg.Freeze(); err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	cs := newStoreWithRegistry(t, reg)
	ctx := context.Background()
	if err := cs.OpenTenant(ctx, "t1"); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}

	const big int64 = 9223372036854775807 // math.MaxInt64, well above 2^53
	if err := insertViaTxn(t, cs, "t1", "n1", 301, `{"1":"google","2":9223372036854775807}`); err != nil {
		t.Fatalf("seed insert: %v", err)
	}
	err := insertViaTxn(t, cs, "t1", "n2", 301, `{"1":"google","2":9223372036854775807}`)
	if err == nil {
		t.Fatalf("expected duplicate big-int composite insert to fail")
	}
	detail, ok := cs.BuildUniqueViolationDetail("t1", map[string]any{"1": "google", "2": big}, err)
	if !ok {
		t.Fatalf("BuildUniqueViolationDetail did not recognise the violation: %v", err)
	}
	if !strings.Contains(detail, "9223372036854775807") {
		t.Fatalf("big int not rendered losslessly: %q", detail)
	}
	if strings.Contains(detail, "e+") || strings.Contains(detail, "E+") {
		t.Fatalf("big int rendered in scientific notation: %q", detail)
	}
	want := "Composite unique constraint violation: tenant=t1 type_id=301 " +
		"constraint='provider_serial' fields=[1, 2] values=['google', 9223372036854775807] already exists"
	if detail != want {
		t.Fatalf("detail mismatch:\n got=%q\nwant=%q", detail, want)
	}
}

// TestBuildUniqueViolationDetail_Unrecognised confirms a non-unique-index
// error (e.g. a primary-key collision, which names no index) is not
// misclassified as a declared unique violation.
func TestBuildUniqueViolationDetail_Unrecognised(t *testing.T) {
	reg := oauthIdentityRegistry(t)
	cs := newStoreWithRegistry(t, reg)
	ctx := context.Background()
	if err := cs.OpenTenant(ctx, "t1"); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}
	if err := insertViaTxn(t, cs, "t1", "dup", 201, `{"1":"google","2":"uid-1"}`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// Re-insert the SAME node_id -> PRIMARY KEY collision, which SQLite
	// reports without an "index '...'" clause.
	err := insertViaTxn(t, cs, "t1", "dup", 201, `{"1":"yahoo","2":"uid-2"}`)
	if err == nil {
		t.Fatalf("expected PK collision")
	}
	if _, ok := cs.BuildUniqueViolationDetail("t1", map[string]any{}, err); ok {
		t.Fatalf("PK collision should NOT be recognised as a declared unique violation")
	}
}

// TestEnsureFieldIndexesTx_CreatesCompositeIndex verifies the applier's
// in-txn ensure path actually mints the composite unique index so the
// constraint fires inside the same transaction.
func TestEnsureFieldIndexesTx_CreatesCompositeIndex(t *testing.T) {
	reg := oauthIdentityRegistry(t)
	cs := newStoreWithRegistry(t, reg)
	ctx := context.Background()
	if err := cs.OpenTenant(ctx, "t1"); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}
	tx, err := cs.BeginBatch(ctx, "t1")
	if err != nil {
		t.Fatalf("BeginBatch: %v", err)
	}
	if err := cs.EnsureFieldIndexesTx(ctx, tx, 201); err != nil {
		_ = tx.Rollback()
		t.Fatalf("EnsureFieldIndexesTx: %v", err)
	}
	// Two distinct rows with the same composite tuple inside the SAME txn
	// must collide.
	conn := tx.Conn()
	exec := func(id, p string) error {
		_, e := conn.ExecContext(ctx,
			`INSERT INTO nodes (tenant_id,node_id,type_id,payload_json,created_at,updated_at,owner_actor,acl_blob) VALUES (?,?,?,?,?,?,?,?)`,
			"t1", id, 201, p, 1, 1, "u", "[]")
		return e
	}
	if err := exec("a", `{"1":"google","2":"uid-1"}`); err != nil {
		_ = tx.Rollback()
		t.Fatalf("insert a: %v", err)
	}
	dupErr := exec("b", `{"1":"google","2":"uid-1"}`)
	_ = tx.Rollback()
	if dupErr == nil {
		t.Fatalf("expected composite collision inside txn")
	}
	if name := indexNameForTest(dupErr); !strings.Contains(name, "idx_unique_t201_cprovider_user_id") {
		t.Fatalf("collision did not name the composite index: %v", dupErr)
	}
}

// indexNameForTest is a small accessor over the error string so the test
// asserts on the index the violation named.
func indexNameForTest(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
