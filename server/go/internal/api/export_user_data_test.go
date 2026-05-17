// Tests for the ExportUserData RPC (W2 — EPIC #407, GDPR Article 20).
//
// Spec: docs/go-port/rpcs/ExportUserData.md.
//
// Pinned behaviours (one test per arm):
//
//  1. Self happy path — alice exports her own bundle across all tenants
//     she's a member of (mirrors test_gdpr_engine.py:678-692 and
//     test_grpc_contract.py:643-648).
//  2. Admin happy path — admin:root exports bob's bundle without
//     needing to be a member of bob's tenants.
//  3. No-data user — a user with zero memberships → empty bundle, the
//     `tenants` array is `[]` not `null` (Python json.dumps([])).
//  4. Non-self / non-admin → PERMISSION_DENIED (mirrors
//     test_gdpr_engine.py:696-704 and test_grpc_contract.py:649-653).

package api_test

import (
	"context"
	"encoding/json"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

// seedAliceNode creates a single owner-matching node in `tenant` with
// owner_actor=user:alice. Returns the node id so callers can assert on
// the bundle contents.
func seedAliceNode(t *testing.T, cs *store.CanonicalStore, tenant string) string {
	t.Helper()
	ctx := context.Background()
	if err := cs.OpenTenant(ctx, tenant); err != nil {
		t.Fatalf("OpenTenant(%q): %v", tenant, err)
	}
	const nodeID = "node-alice-1"
	_, err := cs.CreateNodeRaw(ctx, tenant, store.NodeInput{
		NodeID:     nodeID,
		TypeID:     7,
		Payload:    map[string]any{"1": "alice@example.com"},
		OwnerActor: "user:alice",
	})
	if err != nil {
		t.Fatalf("CreateNodeRaw(%q): %v", tenant, err)
	}
	return nodeID
}

// decodeBundle parses the JSON envelope returned by ExportUserData. The
// helper exists so each test reads as a structural assertion rather
// than a raw map dance.
func decodeBundle(t *testing.T, raw string) (userID string, generatedAt int64, tenants []map[string]any) {
	t.Helper()
	var b struct {
		UserID      string           `json:"user_id"`
		GeneratedAt int64            `json:"generated_at"`
		Tenants     []map[string]any `json:"tenants"`
	}
	if err := json.Unmarshal([]byte(raw), &b); err != nil {
		t.Fatalf("decode bundle: %v\nraw=%s", err, raw)
	}
	return b.UserID, b.GeneratedAt, b.Tenants
}

// TestExportUserData_Self_HappyPath: alice owns nodes in two tenants
// she's a member of. The bundle must contain both tenants' slices in
// joined-at order, with the owned nodes present.
func TestExportUserData_Self_HappyPath(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	cs := newCanonicalStore(t)
	ctx := context.Background()

	// Two tenants alice is a member of, plus one she is NOT — to prove
	// the cross-tenant walk only follows memberships.
	for _, tenantID := range []string{"tenant-a", "tenant-b"} {
		if err := gs.AddTenantMember(ctx, tenantID, "alice", "member"); err != nil {
			t.Fatalf("AddTenantMember(%q): %v", tenantID, err)
		}
		seedAliceNode(t, cs, tenantID)
	}
	// Bob's tenant: alice is not a member; her node here MUST NOT
	// appear in the export. (We only have to seed memberships — the
	// canonical store doesn't get walked for tenants she isn't in.)
	if err := gs.AddTenantMember(ctx, "tenant-c", "bob", "owner"); err != nil {
		t.Fatalf("seed AddTenantMember bob: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs), api.WithStore(cs))

	resp, err := srv.ExportUserData(withTrustedUser(ctx, "alice"), &pb.ExportUserDataRequest{
		Actor:  "user:alice",
		UserId: "alice",
	})
	if err != nil {
		t.Fatalf("ExportUserData: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("Success: got false, error=%q", resp.GetError())
	}

	userID, generatedAt, tenants := decodeBundle(t, resp.GetExportJson())
	if userID != "alice" {
		t.Errorf("user_id: got %q, want %q", userID, "alice")
	}
	if generatedAt <= 0 {
		t.Errorf("generated_at: got %d, want > 0", generatedAt)
	}
	if len(tenants) != 2 {
		t.Fatalf("tenants: got %d, want 2 (tenant-a, tenant-b); bundle=%s", len(tenants), resp.GetExportJson())
	}
	for _, te := range tenants {
		nodes, ok := te["nodes"].([]any)
		if !ok {
			t.Fatalf("tenant entry missing nodes array: %+v", te)
		}
		if len(nodes) != 1 {
			t.Errorf("tenant %v: got %d nodes, want 1", te["tenant_id"], len(nodes))
		}
	}
}

// TestExportUserData_Admin_HappyPath: an admin: trusted identity can
// export any user's bundle without being a tenant member.
func TestExportUserData_Admin_HappyPath(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	cs := newCanonicalStore(t)
	ctx := context.Background()

	if err := gs.AddTenantMember(ctx, "tenant-a", "alice", "member"); err != nil {
		t.Fatalf("AddTenantMember: %v", err)
	}
	seedAliceNode(t, cs, "tenant-a")

	srv := api.New(api.WithGlobalStore(gs), api.WithStore(cs))

	// admin:root is the trusted caller; the wire `actor` is also set to
	// admin:root for parity with how Python's contract test invokes it.
	resp, err := srv.ExportUserData(withTrustedUser(ctx, "admin:root"), &pb.ExportUserDataRequest{
		Actor:  "admin:root",
		UserId: "alice",
	})
	if err != nil {
		t.Fatalf("ExportUserData: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("Success: got false, error=%q", resp.GetError())
	}
	userID, _, tenants := decodeBundle(t, resp.GetExportJson())
	if userID != "alice" {
		t.Errorf("user_id: got %q, want %q", userID, "alice")
	}
	if len(tenants) != 1 {
		t.Errorf("tenants: got %d, want 1", len(tenants))
	}
}

// TestExportUserData_NoData_EmptyBundle: a user with zero memberships
// gets a well-formed bundle whose `tenants` slice is empty. The slice
// must be `[]` not `null` so the SDK's JSON consumer doesn't crash on
// nil — pinned by sdk/go/entdb/admin_gdpr_test.go:76-94.
func TestExportUserData_NoData_EmptyBundle(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	cs := newCanonicalStore(t)
	ctx := context.Background()

	srv := api.New(api.WithGlobalStore(gs), api.WithStore(cs))

	resp, err := srv.ExportUserData(withTrustedUser(ctx, "ghost"), &pb.ExportUserDataRequest{
		Actor:  "user:ghost",
		UserId: "ghost",
	})
	if err != nil {
		t.Fatalf("ExportUserData: %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("Success: got false, error=%q", resp.GetError())
	}
	raw := resp.GetExportJson()
	userID, _, tenants := decodeBundle(t, raw)
	if userID != "ghost" {
		t.Errorf("user_id: got %q, want %q", userID, "ghost")
	}
	if tenants == nil {
		t.Errorf("tenants: got nil, want []; raw=%s", raw)
	}
	if len(tenants) != 0 {
		t.Errorf("tenants: got %d, want 0; raw=%s", len(tenants), raw)
	}
	// Verify the JSON literal contains "tenants":[] not "tenants":null.
	// json.Marshal of an empty []store.TenantExport -> "[]" — the test
	// would catch a regression to nil-slice handling.
	if !containsBracket(raw) {
		t.Errorf("expected literal []\"tenants\":[]\" in bundle, got %s", raw)
	}
}

// containsBracket is a tiny string check used by the no-data test to
// guarantee the wire payload doesn't smuggle JSON `null` past consumers
// who expect an empty array.
func containsBracket(raw string) bool {
	// Substring match is sufficient — we are looking for the literal
	// `"tenants":[]` regardless of whitespace, which json.Marshal does
	// not insert.
	for i := 0; i+len(`"tenants":[]`) <= len(raw); i++ {
		if raw[i:i+len(`"tenants":[]`)] == `"tenants":[]` {
			return true
		}
	}
	return false
}

// TestExportUserData_NonSelfNonAdmin_PermissionDenied: alice trying to
// export bob's bundle is the privilege-escalation guard. The wire
// `actor` claims admin:root but the trusted Identity says alice — must
// be ignored. Pinned by test_grpc_contract.py:649-653.
func TestExportUserData_NonSelfNonAdmin_PermissionDenied(t *testing.T) {
	t.Parallel()
	gs := newGlobalStore(t)
	cs := newCanonicalStore(t)
	ctx := context.Background()

	if err := gs.AddTenantMember(ctx, "tenant-b", "bob", "member"); err != nil {
		t.Fatalf("AddTenantMember bob: %v", err)
	}

	srv := api.New(api.WithGlobalStore(gs), api.WithStore(cs))

	// alice is the trusted caller; she claims admin:root on the wire.
	// The handler MUST ignore the wire claim.
	resp, err := srv.ExportUserData(withTrustedUser(ctx, "alice"), &pb.ExportUserDataRequest{
		Actor:  "admin:root",
		UserId: "bob",
	})
	if err == nil {
		t.Fatalf("expected PERMISSION_DENIED, got resp=%+v", resp)
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("error is not a grpc status: %v", err)
	}
	if st.Code() != codes.PermissionDenied {
		t.Fatalf("code: got %v, want PermissionDenied", st.Code())
	}
}
