// SPDX-License-Identifier: AGPL-3.0-only

package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/elloloop/tenant-shard-db/server/go/internal/schema"
)

// Catalog "kind" discriminators stored in schema_catalog.kind.
const (
	CatalogKindNode = "node"
	CatalogKindEdge = "edge"
)

// UpsertCatalogTx writes one node/edge type definition into the tenant's
// schema_catalog ON the open BatchTxn's connection, so it commits (or
// rolls back) atomically with the register_schema op, its indexes, and
// the applied_events / applied_offsets rows. def_json is the per-type
// canonical JSON (see ops_register_schema.go); an UPSERT against an
// unchanged definition is a no-op (ADR-035 / #626).
func (s *CanonicalStore) UpsertCatalogTx(ctx context.Context, tx *BatchTxn, kind string, typeID int32, defJSON []byte) error {
	if tx == nil {
		return fmt.Errorf("store: UpsertCatalogTx: nil tx")
	}
	_, err := tx.Conn().ExecContext(ctx,
		`INSERT INTO schema_catalog (kind, type_id, def_json, updated_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(kind, type_id) DO UPDATE SET
		     def_json   = excluded.def_json,
		     updated_at = excluded.updated_at`,
		kind, typeID, string(defJSON), s.now(),
	)
	if err != nil {
		return fmt.Errorf("store: upsert schema_catalog %s/%d: %w", kind, typeID, err)
	}
	return nil
}

// LoadCatalogInto replays a tenant's persisted schema_catalog into reg
// via RegisterOrVerifyNode/Edge (establish-or-reject, idempotent). It is
// how the process-global registry is repopulated after a restart that
// did not replay the WAL — the fix for the registry booting empty
// (ADR-035 / #624). A nil reg is a no-op. Node types are loaded before
// edge types so edge cross-references resolve.
//
// The tenant must already be open (callers go through OpenTenant /
// dbAuto, which open then load).
func (s *CanonicalStore) LoadCatalogInto(ctx context.Context, tenantID string, reg *schema.Registry) error {
	if reg == nil {
		return nil
	}
	// Pure SELECT — use the read handle so it never contends for the
	// single write connection (read/write split, issue #137).
	db, err := s.readDB(tenantID)
	if err != nil {
		return err
	}
	rows, err := db.QueryContext(ctx,
		`SELECT kind, def_json FROM schema_catalog
		 ORDER BY CASE kind WHEN 'node' THEN 0 ELSE 1 END, type_id`)
	if err != nil {
		return fmt.Errorf("store: query schema_catalog: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var kind, defJSON string
		if err := rows.Scan(&kind, &defJSON); err != nil {
			return fmt.Errorf("store: scan schema_catalog: %w", err)
		}
		if err := registerCatalogRow(reg, kind, []byte(defJSON)); err != nil {
			return err
		}
	}
	return rows.Err()
}

func registerCatalogRow(reg *schema.Registry, kind string, defJSON []byte) error {
	switch kind {
	case CatalogKindNode:
		var nt schema.NodeTypeDef
		if err := json.Unmarshal(defJSON, &nt); err != nil {
			return fmt.Errorf("store: decode catalog node: %w", err)
		}
		if _, err := reg.RegisterOrVerifyNode(&nt); err != nil {
			return fmt.Errorf("store: load catalog node type_id %d: %w", nt.TypeID, err)
		}
	case CatalogKindEdge:
		var et schema.EdgeTypeDef
		if err := json.Unmarshal(defJSON, &et); err != nil {
			return fmt.Errorf("store: decode catalog edge: %w", err)
		}
		if _, err := reg.RegisterOrVerifyEdge(&et); err != nil {
			return fmt.Errorf("store: load catalog edge_id %d: %w", et.EdgeID, err)
		}
	default:
		return fmt.Errorf("store: unknown schema_catalog kind %q", kind)
	}
	return nil
}

// MarshalCatalogDef renders a node/edge type definition to the canonical
// JSON stored in schema_catalog. It is exported so the applier writes
// exactly the bytes LoadCatalogInto round-trips. v must be a
// *schema.NodeTypeDef or *schema.EdgeTypeDef.
func MarshalCatalogDef(v any) ([]byte, error) {
	switch v.(type) {
	case *schema.NodeTypeDef, *schema.EdgeTypeDef:
		b, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("store: marshal catalog def: %w", err)
		}
		return b, nil
	default:
		return nil, fmt.Errorf("store: MarshalCatalogDef: unsupported type %T", v)
	}
}
