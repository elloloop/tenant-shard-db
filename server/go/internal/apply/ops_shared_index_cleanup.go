// SPDX-License-Identifier: AGPL-3.0-only

package apply

import (
	"fmt"
)

// applySharedIndexCleanup records handler-derived shared_index cleanup
// work for post-commit fanout. The global projection store is updated
// after the tenant transaction commits; projection failures remain
// best-effort and do not poison the tenant apply path.
//
// op shape:
//
//	{
//	  "op": "shared_index_cleanup",
//	  "tenant_id": "tenant_a",
//	  "user_id": "user:bob",
//	  "node_ids": ["n1", "n2"]
//	}
func (a *Applier) applySharedIndexCleanup(ev *Event, op map[string]any, res *Result) error {
	tenantID := stringField(op, "tenant_id")
	if tenantID == "" {
		tenantID = ev.TenantID
	}
	userID := stringField(op, "user_id")
	if userID == "" {
		userID = stringField(op, "member_actor_id")
	}
	if tenantID == "" || userID == "" {
		return fmt.Errorf("%w: shared_index_cleanup missing tenant_id/user_id", ErrPoisonEvent)
	}
	nodeIDs := stringSliceField(op, "node_ids")
	if len(nodeIDs) == 0 {
		return nil
	}
	for _, nodeID := range nodeIDs {
		if nodeID == "" {
			continue
		}
		res.SharedRemoved = append(res.SharedRemoved, SharedRef{
			UserID:       userID,
			SourceTenant: tenantID,
			NodeID:       nodeID,
		})
	}
	return nil
}

func stringSliceField(op map[string]any, key string) []string {
	out := []string{}
	switch raw := op[key].(type) {
	case []any:
		for _, v := range raw {
			if s, ok := v.(string); ok {
				out = append(out, s)
			}
		}
	case []string:
		out = append(out, raw...)
	}
	return out
}
