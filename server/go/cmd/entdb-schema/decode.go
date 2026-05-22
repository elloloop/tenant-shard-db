// Snapshot file decoding.
//
// Accepts both wire shapes:
//   1. Envelope: {"version": 1, "fingerprint": "sha256:…", "schema": {...}}
//      — the shape `entdb-schema snapshot` writes by default.
//   2. Bare body: {"node_types": [...], "edge_types": [...]}
//      — what `(*Registry).MarshalJSON` emits directly.
//
// Snapshot files written by `entdb-schema snapshot` use shape (1). The
// server's `GetSchema` RPC returns shape (2). Both must round-trip
// through this decoder for the migration story in §6 of the issue to
// hold.

package main

import (
	"encoding/json"
	"errors"
)

// supportedSnapshotVersion is the on-disk envelope version this build
// of entdb-schema produces and consumes. Bumping requires a CLI flag
// and a migration note (see issue #488, open question #8).
const supportedSnapshotVersion = 1

// decodeSnapshot returns the inner schema body bytes (always the
// canonical `{"node_types": [...], "edge_types": [...]}` shape that
// schema.LoadFromJSON consumes) and the fingerprint from the envelope
// if any.
//
// The detector inspects top-level keys rather than trying to
// json.Unmarshal-into-typed-struct twice: envelope-shaped files have a
// "schema" key, bare bodies have "node_types".
func decodeSnapshot(data []byte) (bodyBytes []byte, fingerprint string, err error) {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, "", err
	}
	if _, ok := probe["schema"]; ok {
		// Envelope shape.
		var env struct {
			Version     int             `json:"version"`
			Fingerprint string          `json:"fingerprint"`
			Schema      json.RawMessage `json:"schema"`
		}
		if err := json.Unmarshal(data, &env); err != nil {
			return nil, "", err
		}
		if env.Version != 0 && env.Version != supportedSnapshotVersion {
			return nil, "", errors.New(
				"unsupported snapshot version (only v1 is supported by this build)",
			)
		}
		if len(env.Schema) == 0 {
			return nil, "", errors.New("snapshot envelope is missing the \"schema\" body")
		}
		return env.Schema, env.Fingerprint, nil
	}
	if _, ok := probe["node_types"]; ok {
		// Bare-body shape — pass through unchanged.
		return data, "", nil
	}
	return nil, "", errors.New(
		"unrecognised snapshot shape: expected either an envelope with " +
			"a top-level \"schema\" key or a bare body with \"node_types\"",
	)
}
