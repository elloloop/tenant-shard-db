// SPDX-License-Identifier: AGPL-3.0-only

// Keyset cursor page tokens (ADR-029, AIP-158).
//
// A page token is an opaque, base64url-encoded JSON blob carrying the
// keyset anchor of the last row returned — the effective order_by value
// plus node_id — together with the sort direction and a fingerprint of
// the query (type_id + order_by + descending + filters). The fingerprint
// is integrity, not security: it stops a token minted for one query from
// being replayed against a different one and silently returning wrong
// rows (ADR-029 invariant 2). It is a plain hash, not a signature —
// tokens are not a trust boundary, the tenant/actor gate is.
//
// Continuation is a seek, not a skip: the store turns the anchor into
// ``WHERE (order_by, node_id) > (anchor)`` (or ``<`` descending), so the
// next page resumes exactly after the last row without skipping or
// duplicating rows under concurrent writes.

package api

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sort"

	"google.golang.org/grpc/codes"

	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

// pageCursor is the decoded payload of a page token.
type pageCursor struct {
	// Fingerprint binds the token to the exact query it was minted for.
	Fingerprint string `json:"f"`
	// OrderValue is the effective order_by value of the last row. JSON
	// round-trips numbers as float64; the handler coerces back to the
	// column's Go type via cursorOrderValue before handing it to the store.
	OrderValue any `json:"v"`
	// NodeID is the unique tiebreaker — the final sort key.
	NodeID string `json:"n"`
	// Descending records the sort direction the token was minted under, so
	// a token cannot be replayed with the direction flipped.
	Descending bool `json:"d"`
}

// queryFingerprint is a stable hash of the query shape. Every page of the
// same query produces the same fingerprint (the client resends the same
// type_id / order_by / descending / filters), so a continuation token
// validates; any change invalidates it.
func queryFingerprint(typeID int32, orderBy string, descending bool, filters []store.QueryFilter) string {
	h := sha256.New()
	var scratch [8]byte
	binary.BigEndian.PutUint32(scratch[:4], uint32(typeID))
	h.Write(scratch[:4])
	h.Write([]byte(store.EffectiveOrderBy(orderBy)))
	if descending {
		h.Write([]byte{1})
	} else {
		h.Write([]byte{0})
	}
	// Filters are AND-ed and order-insensitive to the result set, so sort
	// a canonical rendering before hashing — two requests with the same
	// predicates in a different slice order share a fingerprint.
	rendered := make([]string, 0, len(filters))
	for _, f := range filters {
		vb, _ := json.Marshal(f.Value)
		rendered = append(rendered, fmt.Sprintf("%d:%d:%s", f.FieldID, f.Op, vb))
	}
	sort.Strings(rendered)
	for _, r := range rendered {
		h.Write([]byte{0x1f}) // unit separator so concatenation is unambiguous
		h.Write([]byte(r))
	}
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

// encodePageToken serializes a cursor to an opaque token.
func encodePageToken(c pageCursor) string {
	b, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(b)
}

// decodePageToken parses a token and verifies it was minted for the given
// query. A malformed token or a fingerprint/direction mismatch is an
// INVALID_ARGUMENT — never a silent wrong-rows result.
func decodePageToken(token, fingerprint string, descending bool) (pageCursor, error) {
	var c pageCursor
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return c, errs.Errorf(codes.InvalidArgument, "page_token: malformed: %v", err)
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return c, errs.Errorf(codes.InvalidArgument, "page_token: malformed: %v", err)
	}
	if c.Fingerprint != fingerprint {
		return c, errs.Errorf(codes.InvalidArgument,
			"page_token: does not match this query (type_id / filters / order_by changed)")
	}
	if c.Descending != descending {
		return c, errs.Errorf(codes.InvalidArgument,
			"page_token: sort direction changed since the token was issued")
	}
	return c, nil
}

// cursorOrderValue coerces a JSON-decoded cursor order value back to the
// Go type the store expects for the effective order column. created_at /
// updated_at / type_id are integer columns (JSON decodes them as
// float64); node_id is a string. created_at/updated_at are unix-ms and
// stay well under 2^53, so the float64 round-trip is lossless.
func cursorOrderValue(orderBy string, v any) any {
	switch store.EffectiveOrderBy(orderBy) {
	case "created_at", "updated_at", "type_id":
		if f, ok := v.(float64); ok {
			return int64(f)
		}
	}
	return v
}
