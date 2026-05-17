// Package pb holds the generated Go protobuf + gRPC stubs for
// the EntDB service. It is internal to the Go SDK — user code
// interacts with the SDK via the typed surface in the parent
// package (entdb.DbClient / entdb.Scope / entdb.Plan).
//
// Regenerate from the repository root with:
//
//	buf generate --template sdk/go/entdb/buf.gen.yaml
//
// The buf template uses managed mode to override the proto's
// ``go_package`` placeholder so the stubs land in this package with
// the SDK module's import path. The Go SDK keeps its OWN copy of these
// stubs (distinct from server/go/internal/pb/) so SDK consumers never
// transitively pull server-only dependencies; the wire format is the
// contract and both copies come from the same proto/entdb/v1/entdb.proto.

//go:generate sh -c "cd ../../../../ && buf generate --template sdk/go/entdb/buf.gen.yaml"

package pb
