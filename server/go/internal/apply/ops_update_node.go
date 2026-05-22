// SPDX-License-Identifier: AGPL-3.0-only

package apply

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
)

// applyUpdateNode dispatches an "update_node" op. The patch is field-id-keyed
// and merged onto the existing payload by string key (rename-free per CLAUDE.md
// invariant #6). storage_mode is immutable — any attempt to set it is a
// poison-event.
//
// GitHub issue #500 (CAS): when the op carries a "precondition" map
// (handler ingress translates “UpdateNodeOp.precondition.field“ to a
// stringified “field_id“ and carries the value through under
// “equals“), the applier compares observed-vs-expected BEFORE the
// patch merge. A mismatch returns a *PreconditionFailure that unwraps
// to ErrPreconditionFailed; the applier's per-event loop catches that
// sentinel, rolls back the batch, and memoizes the failure in the
// idempotency cache so retries replay the same typed error.
//
// opIndex is the zero-based position of this op inside ev.Ops — set by
// the dispatcher so the typed failure can carry it through to the SDK
// without re-deriving it here.
func (a *Applier) applyUpdateNode(ctx context.Context, tx *BatchTxn, ev *Event, op map[string]any, aliases nodeAliasMap, opIndex int) error {
	patch := mapField(op, "patch")
	if patch == nil {
		patch = map[string]any{}
	}
	if _, ok := patch["storage_mode"]; ok {
		return fmt.Errorf("%w: storage_mode is immutable", ErrPoisonEvent)
	}
	nodeID := aliases.resolveRef(stringField(op, "id"))
	if nodeID == "" {
		return fmt.Errorf("%w: update_node missing id", ErrPoisonEvent)
	}

	conn := tx.Conn()
	row := conn.QueryRowContext(ctx,
		`SELECT payload_json FROM nodes WHERE tenant_id = ? AND node_id = ?`,
		ev.TenantID, nodeID,
	)
	var existingJSON string
	if err := row.Scan(&existingJSON); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Missing target is a no-op.
			// A precondition on a missing node is treated as a miss
			// (observed=null, field_present=false) to keep the CAS
			// surface honest — see GitHub issue #500.
			if pre := mapField(op, "precondition"); pre != nil {
				return preconditionFailureFor(pre, nil, false, opIndex)
			}
			return nil
		}
		return fmt.Errorf("apply update_node: read existing: %w", err)
	}
	merged := map[string]any{}
	if existingJSON != "" {
		if err := json.Unmarshal([]byte(existingJSON), &merged); err != nil {
			return fmt.Errorf("apply update_node: parse existing payload: %w", err)
		}
	}

	// Precondition CHECK happens against materialised state (the
	// `merged` map post-load, pre-merge). The check runs inside the
	// same BatchTxn as the rest of the ops so any prior op in the
	// batch that touched the same node is visible — without that,
	// CAS over a sequence of "set X then conditionally do Y based on
	// X" reads stale state.
	if pre := mapField(op, "precondition"); pre != nil {
		observed, present := preconditionObserved(pre, merged)
		if !preconditionMatches(pre, observed, present) {
			return preconditionFailureFor(pre, observed, present, opIndex)
		}
	}

	for k, v := range patch {
		merged[k] = v
	}
	mergedJSON, err := json.Marshal(merged)
	if err != nil {
		return fmt.Errorf("apply update_node: marshal merged: %w", err)
	}
	now := ev.TsMs
	if now == 0 {
		now = a.now()
	}
	if _, err := conn.ExecContext(ctx,
		`UPDATE nodes SET payload_json = ?, updated_at = ? WHERE tenant_id = ? AND node_id = ?`,
		string(mergedJSON), now, ev.TenantID, nodeID,
	); err != nil {
		return fmt.Errorf("apply update_node: update: %w", err)
	}
	return nil
}

// preconditionFieldKey extracts the on-disk field-id key carried by the
// handler-translated precondition. Handlers populate "field_id" with
// the numeric id of the named precondition field; "field" carries the
// human-readable name purely for error reporting and is also set so
// the typed failure can surface it to the SDK without consulting the
// schema registry at apply time. Returns ("", "") if either is
// missing/wrongly typed — caller treats as a structural poison.
func preconditionFieldKey(pre map[string]any) (key string, name string) {
	if pre == nil {
		return "", ""
	}
	switch v := pre["field_id"].(type) {
	case string:
		key = v
	case float64:
		key = fmt.Sprintf("%d", int64(v))
	case int:
		key = fmt.Sprintf("%d", v)
	case int32:
		key = fmt.Sprintf("%d", v)
	case int64:
		key = fmt.Sprintf("%d", v)
	}
	name = stringField(pre, "field")
	return key, name
}

// preconditionObserved reads the observed value for the precondition's
// field out of the merged-but-not-yet-patched payload map. Returns
// (value, present) where present is false when the payload had no key
// for the resolved field_id (distinct from a JSON-null value).
func preconditionObserved(pre map[string]any, merged map[string]any) (any, bool) {
	key, _ := preconditionFieldKey(pre)
	if key == "" {
		return nil, false
	}
	v, ok := merged[key]
	return v, ok
}

// preconditionMatches compares observed vs the "equals" payload. JSON
// numeric equality is the tricky case: the patch round-trips through
// the WAL as float64, while the payload-on-disk also round-trips as
// float64, so reflect.DeepEqual is correct for scalars. Maps / lists
// are compared structurally — callers that need richer semantics
// (e.g. ordering-insensitive list compare) should not lean on v1 CAS.
//
// A precondition referring to an absent field matches ONLY when the
// expected value is nil/JSON-null AND the field is genuinely missing;
// any other shape is a mismatch.
func preconditionMatches(pre map[string]any, observed any, present bool) bool {
	expected, _ := pre["equals"]
	if !present {
		return expected == nil
	}
	return reflect.DeepEqual(observed, expected)
}

// preconditionFailureFor constructs the typed precondition-failure
// returned to the dispatcher.
func preconditionFailureFor(pre map[string]any, observed any, present bool, opIndex int) *PreconditionFailure {
	_, name := preconditionFieldKey(pre)
	expected, _ := pre["equals"]
	return &PreconditionFailure{
		OpIndex:      opIndex,
		Field:        name,
		Expected:     expected,
		Observed:     observed,
		FieldPresent: present,
	}
}
