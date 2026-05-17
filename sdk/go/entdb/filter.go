// SPDX-License-Identifier: MIT
package entdb

// FilterOp is the comparison operator carried by a [Filter]. The
// values mirror the wire FilterOp enum's Eq/Ne/Lt/Le/Gt/Ge subset
// (see proto/entdb/v1/entdb.proto). Issue #501 scopes the SDK to
// the six AND-able comparison operators; CONTAINS / IN remain
// server-side TBD and are not exposed here.
type FilterOp string

// Comparison operators accepted by [Filter]. Each compiles to the
// matching SQL operator on the indexed payload column.
//
// Index usage caveat:
//   - FilterEq/Lt/Le/Gt/Ge use the existing “idx_query_t<type>_f<field>“
//     expression index for B-tree lookups.
//   - FilterNe (not-equal) CANNOT use a B-tree index — it forces a
//     scan over every row of the requested type. Use sparingly on
//     large tables; prefer a positive predicate (“> v OR < v“ is
//     not expressible in this v1 cut — re-issue the query if you
//     need both branches).
const (
	FilterEq FilterOp = "eq"
	FilterNe FilterOp = "ne"
	FilterLt FilterOp = "lt"
	FilterLe FilterOp = "le"
	FilterGt FilterOp = "gt"
	FilterGe FilterOp = "ge"
)

// Filter is one AND-ed predicate passed to [QueryWhere] (and
// [DeleteWhere]). Field is the proto field name (the gRPC boundary
// resolves it to a payload field_id). Op selects the comparison
// operator. Value is bound as a SQLite parameter and supports the
// JSON-marshallable scalars SQLite understands (string, int, int64,
// float64, bool, nil).
//
// Schema-less escape hatch (issue #545): Field may instead be the
// digit-only numeric payload field id (e.g. "4"). The SDK forwards
// Field unchanged; a server with no schema treats a digit-only key as
// a raw field id and skips name resolution. This is the only way to
// filter against a schema-less server — a field NAME there returns
// INVALID_ARGUMENT ("cannot translate filter key … without a
// schema"). A schema-configured server accepts either form.
//
// Multiple filters on the same field are permitted — a half-open
// range is expressed as two filters:
//
//	[]Filter{
//	    {Field: "expires_at", Op: FilterGe, Value: lo},
//	    {Field: "expires_at", Op: FilterLt, Value: hi},
//	}
type Filter struct {
	Field string
	Op    FilterOp
	Value any
}

// mongoKey returns the “$op“ form used by the existing
// transport-level filter-to-proto encoder.
func (op FilterOp) mongoKey() string {
	switch op {
	case FilterNe:
		return "$ne"
	case FilterLt:
		return "$lt"
	case FilterLe:
		return "$lte"
	case FilterGt:
		return "$gt"
	case FilterGe:
		return "$gte"
	default:
		return "$eq"
	}
}

// filtersToMap converts a typed Filter slice into the MongoDB-style
// map shape the existing transport.QueryNodes accepts. Multiple
// filters on the same field are merged into a single nested
// “{"$op": v, "$op2": v2}“ dict so the existing wire encoder
// (filterToProto) emits one FieldFilter per inlined operator.
func filtersToMap(filters []Filter) map[string]any {
	if len(filters) == 0 {
		return nil
	}
	out := make(map[string]any, len(filters))
	for _, f := range filters {
		if f.Op == FilterEq {
			if _, exists := out[f.Field]; !exists {
				out[f.Field] = f.Value
				continue
			}
		}
		// Promote an existing plain value to a nested op-dict if we
		// already wrote one for this field (e.g. two eq-on-same-field
		// would degenerate; preserve last-write-wins for equality).
		existing, ok := out[f.Field]
		if !ok {
			out[f.Field] = map[string]any{f.Op.mongoKey(): f.Value}
			continue
		}
		sub, ok := existing.(map[string]any)
		if !ok {
			// Existing value is a plain equality; rewrite it as $eq
			// so the new operator nests cleanly alongside.
			sub = map[string]any{"$eq": existing}
		}
		sub[f.Op.mongoKey()] = f.Value
		out[f.Field] = sub
	}
	return out
}
