package schema

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// computeFingerprint produces the canonical schema fingerprint:
//
//	canonical = json.dumps(self.to_dict(), sort_keys=True, separators=(",", ":"))
//	hash = sha256(canonical).hexdigest()
//	return f"sha256:{hash}"
//
// We can't rely on encoding/json because Go preserves struct-field
// declaration order rather than sorting keys, and Python's
// separators=(",",":") drops the default ", "/": " spaces. We marshal
// the registry into the same shape as Python's to_dict(), then
// re-emit it with a custom canonical encoder.
//
// Floats: registry data does not include floats today, but the encoder
// supports them for forward compatibility (using strconv with
// 'g'/-1 precision, which matches Python's repr-derived output for
// integral floats — round-trip safety is the contract, not byte
// equality with Python for arbitrary floats).
func (r *Registry) computeFingerprint() (string, error) {
	doc := r.toFile()
	// Round-trip through encoding/json into a generic value so we can
	// re-emit with sorted keys exactly as Python does.
	raw, err := json.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("schema: marshal for fingerprint: %w", err)
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return "", fmt.Errorf("schema: unmarshal for fingerprint: %w", err)
	}
	var sb strings.Builder
	if err := canonicalEncode(&sb, v); err != nil {
		return "", fmt.Errorf("schema: canonical encode: %w", err)
	}
	sum := sha256.Sum256([]byte(sb.String()))
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// CanonicalJSON marshals the registry into the byte-for-byte
// deterministic shape used by the fingerprint (Python's
// json.dumps(sort_keys=True, separators=(",", ":"))). entdb-schema
// uses this for snapshot output so two invocations on the same
// registry produce identical bytes — see issue #488 determinism
// requirements (§Determinism, points 7-8).
func (r *Registry) CanonicalJSON() ([]byte, error) {
	doc := r.toFile()
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("schema: marshal for canonical JSON: %w", err)
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, fmt.Errorf("schema: unmarshal for canonical JSON: %w", err)
	}
	var sb strings.Builder
	if err := canonicalEncode(&sb, v); err != nil {
		return nil, fmt.Errorf("schema: canonical encode: %w", err)
	}
	return []byte(sb.String()), nil
}

// CanonicalEncodeJSON is the exported escape hatch used by
// entdb-schema to canonicalise a generic JSON value (e.g. the
// snapshot envelope) using the same sort-keys/no-whitespace rules as
// the fingerprint. Accepts the standard encoding/json value shapes
// (map[string]any, []any, string, bool, float64, json.Number, nil).
func CanonicalEncodeJSON(v any) ([]byte, error) {
	var sb strings.Builder
	if err := canonicalEncode(&sb, v); err != nil {
		return nil, fmt.Errorf("schema: canonical encode: %w", err)
	}
	return []byte(sb.String()), nil
}

// canonicalEncode writes v to sb using Python's
// `json.dumps(sort_keys=True, separators=(",", ":"))` byte format:
//   - Object keys sorted by Unicode code point (Python sorted() default).
//   - No whitespace between separators.
//   - Strings escaped using the standard ensure_ascii=True semantics
//     used by Python by default, which matches Go's encoding/json
//     escaping for ASCII payloads. Schema metadata is ASCII today
//     (names, descriptions in English), so we use json.Marshal of the
//     leaf string which is a strict superset (Go always escapes the
//     same control chars and uses the same backslash sequences).
//
// If the schema gains non-ASCII content we will need a custom string
// encoder that matches Python's `\uXXXX` escapes — flagged in the
// fingerprint parity test.
func canonicalEncode(sb *strings.Builder, v any) error {
	switch x := v.(type) {
	case nil:
		sb.WriteString("null")
	case bool:
		if x {
			sb.WriteString("true")
		} else {
			sb.WriteString("false")
		}
	case string:
		b, err := json.Marshal(x)
		if err != nil {
			return err
		}
		sb.Write(b)
	case float64:
		// json.Unmarshal lands all numbers in float64. Emit integers
		// without trailing ".0" to match Python's JSON output.
		if x == float64(int64(x)) {
			sb.WriteString(strconv.FormatInt(int64(x), 10))
		} else {
			sb.WriteString(strconv.FormatFloat(x, 'g', -1, 64))
		}
	case json.Number:
		sb.WriteString(string(x))
	case []any:
		sb.WriteByte('[')
		for i, e := range x {
			if i > 0 {
				sb.WriteByte(',')
			}
			if err := canonicalEncode(sb, e); err != nil {
				return err
			}
		}
		sb.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		sb.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				sb.WriteByte(',')
			}
			kb, err := json.Marshal(k)
			if err != nil {
				return err
			}
			sb.Write(kb)
			sb.WriteByte(':')
			if err := canonicalEncode(sb, x[k]); err != nil {
				return err
			}
		}
		sb.WriteByte('}')
	default:
		return fmt.Errorf("canonicalEncode: unsupported type %T", v)
	}
	return nil
}
