// GetSchema RPC.
// Spec: docs/go-port/rpcs/GetSchema.md.
//
// Wire contract: proto/entdb/v1/entdb.proto:79 (rpc), :590-596 (request),
// :598-607 (response).
//
// Semantics:
//
//   - Read-only. No WAL append, no global_store touch.
//   - No Permission check; auth interceptor handles credentials.
//   - Swallow all errors as OK: returns &GetSchemaResponse{Fingerprint: ""}
//     with grpc.OK. The sole contract pin
//     (tests/python/integration/test_grpc_contract.py:208) only
//     asserts `fingerprint != "" || HasField("schema")`, so the
//     empty-but-OK response is the documented degraded path.
//   - Pure serializer of the process-wide schema registry. As of
//     ADR-035 that registry is catalog-backed (loaded from each
//     tenant's SQLite schema_catalog on tenant open), so it is durably
//     correct after a restart instead of booting empty. The old
//     empty-registry SQLite fallback (which never had a backing store
//     method) has been removed. The optional req.type_id filter still
//     applies. req.TenantId is currently unused.

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
func (s *Server) GetSchema(ctx context.Context, req *pb.GetSchemaRequest) (resp *pb.GetSchemaResponse, err error) {
	start := time.Now()
	status := "ok"
	defer func() {
		// Single emission point: emit metric whether we exited
		// normally or via recover. status is mutated on the
		// degraded path below.
		metrics.RecordGRPCRequest(ctx, getSchemaMethod, status, time.Since(start))
	}()

	defer func() {
		if r := recover(); r != nil {
			// Degrade to empty fingerprint with OK status. resp/err
			// are named returns so the deferred closure can overwrite
			// them.
			status = "error"
			resp = &pb.GetSchemaResponse{Fingerprint: ""}
			err = nil
			_ = r
		}
	}()

	// Snapshot fingerprint up front so a marshal failure later still
	// returns it. Fingerprint is computed at freeze, not at
	// serialise time.
	var fingerprint string
	if s.registry != nil {
		fingerprint = s.registry.Fingerprint()
	}

	schemaMap, ierr := s.snapshotSchemaMap()
	if ierr != nil {
		// Marshal of an in-memory registry should never fail (it's
		// strongly typed Go), but if it does, degrade to empty
		// fingerprint with OK status.
		status = "error"
		return &pb.GetSchemaResponse{Fingerprint: ""}, nil
	}

	// type_id filter. With ADR-035 the registry is catalog-backed (loaded
	// from per-tenant SQLite on tenant open), so it is durably correct
	// after a restart and GetSchema is a pure serializer of it — the old
	// empty-registry SQLite fallback (which never had a backing store
	// method) is gone.
	if req.GetTypeId() != 0 {
		schemaMap = filterByTypeID(schemaMap, req.GetTypeId())
	}

	structVal, ierr := structpb.NewStruct(schemaMap)
	if ierr != nil {
		// Numeric values are emitted as float64 via the JSON round-trip
		// in snapshotSchemaMap, so this should not fire in practice.
		// If it does, degrade to empty fingerprint with OK status.
		status = "error"
		return &pb.GetSchemaResponse{Fingerprint: ""}, nil
	}

	return &pb.GetSchemaResponse{
		Schema:      structVal,
		Fingerprint: fingerprint,
	}, nil
}

// snapshotSchemaMap lowers the registry into the wire shape:
// {"node_types": [...], "edge_types": [...]}. It piggybacks on
// schema.Registry.MarshalJSON (which already sorts by id and normalises
// nil slices to []) and unmarshals back into a map[string]any so
// structpb.NewStruct can consume it. The JSON round-trip is the same
// canonical form the fingerprint canonicaliser uses.
//
// Returns an empty map for a nil registry.
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

// filterByTypeID applies the optional req.type_id filter.
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
