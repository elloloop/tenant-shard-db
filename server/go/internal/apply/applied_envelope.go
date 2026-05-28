// SPDX-License-Identifier: AGPL-3.0-only

package apply

import (
	"encoding/json"
)

// AppliedResultEnvelope is the shape persisted into applied_events
// .failure_json on a v2.2 APPLIED record that involved at least one
// CreateNodeOp with on_conflict=SKIP (issue #599 single-RTT
// InsertIfNotExists). The handler reads this from the idempotency
// cache to populate ExecuteAtomicResponse.{created_node_ids,
// existing_node_ids}, replacing the pre-minted UUIDs it used as
// placeholders. Empty failure_json on APPLIED is the legacy "no SKIP
// happened" signal — the handler keeps using pre-minted UUIDs.
//
// The arrays are index-aligned (one slot per create_node op in the
// batch). At any given index i exactly one is non-empty:
//
//	CreatedNodes[i]  ExistingNodes[i]  meaning
//	"new-id"         ""                fresh create
//	""               "existing-id"     SKIP swallowed a unique violation
//
// Tag names match the wire field names so a future migration that
// stores the envelope in a dedicated column reads back as-is.
type AppliedResultEnvelope struct {
	CreatedNodeIDs  []string `json:"created_node_ids,omitempty"`
	ExistingNodeIDs []string `json:"existing_node_ids,omitempty"`
}

// encodeAppliedResultEnvelope serialises the (CreatedNodes,
// ExistingNodes) arrays into the failure_json string. Returns "" when
// no SKIP happened — the legacy APPLIED row shape — so a pre-v2.2
// applier replay against a v2.2 cache reads back as before. Any
// non-empty entry in ExistingNodes triggers the envelope (a batch can
// have SKIPs that resolved a node AND SKIPs that successfully created;
// the envelope must capture both sides to preserve index alignment on
// the wire).
func encodeAppliedResultEnvelope(res *Result) string {
	hasSkip := false
	for _, id := range res.ExistingNodes {
		if id != "" {
			hasSkip = true
			break
		}
	}
	if !hasSkip {
		return ""
	}
	env := AppliedResultEnvelope{
		CreatedNodeIDs:  res.CreatedNodes,
		ExistingNodeIDs: res.ExistingNodes,
	}
	b, err := json.Marshal(env)
	if err != nil {
		// json.Marshal on string slices can't realistically fail;
		// degrade to empty envelope so the row still records APPLIED.
		return ""
	}
	return string(b)
}

// DecodeAppliedResultEnvelope is the read side. Returns ok=false on
// empty / malformed input so the caller falls back to the legacy
// pre-minted-UUID path.
func DecodeAppliedResultEnvelope(s string) (AppliedResultEnvelope, bool) {
	if s == "" {
		return AppliedResultEnvelope{}, false
	}
	var env AppliedResultEnvelope
	if err := json.Unmarshal([]byte(s), &env); err != nil {
		return AppliedResultEnvelope{}, false
	}
	if len(env.CreatedNodeIDs) == 0 && len(env.ExistingNodeIDs) == 0 {
		return AppliedResultEnvelope{}, false
	}
	return env, true
}
