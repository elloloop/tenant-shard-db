package schema

import "encoding/json"

// nodeDefEqual reports whether two node type definitions are identical
// for the purpose of the establish-or-reject policy
// (RegisterOrVerifyNode). "Identical" means the canonical JSON form of
// both definitions is byte-for-byte equal — the SAME notion of identity
// the schema fingerprint uses, so two clients that share a fingerprint
// always agree on what each type means.
//
// Comparing canonical JSON (rather than reflect.DeepEqual) keeps the
// comparison stable across the `Default any` field's dynamic type and
// matches how the cross-language contract defines type identity.
func nodeDefEqual(a, b *NodeTypeDef) bool {
	if a == nil || b == nil {
		return a == b
	}
	return canonicalEqual(a, b)
}

// edgeDefEqual is the edge counterpart of nodeDefEqual.
func edgeDefEqual(a, b *EdgeTypeDef) bool {
	if a == nil || b == nil {
		return a == b
	}
	return canonicalEqual(a, b)
}

// canonicalEqual marshals a and b to canonical (sort-keys, no-whitespace)
// JSON and compares the bytes. Returns false if either fails to marshal
// (treating an un-canonicalisable value as non-equal is the safe,
// conflict-rejecting default).
func canonicalEqual(a, b any) bool {
	ca, err := canonicalBytes(a)
	if err != nil {
		return false
	}
	cb, err := canonicalBytes(b)
	if err != nil {
		return false
	}
	return string(ca) == string(cb)
}

// canonicalBytes renders v as canonical JSON using the same sort-keys /
// no-whitespace rules as the fingerprint canonicaliser, so equality here
// is consistent with fingerprint equality.
func canonicalBytes(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil, err
	}
	return CanonicalEncodeJSON(generic)
}
