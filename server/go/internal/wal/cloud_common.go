// SPDX-License-Identifier: AGPL-3.0-only

package wal

// Helpers shared by the cloud-native WAL backends (Kinesis, Pub/Sub,
// SQS, Service Bus, Event Hubs). These backends differ in their
// transport but share two cross-cutting concerns:
//
//   - Header transport: Kinesis records and Pub/Sub messages have no
//     first-class binary header map the way Kafka does. To keep the
//     wal.Record.Headers contract (and the HeaderIdempotencyKey
//     round-trip) intact we embed headers in the payload envelope when
//     the backend can't carry them natively. This mirrors the retired
//     Python source (kinesis.py / pubsub.py _headers/_data wrapping).
//   - At-least-once redelivery: backends that ack-on-commit (SQS,
//     Service Bus) keep an in-memory map from StreamPos.Offset to the
//     backend-native ack handle so Commit can resolve it.
//
// The envelope is intentionally the same JSON shape the Python source
// used so a stream written by the (now retired) Python server and read
// by the Go server — or vice versa during a migration — round-trips.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/aws/smithy-go"
)

// headerEnvelope is the JSON wrapper used to smuggle headers through a
// transport with no native header map. Byte-for-byte the shape the
// Python kinesis.py / pubsub.py used: {"_headers": {...}, "_data": ...}.
type headerEnvelope struct {
	Headers map[string]string `json:"_headers"`
	Data    string            `json:"_data"`
}

// wrapHeaders returns value unchanged when there are no headers (so a
// plain payload stays a plain payload — only records that actually
// carry headers pay the envelope cost, exactly like the Python source).
// When headers are present it wraps them into the JSON envelope.
//
// Header values are decoded as UTF-8 (errors replaced) to match the
// Python `.decode("utf-8", errors="replace")` behaviour; the WAL
// idempotency key and applier-set headers are always UTF-8 text.
func wrapHeaders(value []byte, headers map[string][]byte) []byte {
	if len(headers) == 0 {
		return value
	}
	env := headerEnvelope{
		Headers: make(map[string]string, len(headers)),
		Data:    decodeUTF8(value),
	}
	for k, v := range headers {
		env.Headers[k] = decodeUTF8(v)
	}
	out, err := json.Marshal(env)
	if err != nil {
		// Marshal of a string map can't realistically fail; fall back
		// to the raw value rather than dropping the record.
		return value
	}
	return out
}

// unwrapHeaders is the inverse of wrapHeaders. If data is the JSON
// envelope it returns the inner payload + headers; otherwise it returns
// data unchanged with no headers (a record that was written without
// headers).
func unwrapHeaders(data []byte) ([]byte, map[string][]byte) {
	if len(data) == 0 || data[0] != '{' {
		return data, nil
	}
	var env headerEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return data, nil
	}
	if env.Headers == nil && env.Data == "" {
		// Not our envelope (some other JSON object); pass through.
		return data, nil
	}
	hdrs := make(map[string][]byte, len(env.Headers))
	for k, v := range env.Headers {
		hdrs[k] = []byte(v)
	}
	return []byte(env.Data), hdrs
}

// decodeUTF8 returns b as a string, replacing invalid UTF-8 with the
// Unicode replacement char (matching the Python source's
// .decode("utf-8", errors="replace")). Go's []byte->string conversion
// already substitutes U+FFFD for invalid sequences when the bytes are
// later range-iterated/re-encoded; round-tripping through []rune makes
// the substitution eager and explicit so the stored/transmitted form
// is deterministic.
func decodeUTF8(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	return string([]rune(string(b)))
}

// stringHeaders converts a binary header map into a string map for
// backends whose native attribute type is string->string (SQS message
// attributes, Pub/Sub attributes, Service Bus application properties,
// Event Hubs properties).
func stringHeaders(headers map[string][]byte) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	out := make(map[string]string, len(headers))
	for k, v := range headers {
		out[k] = decodeUTF8(v)
	}
	return out
}

// bytesHeaders is the inverse of stringHeaders.
func bytesHeaders(headers map[string]string) map[string][]byte {
	if len(headers) == 0 {
		return map[string][]byte{}
	}
	out := make(map[string][]byte, len(headers))
	for k, v := range headers {
		out[k] = []byte(v)
	}
	return out
}

// classifyAWSErr maps an AWS SDK error to a WAL sentinel so callers can
// errors.Is() it. Context cancellation passes through untouched;
// throughput/throttling maps to ErrTimeout; missing-resource maps to
// ErrConnection; everything else rolls up under ErrWal. Shared by the
// AWS-backed backends (SQS today; Kinesis keeps its own classifier
// because it adds Kinesis-specific codes).
func classifyAWSErr(op string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return err
	}
	var ae smithy.APIError
	if errors.As(err, &ae) {
		code := ae.ErrorCode()
		switch {
		case strings.Contains(code, "Throttl"),
			strings.Contains(code, "ProvisionedThroughputExceeded"),
			strings.Contains(code, "RequestLimitExceeded"):
			return fmt.Errorf("%w: %s: %v", ErrTimeout, op, err)
		case strings.Contains(code, "NonExistentQueue"),
			strings.Contains(code, "QueueDoesNotExist"),
			strings.Contains(code, "ResourceNotFound"):
			return fmt.Errorf("%w: %s: %v", ErrConnection, op, err)
		}
	}
	if isTimeout(err) {
		return fmt.Errorf("%w: %s: %v", ErrTimeout, op, err)
	}
	return fmt.Errorf("%w: %s: %v", ErrWal, op, err)
}
