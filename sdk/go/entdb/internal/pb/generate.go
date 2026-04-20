// Package pb holds the generated Go protobuf + gRPC stubs for
// the EntDB service. It is internal to the Go SDK — user code
// interacts with the SDK via the typed surface in the parent
// package (entdb.DbClient / entdb.Scope / entdb.Plan).
//
// Regenerate with:
//
//	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
//	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
//	go generate ./...
//
// The go:generate directive below runs protoc from this directory
// against the canonical .proto in dbaas/entdb_server/api/proto so
// that the Go SDK is wire-compatible with the server and the
// Python SDK. The ``Mentdb.proto=...;pb`` flag overrides the proto's
// ``go_package`` placeholder so the stubs land in this package
// with the right import path.

//go:generate protoc -I ../../../../dbaas/entdb_server/api/proto --go_out=. --go_opt=paths=source_relative "--go_opt=Mentdb.proto=github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/pb;pb" --go-grpc_out=. --go-grpc_opt=paths=source_relative "--go-grpc_opt=Mentdb.proto=github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/pb;pb" ../../../../dbaas/entdb_server/api/proto/entdb.proto

package pb
