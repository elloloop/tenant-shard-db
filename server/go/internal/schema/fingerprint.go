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
//	canonical = sort-keys compact JSON of the registry's to_dict form
//	hash      = sha256(canonical).hexdigest()
//	return "sha256:" + hash
//
// We can't rely on encoding/json because Go preserves struct-field
// declaration order rather than sorting keys, and the canonical format
// omits whitespace between separators. We marshal the registry into
// the to_dict() shape, then re-emit it with a custom canonical encoder.
//
// Floats: registry data does not include floats today, but the encoder
// supports them for forward compatibility (using strconv with
// 'g'/-1 precision for integral floats — round-trip safety is the
// contract).
func (r *Registry) computeFingerprint() (string, error) {
	doc := r.toFile()
	// Round-trip through encoding/json into a generic value so we can
	// re-emit with sorted keys in canonical order.
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
// deterministic shape used by the fingerprint (sort-keys, no-whitespace
// compact JSON). entdb-schema
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

// canonicalEncode writes v to sb in sort-keys, no-whitespace compact
// JSON format:
//   - Object keys sorted by Unicode code point.
//   - No whitespace between separators.
//   - Strings escaped via json.Marshal (ASCII payloads; schema metadata
//     is ASCII today — names and descriptions in English).
//
// If the schema gains non-ASCII content a custom string encoder will
// be needed to emit `\uXXXX` escapes — flagged in the fingerprint
// parity test.
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
		// without trailing ".0" for canonical output.
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
