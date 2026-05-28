// SPDX-License-Identifier: AGPL-3.0-only

package store

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// execContext narrows what we need from a sql.Conn or sql.Tx — only
// the read used by LookupNodeIDByUniqueViolation. Lets the helper run
// inside the applier's per-event txn without re-typing the txn
// boundary all the way down.
type execContext interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// This file owns the translation of a raw SQLite UNIQUE-constraint
// failure into the structured ALREADY_EXISTS detail strings the SDKs
// parse (Go: parseUniqueConstraintFromStatus; Python:
// _parse_unique_constraint_detail / _parse_composite_unique_constraint_detail).
//
// modernc.org/sqlite reports an expression-index UNIQUE violation as:
//
//	UNIQUE constraint failed: index 'idx_unique_t<T>_f<F>'      (single-field)
//	UNIQUE constraint failed: index 'idx_unique_t<T>_c<name>'   (composite)
//
// The index NAME is the source of truth for WHICH declared constraint
// fired, so we parse it rather than guessing from the payload. The
// names are minted by EnsureUniqueIndex / EnsureCompositeUniqueIndex in
// indexes.go and the two must stay in lock-step.

// sqliteIndexNameRE captures the index name out of a modernc.org/sqlite
// UNIQUE-violation error message. The driver wraps the message in its
// own prefix/suffix ("constraint failed: UNIQUE constraint failed:
// index '<name>' (2067)"); we only need the quoted index name.
var sqliteIndexNameRE = regexp.MustCompile(`index '([^']+)'`)

// uniqueViolationParts is the parsed coordinates of a UNIQUE-constraint
// failure, ready to be lowered into the structured ALREADY_EXISTS
// detail. Composite is true when the violated index is a composite
// (multi-field) constraint; for single-field violations FieldIDs holds
// exactly one id. Per ADR-031 a composite constraint is identified by
// its FieldIDs tuple (Constraint carries the tuple signature, e.g.
// "(1,2)", not a name).
type uniqueViolationParts struct {
	Composite  bool
	TypeID     int32
	Constraint string
	FieldIDs   []uint32
}

// parseUniqueIndexName decodes an index name minted by indexes.go back
// into its constraint coordinates. Returns ok=false when the name is
// not one of our unique-index names (e.g. a UNIQUE failure on a
// built-in table constraint such as applied_events).
//
// NAME-FREE per ADR-031: the composite index suffix is a field_id tuple
// "f<id>_<id>_...", so the field_ids are recovered directly from the
// index name. The registry is consulted only to canonicalise the tuple
// to declaration order; if it has no match (e.g. WAL retention dropped
// the register_schema op) the tuple parsed from the index name is used
// as-is.
func (s *CanonicalStore) parseUniqueIndexName(name string) (uniqueViolationParts, bool) {
	// Single-field: idx_unique_t<T>_f<F>
	if m := singleFieldIndexRE.FindStringSubmatch(name); m != nil {
		t, err1 := strconv.Atoi(m[1])
		f, err2 := strconv.Atoi(m[2])
		if err1 == nil && err2 == nil {
			return uniqueViolationParts{
				Composite: false,
				TypeID:    int32(t),
				FieldIDs:  []uint32{uint32(f)},
			}, true
		}
		return uniqueViolationParts{}, false
	}
	// Composite: idx_unique_t<T>_c<suffix> where suffix is "f<id>_<id>_...".
	if m := compositeIndexRE.FindStringSubmatch(name); m != nil {
		t, err := strconv.Atoi(m[1])
		if err != nil {
			return uniqueViolationParts{}, false
		}
		typeID := int32(t)
		suffix := m[2]
		fieldIDs, parsed := parseCompositeSuffix(suffix)
		if !parsed {
			return uniqueViolationParts{}, false
		}
		// Canonicalise against the registry so the tuple is in declared
		// order; fall back to the parsed (sorted-by-suffix) order.
		if s.registry != nil {
			for _, c := range s.registry.CompositeUnique(typeID) {
				if compositeIndexSuffix(c.FieldIDs) == suffix {
					fieldIDs = append([]uint32(nil), c.FieldIDs...)
					break
				}
			}
		}
		return uniqueViolationParts{
			Composite:  true,
			TypeID:     typeID,
			Constraint: signatureOf(fieldIDs),
			FieldIDs:   fieldIDs,
		}, true
	}
	return uniqueViolationParts{}, false
}

// parseCompositeSuffix decodes a composite index suffix
// "f<id>_<id>_..." back into the field_id tuple. Returns ok=false for a
// malformed suffix.
func parseCompositeSuffix(suffix string) ([]uint32, bool) {
	if len(suffix) < 2 || suffix[0] != 'f' {
		return nil, false
	}
	parts := strings.Split(suffix[1:], "_")
	out := make([]uint32, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.ParseUint(p, 10, 32)
		if err != nil {
			return nil, false
		}
		out = append(out, uint32(n))
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

// signatureOf renders a field_id tuple as "(<id>,<id>,...)" in the given
// order — the name-free constraint identity surfaced in the composite
// ALREADY_EXISTS detail (ADR-031).
func signatureOf(ids []uint32) string {
	parts := make([]string, 0, len(ids))
	for _, f := range ids {
		parts = append(parts, strconv.FormatUint(uint64(f), 10))
	}
	return "(" + strings.Join(parts, ",") + ")"
}

var (
	singleFieldIndexRE = regexp.MustCompile(`^idx_unique_t(\d+)_f(\d+)$`)
	compositeIndexRE   = regexp.MustCompile(`^idx_unique_t(\d+)_c(.+)$`)
)

// indexNameFromSQLiteError extracts the quoted index name from a
// modernc.org/sqlite UNIQUE-violation error message. Returns "" when no
// index name is present (e.g. a primary-key collision).
func indexNameFromSQLiteError(err error) string {
	if err == nil {
		return ""
	}
	m := sqliteIndexNameRE.FindStringSubmatch(err.Error())
	if m == nil {
		return ""
	}
	return m[1]
}

// LookupNodeIDByUniqueViolation resolves the colliding row's node_id
// when a CreateNodeOp's INSERT trips a declared unique index and the
// op opted into ``on_conflict = SKIP`` (v2.2 single-RTT
// InsertIfNotExists, issue #599). It reads the violated index name
// out of err to recover the field_id tuple, then projects the
// matching row out of the same transaction's connection — so the
// lookup sees the SAME pre-insert state the INSERT just collided
// against (read-after-write inside the txn, no cross-connection
// race).
//
// Returns ("", false, nil) when the error isn't a recognised
// unique-index violation (i.e. not a SKIP-able situation — the
// caller should fall back to the typed UniqueViolation path).
// Returns ("", false, err) on a genuine query/scan error.
//
// payload is the field-id-keyed map the INSERT carried; its values
// pin down the row the index collided against (the index columns
// are deterministic JSON paths into payload_json, so the colliding
// row's payload has the same scalars at the same field_id keys).
//
// execContext narrows what we need from the txn-bound connection
// without dragging the whole *sql.Tx type through the package
// boundary — the same idiom used by EnsureFieldIndexesTx.
func (s *CanonicalStore) LookupNodeIDByUniqueViolation(
	ctx context.Context,
	exec execContext,
	tenantID string,
	payload map[string]any,
	err error,
) (string, bool, error) {
	idxName := indexNameFromSQLiteError(err)
	if idxName == "" {
		return "", false, nil
	}
	parts, ok := s.parseUniqueIndexName(idxName)
	if !ok {
		return "", false, nil
	}
	// Build the SELECT: one json_extract clause per field_id in the
	// violated index, AND-ed. Single-field is degenerate composite.
	var (
		clauses = make([]string, 0, len(parts.FieldIDs))
		args    = []any{tenantID, parts.TypeID}
	)
	for _, fid := range parts.FieldIDs {
		path := fmt.Sprintf(`$."%d"`, fid)
		clauses = append(clauses, "json_extract(payload_json, ?) = ?")
		args = append(args, path, lookupPayloadValue(payload, fid))
	}
	q := "SELECT node_id FROM nodes WHERE tenant_id = ? AND type_id = ? AND " +
		strings.Join(clauses, " AND ") + " LIMIT 1"
	row := exec.QueryRowContext(ctx, q, args...)
	var existing string
	if scanErr := row.Scan(&existing); scanErr != nil {
		// The row MUST be there — the INSERT just collided with it.
		// A miss here would indicate index/row drift, which is
		// non-recoverable; surface as a real error so the per-event
		// loop falls back to memoising the original UniqueViolation
		// instead of swallowing silently.
		return "", false, fmt.Errorf("LookupNodeIDByUniqueViolation: row missed after %s violation: %w", idxName, scanErr)
	}
	return existing, true, nil
}

// BuildUniqueViolationDetail turns a raw SQLite UNIQUE-constraint error
// into the structured ALREADY_EXISTS detail string the SDKs parse, plus
// a bool reporting whether it recognised the violation as one of our
// declared (single-field or composite) unique constraints.
//
// payload is the field-id-keyed map being written (the colliding row's
// values); tenantID identifies the shard. When the error is not a
// recognised unique-index violation, ok is false and the caller should
// fall back to the generic ErrUniqueConstraint message.
//
// Output formats (verbatim — pinned by the SDK parsers):
//
//	single-field:
//	  Unique constraint violation: tenant=<t> type_id=<T> field_id=<F> value=<repr> already exists
//	composite (name-free, ADR-031 — constraint identity is the field_id tuple):
//	  Composite unique constraint violation: tenant=<t> type_id=<T> constraint='(<F>,...)' fields=[<F>, ...] values=[<repr>, ...] already exists
func (s *CanonicalStore) BuildUniqueViolationDetail(tenantID string, payload map[string]any, err error) (detail string, ok bool) {
	idxName := indexNameFromSQLiteError(err)
	if idxName == "" {
		return "", false
	}
	parts, ok := s.parseUniqueIndexName(idxName)
	if !ok {
		return "", false
	}
	if parts.Composite {
		return formatCompositeUniqueDetail(tenantID, parts, payload), true
	}
	if len(parts.FieldIDs) != 1 {
		return "", false
	}
	fid := parts.FieldIDs[0]
	val := lookupPayloadValue(payload, fid)
	return fmt.Sprintf(
		"Unique constraint violation: tenant=%s type_id=%d field_id=%d value=%s already exists",
		tenantID, parts.TypeID, fid, pyRepr(val),
	), true
}

// formatCompositeUniqueDetail renders the composite ALREADY_EXISTS
// detail. fields/values are rendered as Python-literal lists so the
// SDK's ast.literal_eval / Go regex both parse them.
func formatCompositeUniqueDetail(tenantID string, parts uniqueViolationParts, payload map[string]any) string {
	fieldStrs := make([]string, 0, len(parts.FieldIDs))
	valStrs := make([]string, 0, len(parts.FieldIDs))
	for _, fid := range parts.FieldIDs {
		fieldStrs = append(fieldStrs, strconv.FormatUint(uint64(fid), 10))
		valStrs = append(valStrs, pyRepr(lookupPayloadValue(payload, fid)))
	}
	return fmt.Sprintf(
		"Composite unique constraint violation: tenant=%s type_id=%d constraint=%s fields=[%s] values=[%s] already exists",
		tenantID, parts.TypeID, pyReprString(parts.Constraint),
		strings.Join(fieldStrs, ", "), strings.Join(valStrs, ", "),
	)
}

// lookupPayloadValue fetches the field_id-keyed value out of the
// payload map. Payloads are keyed by the stringified field_id (CLAUDE.md
// invariant #6 / ADR-018). Returns nil when the field is absent.
func lookupPayloadValue(payload map[string]any, fid uint32) any {
	if payload == nil {
		return nil
	}
	return payload[strconv.FormatUint(uint64(fid), 10)]
}

// pyRepr renders v as a Python repr()-compatible literal so the SDK
// parsers can round-trip it via ast.literal_eval. The applier decodes
// payloads with jsonnum so integral values arrive as int64 (ADR-028)
// and survive losslessly here — big ints are NOT collapsed to float
// scientific notation.
func pyRepr(v any) string {
	switch x := v.(type) {
	case nil:
		return "None"
	case string:
		return pyReprString(x)
	case bool:
		if x {
			return "True"
		}
		return "False"
	case int:
		return strconv.FormatInt(int64(x), 10)
	case int32:
		return strconv.FormatInt(int64(x), 10)
	case int64:
		return strconv.FormatInt(x, 10)
	case uint32:
		return strconv.FormatUint(uint64(x), 10)
	case uint64:
		return strconv.FormatUint(x, 10)
	case float64:
		// Integral float64 with no fractional part still renders as an
		// int-looking literal would mis-type on the SDK side, so keep a
		// trailing ".0" for true floats. jsonnum already promotes exact
		// integers to int64, so a float64 reaching here is genuinely
		// fractional or out of int64 range.
		if x == math.Trunc(x) && !math.IsInf(x, 0) {
			return strconv.FormatFloat(x, 'f', 1, 64)
		}
		return strconv.FormatFloat(x, 'g', -1, 64)
	default:
		// Lists / maps / bytes: fall back to a quoted Go rendering. These
		// are not valid composite-key field kinds (the schema rejects
		// non-scalar unique fields) so this branch is diagnostic only.
		return pyReprString(fmt.Sprintf("%v", x))
	}
}

// pyReprString renders s as a single-quoted Python string literal,
// escaping backslashes and single quotes so ast.literal_eval round-trips
// it. Matches CPython's repr() for the common (no control-char) case.
func pyReprString(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('\'')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '\\':
			b.WriteString(`\\`)
		case '\'':
			b.WriteString(`\'`)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('\'')
	return b.String()
}

// compositeIndexSuffix returns the index-name suffix
// EnsureCompositeUniqueIndex mints for a constraint, so
// parseUniqueIndexName can match a SQLite index name back to its declared
// constraint. Kept here next to the parser and exercised by indexes.go to
// guarantee the two agree.
//
// NAME-FREE per ADR-031: the suffix is derived solely from the field_id
// TUPLE (no constraint name). The form is "f<id>_<id>_..." in
// declaration order, which round-trips through parseUniqueIndexName back
// into the field_id tuple, identifying which constraint fired.
func compositeIndexSuffix(fieldIDs []uint32) string {
	parts := make([]string, 0, len(fieldIDs))
	for _, f := range fieldIDs {
		parts = append(parts, strconv.FormatUint(uint64(f), 10))
	}
	return "f" + strings.Join(parts, "_")
}
