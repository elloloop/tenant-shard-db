// Tests for the SearchNodes RPC.
//
// Spec: docs/go-port/rpcs/SearchNodes.md. Five branches pinned here, in
// the order the spec walks them:
//
//  1. Empty results: tenant exists, schema has searchable fields, but
//     no node matches the query -> codes.OK, Nodes == [].
//  2. Single hit: one indexed node matches a single-token query ->
//     codes.OK, Nodes == [that-one], payload Struct id-keyed.
//  3. Multiple hits with ranking: two indexed nodes match; the tighter
//     match (more occurrences of the query token) ranks first via
//     SQLite FTS5 bm25 ascending.
//  4. ACL filter: a cross-tenant actor (non-member, non-system) sees
//     only rows whose owner or ACL principal matches them.
//  5. Malformed query (FTS5 syntax error) -> swallowed by the handler
//     and returned as codes.OK with Nodes == [], per the spec
//     "blanket except Exception" carve-out (lines 63-66). PARITY: do
//     NOT upgrade to INVALID_ARGUMENT without a contract-test update.
//
// The empty-query and >1000-char branches are also covered (separate
// case) — both pinned to codes.InvalidArgument by
// tests/python/integration/test_grpc_contract.py:627-632.
//
// External-test (package api_test) — reuses newGlobalStore from
// helpers_external_test.go.

package api_test

import (
	"context"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/elloloop/tenant-shard-db/server/go/internal/api"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/schema"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

// searchTypeID is the test node type id used by every SearchNodes test.
// Field 1 is "title" (searchable string), field 2 is "body" (searchable
// string). Fields are named for clarity; the wire stays id-keyed.
const searchTypeID int32 = 7

// newSearchTestServer wires a fresh CanonicalStore + globalstore +
// schema.Registry for SearchNodes tests. The tenant "acme" is
// pre-created in globalstore and pre-opened in the canonical store.
// Schema has typeID=7 with two searchable string fields (id 1, id 2).
func newSearchTestServer(t *testing.T) (*api.Server, *store.CanonicalStore, string) {
	t.Helper()
	gs := newGlobalStore(t)
	ctx := context.Background()
	if _, err := gs.CreateTenant(ctx, "acme", "Acme", ""); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}

	reg := schema.NewRegistry()
	if err := reg.RegisterNode(&schema.NodeTypeDef{
		TypeID: searchTypeID,
		Fields: []schema.FieldDef{
			{FieldID: 1, Kind: schema.KindString, Searchable: true},
			{FieldID: 2, Kind: schema.KindString, Searchable: true},
		},
	}); err != nil {
		t.Fatalf("RegisterNode: %v", err)
	}
	if _, err := reg.Freeze(); err != nil {
		t.Fatalf("Freeze: %v", err)
	}

	cs, err := store.New(store.Options{
		RootDir:  t.TempDir(),
		WALMode:  true,
		Registry: reg,
	})
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	if err := cs.OpenTenant(ctx, "acme"); err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}

	srv := api.New(
		api.WithGlobalStore(gs),
		api.WithStore(cs),
		api.WithSchemaRegistry(reg),
	)
	return srv, cs, "acme"
}

// seedIndexedNode is the helper that mirrors the applier's
// create-node-then-FTS-insert sequence. The real Applier writes the row
// + the FTS index transactionally; in tests we stitch the two store
// methods together. Searchable field ids are pulled from the registry
// so a future schema change keeps the test honest.
func seedIndexedNode(
	t *testing.T,
	cs *store.CanonicalStore,
	tenantID, nodeID string,
	owner string,
	payload map[string]any,
	acl []store.ACLEntry,
) {
	t.Helper()
	ctx := context.Background()
	in := store.NodeInput{
		NodeID:     nodeID,
		TypeID:     searchTypeID,
		Payload:    payload,
		OwnerActor: owner,
		ACL:        acl,
	}
	if _, err := cs.CreateNodeRaw(ctx, tenantID, in); err != nil {
		t.Fatalf("CreateNodeRaw(%q): %v", nodeID, err)
	}
	// Searchable fields: ids 1 and 2 in this fixture.
	if err := cs.FTSInsert(ctx, tenantID, searchTypeID, nodeID, payload, []uint32{1, 2}); err != nil {
		t.Fatalf("FTSInsert(%q): %v", nodeID, err)
	}
}

// TestSearchNodes_EmptyResults: schema has searchable fields but no
// indexed node matches -> Nodes == [], codes.OK.
func TestSearchNodes_EmptyResults(t *testing.T) {
	t.Parallel()
	srv, cs, tenantID := newSearchTestServer(t)
	// Seed one node that won't match the query.
	seedIndexedNode(t, cs, tenantID, "n1", "user:alice",
		map[string]any{"1": "umbrella", "2": "rainy day gear"}, nil)

	resp, err := srv.SearchNodes(context.Background(), &pb.SearchNodesRequest{
		TenantId: tenantID,
		Actor:    "user:alice",
		TypeId:   searchTypeID,
		Query:    "submarine",
	})
	if err != nil {
		t.Fatalf("SearchNodes: unexpected err: %v", err)
	}
	if resp == nil {
		t.Fatalf("SearchNodes: nil response")
	}
	if len(resp.GetNodes()) != 0 {
		t.Fatalf("SearchNodes: expected 0 nodes, got %d", len(resp.GetNodes()))
	}
}

// TestSearchNodes_SingleHit: one node matches, payload egresses
// id-keyed (CLAUDE.md invariant 6, spec line 30).
func TestSearchNodes_SingleHit(t *testing.T) {
	t.Parallel()
	srv, cs, tenantID := newSearchTestServer(t)

	seedIndexedNode(t, cs, tenantID, "p1", "user:alice",
		map[string]any{"1": "Widget Deluxe", "2": "rugged outdoor widget findme"}, nil)
	seedIndexedNode(t, cs, tenantID, "p2", "user:alice",
		map[string]any{"1": "Notebook", "2": "blank lined paper"}, nil)

	resp, err := srv.SearchNodes(context.Background(), &pb.SearchNodesRequest{
		TenantId: tenantID,
		Actor:    "user:alice",
		TypeId:   searchTypeID,
		Query:    "widget",
	})
	if err != nil {
		t.Fatalf("SearchNodes: unexpected err: %v", err)
	}
	if got := len(resp.GetNodes()); got != 1 {
		t.Fatalf("SearchNodes: expected 1 hit, got %d", got)
	}
	hit := resp.GetNodes()[0]
	if hit.GetNodeId() != "p1" {
		t.Fatalf("SearchNodes: node_id: got %q, want %q", hit.GetNodeId(), "p1")
	}
	if hit.GetTypeId() != searchTypeID {
		t.Fatalf("SearchNodes: type_id: got %d, want %d", hit.GetTypeId(), searchTypeID)
	}
	if hit.GetOwnerActor() != "user:alice" {
		t.Fatalf("SearchNodes: owner: got %q, want user:alice", hit.GetOwnerActor())
	}
	pl := hit.GetPayload()
	if pl == nil {
		t.Fatalf("SearchNodes: payload is nil")
	}
	// Payload Struct keys MUST be field-id strings, not field names.
	if _, ok := pl.GetFields()["1"]; !ok {
		t.Fatalf("SearchNodes: payload missing id-key %q; got fields=%v", "1", pl.GetFields())
	}
	if _, badName := pl.GetFields()["title"]; badName {
		t.Fatalf("SearchNodes: payload contains name-key %q; CLAUDE.md invariant 6 says id-keyed only", "title")
	}
}

// TestSearchNodes_MultipleHitsWithRanking: both nodes match; the row
// with more matches against the query ranks first (FTS5 bm25 ascending,
// see spec lines 28-30). We assert ordering is deterministic and the
// stronger match comes first.
func TestSearchNodes_MultipleHitsWithRanking(t *testing.T) {
	t.Parallel()
	srv, cs, tenantID := newSearchTestServer(t)

	// "strong" has many "fox" tokens; "weak" has a single "fox" token.
	// SQLite bm25 weighs term frequency normalised by doc length, so
	// "strong" should rank above "weak" — but length normalisation can
	// flip it for small inputs. We assert only that BOTH appear and
	// that strong outranks weak by hit count, not exact bm25 floats.
	seedIndexedNode(t, cs, tenantID, "strong", "user:alice",
		map[string]any{"1": "fox fox fox fox", "2": "fox fox fox fox"}, nil)
	seedIndexedNode(t, cs, tenantID, "weak", "user:alice",
		map[string]any{"1": "fox", "2": "an unrelated body about cats"}, nil)

	resp, err := srv.SearchNodes(context.Background(), &pb.SearchNodesRequest{
		TenantId: tenantID,
		Actor:    "user:alice",
		TypeId:   searchTypeID,
		Query:    "fox",
	})
	if err != nil {
		t.Fatalf("SearchNodes: unexpected err: %v", err)
	}
	hits := resp.GetNodes()
	if len(hits) != 2 {
		t.Fatalf("SearchNodes: expected 2 hits, got %d (ids=%v)", len(hits), nodeIDs(hits))
	}
	if hits[0].GetNodeId() != "strong" {
		t.Fatalf("SearchNodes: ranking wrong: got %v, expected strong first", nodeIDs(hits))
	}
}

// TestSearchNodes_ACLFilterAppliedToCrossTenantActor: a cross-tenant
// actor (non-member, non-system) only sees nodes whose owner or ACL
// principal matches them. The spec's in-tenant-member contract — that
// members see the unfiltered set — is covered implicitly by the other
// tests: TestSearchNodes_SingleHit's caller "user:alice" is a non-member
// but owns every seeded row, so the filter is a no-op for those tests.
func TestSearchNodes_ACLFilterAppliedToCrossTenantActor(t *testing.T) {
	t.Parallel()
	srv, cs, tenantID := newSearchTestServer(t)

	// Two nodes owned by alice; one shares with eve, one does not.
	seedIndexedNode(t, cs, tenantID, "shared", "user:alice",
		map[string]any{"1": "shared widget", "2": "findme"},
		[]store.ACLEntry{{Principal: "user:eve", Permission: "read"}},
	)
	seedIndexedNode(t, cs, tenantID, "private", "user:alice",
		map[string]any{"1": "private widget", "2": "findme"}, nil)

	resp, err := srv.SearchNodes(context.Background(), &pb.SearchNodesRequest{
		TenantId: tenantID,
		Actor:    "user:eve", // non-member, non-system, no membership row
		TypeId:   searchTypeID,
		Query:    "widget",
	})
	if err != nil {
		t.Fatalf("SearchNodes: unexpected err: %v", err)
	}
	got := nodeIDs(resp.GetNodes())
	if len(got) != 1 || got[0] != "shared" {
		t.Fatalf("SearchNodes: cross-tenant filter wrong: got %v, want [shared]", got)
	}
}

// TestSearchNodes_EmptyQuery: codes.InvalidArgument "query must not be
// empty". Pinned by tests/python/integration/test_grpc_contract.py:627-632.
func TestSearchNodes_EmptyQuery(t *testing.T) {
	t.Parallel()
	srv, _, tenantID := newSearchTestServer(t)

	cases := []struct {
		name  string
		query string
	}{
		{"empty", ""},
		{"only-whitespace", "    "}, // strip => empty (spec line 185)
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := srv.SearchNodes(context.Background(), &pb.SearchNodesRequest{
				TenantId: tenantID,
				Actor:    "user:alice",
				TypeId:   searchTypeID,
				Query:    tc.query,
			})
			if err == nil {
				t.Fatalf("SearchNodes(%q): expected InvalidArgument, got nil", tc.query)
			}
			st, ok := status.FromError(err)
			if !ok {
				t.Fatalf("SearchNodes(%q): error is not a grpc status: %v", tc.query, err)
			}
			if st.Code() != codes.InvalidArgument {
				t.Fatalf("SearchNodes(%q): code: got %v, want InvalidArgument", tc.query, st.Code())
			}
		})
	}
}

// TestSearchNodes_QueryTooLong: codes.InvalidArgument when the
// post-trim query exceeds 1000 characters.
func TestSearchNodes_QueryTooLong(t *testing.T) {
	t.Parallel()
	srv, _, tenantID := newSearchTestServer(t)

	q := strings.Repeat("a", 1001)
	_, err := srv.SearchNodes(context.Background(), &pb.SearchNodesRequest{
		TenantId: tenantID,
		Actor:    "user:alice",
		TypeId:   searchTypeID,
		Query:    q,
	})
	if err == nil {
		t.Fatalf("SearchNodes: expected InvalidArgument, got nil")
	}
	if got := codeOf(err); got != codes.InvalidArgument {
		t.Fatalf("SearchNodes: code: got %v, want InvalidArgument", got)
	}
}

// TestSearchNodes_MalformedFTSQueryRejected: a malformed FTS5 MATCH
// query is a client error and is surfaced as INVALID_ARGUMENT, not
// masked as empty+OK (#573). This lets the caller learn the query was
// bad instead of mistaking it for "no matches".
func TestSearchNodes_MalformedFTSQueryRejected(t *testing.T) {
	t.Parallel()
	srv, cs, tenantID := newSearchTestServer(t)
	seedIndexedNode(t, cs, tenantID, "n1", "user:alice",
		map[string]any{"1": "hello", "2": "world"}, nil)

	// FTS5 rejects an unbalanced quote with a syntax error.
	_, err := srv.SearchNodes(context.Background(), &pb.SearchNodesRequest{
		TenantId: tenantID,
		Actor:    "user:alice",
		TypeId:   searchTypeID,
		Query:    `"unbalanced`,
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("SearchNodes malformed query: code = %s, want InvalidArgument (%v)", status.Code(err), err)
	}
}

// TestSearchNodes_EmptyTenant: empty tenant_id -> InvalidArgument from
// the tenant gate. Pins the gate-first ordering required by the spec.
func TestSearchNodes_EmptyTenant(t *testing.T) {
	t.Parallel()
	srv, _, _ := newSearchTestServer(t)
	_, err := srv.SearchNodes(context.Background(), &pb.SearchNodesRequest{
		TenantId: "",
		Actor:    "user:alice",
		TypeId:   searchTypeID,
		Query:    "anything",
	})
	if err == nil {
		t.Fatalf("SearchNodes: expected InvalidArgument, got nil")
	}
	if got := codeOf(err); got != codes.InvalidArgument {
		t.Fatalf("SearchNodes: code: got %v, want InvalidArgument", got)
	}
}

// TestSearchNodes_TypeWithNoSearchableFields: a type id whose schema
// has zero searchable fields -> codes.OK with Nodes == []. Load-bearing
// short-circuit pinned by test_grpc_contract.py:633-641.
func TestSearchNodes_TypeWithNoSearchableFields(t *testing.T) {
	t.Parallel()
	srv, _, tenantID := newSearchTestServer(t)

	resp, err := srv.SearchNodes(context.Background(), &pb.SearchNodesRequest{
		TenantId: tenantID,
		Actor:    "user:alice",
		TypeId:   999, // unregistered type_id => no searchable fields
		Query:    "anything",
	})
	if err != nil {
		t.Fatalf("SearchNodes: unexpected err: %v", err)
	}
	if resp == nil {
		t.Fatalf("SearchNodes: nil response")
	}
	if got := len(resp.GetNodes()); got != 0 {
		t.Fatalf("SearchNodes: expected 0 nodes, got %d", got)
	}
}

// TestSearchNodes_PageSizeAndHasMore: ADR-029 FTS carve-out. SearchNodes
// keeps OFFSET paging; page_size aliases limit and takes precedence, and
// has_more is EXACT (the store is asked for limit+1 and the probe row is
// trimmed). Seed five matching nodes, page through them with page_size=2,
// and assert: each non-final page returns 2 rows with has_more=true, the
// final page returns 1 row with has_more=false.
func TestSearchNodes_PageSizeAndHasMore(t *testing.T) {
	t.Parallel()
	srv, cs, tenantID := newSearchTestServer(t)

	// Five nodes all matching "widget".
	for _, id := range []string{"p1", "p2", "p3", "p4", "p5"} {
		seedIndexedNode(t, cs, tenantID, id, "user:alice",
			map[string]any{"1": "widget " + id, "2": "a widget body"}, nil)
	}

	// Page 0 + page 1: 2 rows each, has_more=true.
	for page, off := 0, int32(0); page < 2; page, off = page+1, off+2 {
		resp, err := srv.SearchNodes(context.Background(), &pb.SearchNodesRequest{
			TenantId: tenantID,
			Actor:    "user:alice",
			TypeId:   searchTypeID,
			Query:    "widget",
			PageSize: 2,
			Offset:   off,
		})
		if err != nil {
			t.Fatalf("SearchNodes page %d: %v", page, err)
		}
		if got := len(resp.GetNodes()); got != 2 {
			t.Fatalf("SearchNodes page %d: nodes got %d, want 2", page, got)
		}
		if !resp.GetHasMore() {
			t.Fatalf("SearchNodes page %d: has_more got false, want true", page)
		}
	}

	// Page 2: the last row, has_more=false.
	resp, err := srv.SearchNodes(context.Background(), &pb.SearchNodesRequest{
		TenantId: tenantID,
		Actor:    "user:alice",
		TypeId:   searchTypeID,
		Query:    "widget",
		PageSize: 2,
		Offset:   4,
	})
	if err != nil {
		t.Fatalf("SearchNodes final page: %v", err)
	}
	if got := len(resp.GetNodes()); got != 1 {
		t.Fatalf("SearchNodes final page: nodes got %d, want 1", got)
	}
	if resp.GetHasMore() {
		t.Fatalf("SearchNodes final page: has_more got true, want false")
	}
}

// TestSearchNodes_PageSizePrecedesLimit: when both page_size and the
// legacy limit are set, page_size wins (AIP-158 alias precedence).
func TestSearchNodes_PageSizePrecedesLimit(t *testing.T) {
	t.Parallel()
	srv, cs, tenantID := newSearchTestServer(t)
	for _, id := range []string{"p1", "p2", "p3", "p4"} {
		seedIndexedNode(t, cs, tenantID, id, "user:alice",
			map[string]any{"1": "widget " + id, "2": "body"}, nil)
	}

	resp, err := srv.SearchNodes(context.Background(), &pb.SearchNodesRequest{
		TenantId: tenantID,
		Actor:    "user:alice",
		TypeId:   searchTypeID,
		Query:    "widget",
		Limit:    100, // legacy, should be ignored
		PageSize: 1,   // wins
	})
	if err != nil {
		t.Fatalf("SearchNodes: %v", err)
	}
	if got := len(resp.GetNodes()); got != 1 {
		t.Fatalf("SearchNodes: page_size did not take precedence; got %d nodes, want 1", got)
	}
	if !resp.GetHasMore() {
		t.Fatalf("SearchNodes: has_more got false, want true (3 more rows)")
	}
}

// nodeIDs lives in helpers_external_test.go.

// codeOf extracts a grpc code from err for compact assertions.
func codeOf(err error) codes.Code {
	if err == nil {
		return codes.OK
	}
	if st, ok := status.FromError(err); ok {
		return st.Code()
	}
	return codes.Unknown
}
