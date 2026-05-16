// Package pb holds the generated Go protobuf + gRPC stubs for the
// EntDB gRPC service (entdb.v1). It is internal to the server module —
// handlers in server/go/internal/api consume these types directly.
//
// The console-service stubs (entdb.console.v1) live in the sibling
// sub-package server/go/internal/pb/consolev1; they re-use the message
// types defined here.
//
// Regenerate both surfaces with:
//
//	cd server/go && go generate ./...
//
// which runs `buf generate` against both templates from the repository
// root. The generated files are checked in; CI fails if regeneration
// produces a diff (drift guard in .github/workflows/ci.yml).
//
// Each `go:generate` line runs through `sh -c` so we can change to
// the repository root before invoking buf — both buf templates use
// repo-root-relative paths in their `inputs:` and plugin `out:` fields,
// matching the convention documented in server/go/README.md.

//go:generate sh -c "cd ../../../ ...&& buf generate --template server/go/buf.gen.yaml"
//go:generate sh -c "cd ../../../ ...&& buf generate --template server/go/buf.gen.console.yaml"

package pb
