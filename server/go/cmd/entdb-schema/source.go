// Source loaders: --from-file | --from-descriptors | --from-server.
//
// --from-file is the primary supported source today. It reads a
// previously emitted snapshot file (the same shape the tool writes via
// `snapshot`) — either the bare `schema` body or the wrapped
// `{version,fingerprint,schema}` envelope.
//
// --from-server calls GetSchema on a running entdb-server over gRPC.
// Implemented in source_server.go; kept behind a build-tag-free import
// so the binary still builds without network.
//
// --from-descriptors is reserved for v1.1. It is intended to accept a
// `buf build -o fds.bin` descriptor set and reconstruct the registry
// by walking the (entdb.node)/(entdb.edge)/(entdb.field) custom proto
// options. The Go server today builds its registry from JSON only —
// the proto-options-loader is a separate piece of work (issue #488
// open question #6). For now, `--from-descriptors` returns an
// explanatory error pointing users at `--from-file` or `--from-server`.

package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/elloloop/tenant-shard-db/server/go/internal/schema"
)

// loadFromFile reads a snapshot file (envelope or bare-schema body),
// builds a registry, freezes it. Returns the loaded registry and the
// optional fingerprint from the envelope (empty when the file is a
// bare body).
func loadFromFile(path string) (*schema.Registry, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("read %q: %w", path, err)
	}
	return loadFromBytes(data, path)
}

func loadFromBytes(data []byte, label string) (*schema.Registry, string, error) {
	body, fp, err := decodeSnapshot(data)
	if err != nil {
		return nil, "", fmt.Errorf("decode %s: %w", label, err)
	}
	r, err := schema.LoadFromJSON(body)
	if err != nil {
		return nil, "", fmt.Errorf("load %s: %w", label, err)
	}
	if _, err := r.Freeze(); err != nil {
		return nil, "", fmt.Errorf("freeze %s: %w", label, err)
	}
	return r, fp, nil
}

// loadRegistry resolves one of the --from-* flags into a frozen
// registry plus an optional fingerprint (empty when the source
// doesn't carry one). Exactly one source flag must be set.
func loadRegistry(fromFile, fromServer, fromDescriptors string) (*schema.Registry, string, error) {
	n := 0
	if fromFile != "" {
		n++
	}
	if fromServer != "" {
		n++
	}
	if fromDescriptors != "" {
		n++
	}
	switch n {
	case 0:
		return nil, "", errors.New("one of --from-file, --from-server, --from-descriptors is required")
	case 1:
		// OK
	default:
		return nil, "", errors.New("at most one of --from-file, --from-server, --from-descriptors may be set")
	}
	switch {
	case fromFile != "":
		return loadFromFile(fromFile)
	case fromServer != "":
		return loadFromServer(fromServer)
	default:
		return nil, "", errFromDescriptorsUnsupported
	}
}

// errFromDescriptorsUnsupported is the documented v1 error for the
// --from-descriptors path. Plumbed through stderr by the cmd*.go
// handlers so users get a clear next-step.
var errFromDescriptorsUnsupported = errors.New(
	"--from-descriptors is not yet wired in v1 (the proto-options-based " +
		"registry loader is a separate piece of work; see issue #488 open " +
		"question #6). Use --from-file (against a snapshot JSON) or " +
		"--from-server (against a running entdb-server) instead. " +
		"Workflow: cd server/go && ./entdb-server <args>... ; then " +
		"entdb-schema snapshot --from-server localhost:50051 > /tmp/snap.json ; " +
		"entdb-schema check --baseline .schema-snapshot.json --from-file /tmp/snap.json",
)
