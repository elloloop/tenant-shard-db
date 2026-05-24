// SPDX-License-Identifier: AGPL-3.0-only

package api

import (
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

func TestPageToken_RoundTrip(t *testing.T) {
	t.Parallel()
	fp := queryFingerprint(1, "created_at", false, nil)
	in := pageCursor{Fingerprint: fp, OrderValue: int64(1700000000123), NodeID: "n-42", Descending: false}
	tok := encodePageToken(in)
	got, err := decodePageToken(tok, fp, false)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.NodeID != in.NodeID {
		t.Errorf("NodeID = %q, want %q", got.NodeID, in.NodeID)
	}
	// JSON numbers decode as float64; the coercion helper restores int64.
	if v := cursorOrderValue("created_at", got.OrderValue); v != int64(1700000000123) {
		t.Errorf("cursorOrderValue = %v (%T), want int64(1700000000123)", v, v)
	}
}

func TestQueryFingerprint_StableAndFilterOrderInsensitive(t *testing.T) {
	t.Parallel()
	f1 := []store.QueryFilter{
		{FieldID: 2, Op: store.QueryFilterGe, Value: int64(10)},
		{FieldID: 1, Op: store.QueryFilterEq, Value: "alice"},
	}
	// Same predicates, reversed slice order.
	f2 := []store.QueryFilter{
		{FieldID: 1, Op: store.QueryFilterEq, Value: "alice"},
		{FieldID: 2, Op: store.QueryFilterGe, Value: int64(10)},
	}
	a := queryFingerprint(1, "created_at", false, f1)
	b := queryFingerprint(1, "created_at", false, f2)
	if a != b {
		t.Errorf("fingerprint is filter-order sensitive: %q != %q", a, b)
	}
	if a == queryFingerprint(1, "created_at", false, nil) {
		t.Errorf("fingerprint ignored the filters")
	}
}

func TestQueryFingerprint_DiscriminatesQueryShape(t *testing.T) {
	t.Parallel()
	base := queryFingerprint(1, "created_at", false, nil)
	cases := map[string]string{
		"type_id":    queryFingerprint(2, "created_at", false, nil),
		"order_by":   queryFingerprint(1, "node_id", false, nil),
		"descending": queryFingerprint(1, "created_at", true, nil),
		"filter":     queryFingerprint(1, "created_at", false, []store.QueryFilter{{FieldID: 1, Op: store.QueryFilterEq, Value: "x"}}),
	}
	for name, fp := range cases {
		if fp == base {
			t.Errorf("fingerprint did not change when %s changed", name)
		}
	}
}

func TestDecodePageToken_Rejections(t *testing.T) {
	t.Parallel()
	fp := queryFingerprint(1, "created_at", false, nil)
	good := encodePageToken(pageCursor{Fingerprint: fp, OrderValue: int64(1), NodeID: "n1", Descending: false})

	t.Run("malformed base64", func(t *testing.T) {
		_, err := decodePageToken("!!!not base64!!!", fp, false)
		assertInvalidArg(t, err)
	})
	t.Run("fingerprint mismatch", func(t *testing.T) {
		_, err := decodePageToken(good, queryFingerprint(2, "created_at", false, nil), false)
		assertInvalidArg(t, err)
	})
	t.Run("direction mismatch", func(t *testing.T) {
		_, err := decodePageToken(good, fp, true)
		assertInvalidArg(t, err)
	})
	t.Run("valid token passes", func(t *testing.T) {
		if _, err := decodePageToken(good, fp, false); err != nil {
			t.Fatalf("valid token rejected: %v", err)
		}
	})
}

func assertInvalidArg(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("code = %s, want InvalidArgument (%v)", status.Code(err), err)
	}
}
