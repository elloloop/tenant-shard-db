// SPDX-License-Identifier: AGPL-3.0-only

package store

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

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
// (multi-field) constraint; for single-field violations ConstraintName
// is empty and FieldIDs holds exactly one id.
type uniqueViolationParts struct {
	Composite      bool
	TypeID         int32
	ConstraintName string
	FieldIDs       []uint32
}

// parseUniqueIndexName decodes an index name minted by indexes.go back
// into its constraint coordinates. Returns ok=false when the name is
// not one of our unique-index names (e.g. a UNIQUE failure on a
// built-in table constraint such as applied_events). The registry is
// consulted to map a single-field index back to its field_id and a
// composite index back to its declared field_id tuple — the index name
// only carries the type_id and (for composites) the constraint name.
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
	// Composite: idx_unique_t<T>_c<safeName>
	if m := compositeIndexRE.FindStringSubmatch(name); m != nil {
		t, err := strconv.Atoi(m[1])
		if err != nil {
			return uniqueViolationParts{}, false
		}
		typeID := int32(t)
		safe := m[2]
		// Resolve the safe-name suffix back to the declared constraint
		// via the registry so we recover the real (un-sanitized) name
		// and the field_id tuple in declaration order.
		if s.registry != nil {
			for _, c := range s.registry.CompositeUnique(typeID) {
				if compositeIndexSuffix(c.Name, c.FieldIDs) == safe {
					return uniqueViolationParts{
						Composite:      true,
						TypeID:         typeID,
						ConstraintName: c.Name,
						FieldIDs:       append([]uint32(nil), c.FieldIDs...),
					}, true
				}
			}
		}
		// No registry match — surface what we can (the safe suffix as a
		// best-effort name). Still flagged composite so the SDK routes
		// to the composite branch.
		return uniqueViolationParts{
			Composite:      true,
			TypeID:         typeID,
			ConstraintName: safe,
		}, true
	}
	return uniqueViolationParts{}, false
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
//	composite:
//	  Composite unique constraint violation: tenant=<t> type_id=<T> constraint='<name>' fields=[<F>, ...] values=[<repr>, ...] already exists
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
		tenantID, parts.TypeID, pyReprString(parts.ConstraintName),
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

// compositeIndexSuffix returns the index-name suffix EnsureCompositeUniqueIndex
// mints for a constraint, so parseUniqueIndexName can match a SQLite
// index name back to its declared constraint. Kept here next to the
// parser and exercised by indexes.go to guarantee the two agree.
func compositeIndexSuffix(name string, fieldIDs []uint32) string {
	safe := safeIdent(name)
	if safe != "" {
		return safe
	}
	parts := make([]string, 0, len(fieldIDs))
	for _, f := range fieldIDs {
		parts = append(parts, strconv.FormatUint(uint64(f), 10))
	}
	return "f" + strings.Join(parts, "_")
}
