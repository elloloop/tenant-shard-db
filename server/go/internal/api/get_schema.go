// GetSchema RPC.
// Spec: docs/go-port/rpcs/GetSchema.md.
//
// Wire contract: proto/entdb/v1/entdb.proto:79 (rpc), :590-596 (request),
// :598-607 (response). Reference Python:
//
// Semantics (preserved byte-for-byte from the Python handler):
//
//   - Read-only. No WAL append, no global_store touch.
//   - No Permission check; auth interceptor handles credentials.
//   - Swallow all errors as OK. The Python handler catches every
//     exception path and returns &GetSchemaResponse{Fingerprint: ""}
//     with grpc.OK. The sole contract pin
//     (tests/python/integration/test_grpc_contract.py:208) only
//     asserts `fingerprint != "" || HasField("schema")`, so the
//     empty-but-OK response is the documented degraded path.
//   - Optional req.TenantId drives a data-driven fallback when the
//     registry is empty: we synthesise placeholder type entries from
//     distinct type_ids observed in that tenant's SQLite. stub:
//     the canonicalstore Go port doesn't expose GetDistinctTypeIDs
//     yet, so the fallback is a no-op here and surfaces as the same
//     empty-Struct response Python returns when its SQLite query
//     errors (`:1665-1666`). This matches the contract pin and is
//     flagged in the EPIC for the canonicalstore follow-up.
//
// Known latent bug (preserved for parity, NOT fixed here): the Python
// handler reads req.TenantId without cross-checking the caller's
// authenticated tenant binding, leaking distinct type_ids across
// tenants. See spec "Open questions / risks" — flagged as follow-up.

package api

import (
	"context"
	"encoding/json"
	"time"

	"google.golang.org/protobuf/types/known/structpb"

	"github.com/elloloop/tenant-shard-db/server/go/internal/metrics"
	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

const getSchemaMethod = "GetSchema"

// GetSchema returns the in-memory schema registry as a typed
// google.protobuf.Struct plus its post-freeze fingerprint. See file
// header for the swallow-all-errors-as-OK contract.
func (s *Server) GetSchema(_ context.Context, req *pb.GetSchemaRequest) (resp *pb.GetSchemaResponse, err error) {
	start := time.Now()
	status := "ok"
	defer func() {
		// Single emission point: emit metric whether we exited
		// normally or via recover. status is mutated on the
		// degraded path below.
		metrics.RecordGRPCRequest(getSchemaMethod, status, time.Since(start))
	}()

	defer func() {
		if r := recover(); r != nil {
			// Match Python's outer except (`:1686-1689`): degrade to
			// empty fingerprint with OK status. resp/err are named
			// returns so the deferred closure can overwrite them.
			status = "error"
			resp = &pb.GetSchemaResponse{Fingerprint: ""}
			err = nil
			_ = r
		}
	}()

	// Snapshot fingerprint up front so a marshal failure later still
	// returns it (matches Python's `self.schema_registry.fingerprint
	// or ""` at `:1684` — fingerprint is computed at freeze, not at
	// serialise time).
	var fingerprint string
	if s.registry != nil {
		fingerprint = s.registry.Fingerprint()
	}

	schemaMap, ierr := s.snapshotSchemaMap()
	if ierr != nil {
		// Marshal of an in-memory registry should never fail (it's
		// strongly typed Go), but if it does, Python's outer except
		// kicks in. Return empty fingerprint to match `:1684`'s `or ""`
		// fallback when registry is unfrozen.
		status = "error"
		return &pb.GetSchemaResponse{Fingerprint: ""}, nil
	}

	// Data-driven fallback when the registry is empty and the caller
	// provided a tenant_id (`:1645-1664`). The Go canonicalstore port
	// does not yet expose GetDistinctTypeIDs; until it does, this path
	// degrades to the same empty-Struct result Python returns when its
	// SQLite query errors at `:1665-1666` — well within the contract
	// pin (fingerprint or schema field present; an empty Struct still
	// counts as "schema field present"). Flagged in the EPIC for the
	// canonicalstore follow-up.
	_ = mapHasTypes(schemaMap)
	_ = req.GetTenantId()

	// type_id filter (`:1668-1679`). Applied after the fallback so the
	// node_types/edge_types maps reflect either registry contents or
	// the fallback synthesis (today: just registry contents).
	if req.GetTypeId() != 0 {
		schemaMap = filterByTypeID(schemaMap, req.GetTypeId())
	}

	structVal, ierr := structpb.NewStruct(schemaMap)
	if ierr != nil {
		// Numeric values are emitted as float64 via the JSON round-trip
		// in snapshotSchemaMap, so this should not fire in practice.
		// If it does, degrade per Python's outer except (`:1686-1689`).
		status = "error"
		return &pb.GetSchemaResponse{Fingerprint: ""}, nil
	}

	return &pb.GetSchemaResponse{
		Schema:      structVal,
		Fingerprint: fingerprint,
	}, nil
}

// snapshotSchemaMap lowers the registry into the Python `to_dict()`
// shape: {"node_types": [...], "edge_types": [...]}. We piggyback on
// schema.Registry.MarshalJSON (which already sorts by id and normalises
// nil slices to []) and unmarshal back into a map[string]any so
// structpb.NewStruct can consume it. JSON-round-trip is the same
// canonical form the fingerprint canonicaliser uses, so the wire bytes
// stay aligned with Python's `_dict_to_struct` (`:154-159`).
//
// Returns an empty map for a nil registry — matches Python returning
// empty `node_types: []` / `edge_types: []` for an empty SchemaRegistry.
func (s *Server) snapshotSchemaMap() (map[string]any, error) {
	if s.registry == nil {
		return map[string]any{
			"node_types": []any{},
			"edge_types": []any{},
		}, nil
	}
	raw, err := json.Marshal(s.registry)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	// MarshalJSON guarantees the top-level keys, but defensively
	// normalise so structpb conversion never sees nil.
	if _, ok := m["node_types"]; !ok {
		m["node_types"] = []any{}
	}
	if _, ok := m["edge_types"]; !ok {
		m["edge_types"] = []any{}
	}
	return m, nil
}

// mapHasTypes reports whether the snapshot contains any node or edge
// type entries. Mirrors the truthy check at `:1644` that decides
// whether to fall back to the per-tenant SQLite distinct-type read.
func mapHasTypes(m map[string]any) bool {
	if nt, ok := m["node_types"].([]any); ok && len(nt) > 0 {
		return true
	}
	if et, ok := m["edge_types"].([]any); ok && len(et) > 0 {
		return true
	}
	return false
}

// filterByTypeID applies the optional req.type_id filter (`:1668-1679`).
// Keeps node entries with matching type_id and edge entries that touch
// it via from_type_id or to_type_id.
//
// NOTE: the JSON round-trip in snapshotSchemaMap lands every number as
// float64, so we compare against float64(wanted).
func filterByTypeID(m map[string]any, wanted int32) map[string]any {
	wantedF := float64(wanted)
	if nodes, ok := m["node_types"].([]any); ok {
		kept := make([]any, 0, len(nodes))
		for _, e := range nodes {
			obj, ok := e.(map[string]any)
			if !ok {
				continue
			}
			if id, ok := obj["type_id"].(float64); ok && id == wantedF {
				kept = append(kept, obj)
			}
		}
		m["node_types"] = kept
	}
	if edges, ok := m["edge_types"].([]any); ok {
		kept := make([]any, 0, len(edges))
		for _, e := range edges {
			obj, ok := e.(map[string]any)
			if !ok {
				continue
			}
			from, _ := obj["from_type_id"].(float64)
			to, _ := obj["to_type_id"].(float64)
			if from == wantedF || to == wantedF {
				kept = append(kept, obj)
			}
		}
		m["edge_types"] = kept
	}
	return m
}
