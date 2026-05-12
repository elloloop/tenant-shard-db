// `snapshot` subcommand — emit the current schema as a deterministic
// JSON envelope.
//
// Wire shape (v1):
//
//	{
//	  "version": 1,
//	  "fingerprint": "sha256:…",
//	  "schema": { "node_types": [...], "edge_types": [...] }
//	}
//
// Byte-for-byte deterministic: keys are sorted (via
// schema.CanonicalEncodeJSON), no whitespace beyond separators. Two
// invocations on the same registry produce identical bytes — see issue
// #488 §Determinism. The `--pretty` flag opts into encoding/json's
// MarshalIndent for human reading; CI must use the default compact
// output so byte equality is preserved.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/elloloop/tenant-shard-db/server/go/internal/schema"
)

func cmdSnapshot(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("snapshot", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		fromFile        = fs.String("from-file", "", "path to an existing snapshot file")
		fromServer      = fs.String("from-server", "", "gRPC server URL (calls GetSchema)")
		fromDescriptors = fs.String("from-descriptors", "", "path to a protoc/buf descriptor set (v1.1; not yet supported)")
		output          = fs.String("output", "", "output path (default: stdout)")
		outputShort     = fs.String("o", "", "alias for --output")
		pretty          = fs.Bool("pretty", false, "pretty-print with 2-space indent")
	)
	fs.Usage = func() {
		fmt.Fprintln(stderr, `Usage: entdb-schema snapshot [flags]

Emit the current schema as a deterministic JSON document.

Flags:
  --from-file PATH        Path to an existing snapshot file
  --from-server URL       gRPC server URL (calls GetSchema)
  --from-descriptors PATH Path to a protoc/buf descriptor set (v1.1)
  --output, -o PATH       Output path (default: stdout)
  --pretty                Pretty-print with 2-space indent

Exactly one of --from-file, --from-server, --from-descriptors is required.`)
	}
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if *outputShort != "" && *output == "" {
		*output = *outputShort
	}

	reg, _, err := loadRegistry(*fromFile, *fromServer, *fromDescriptors)
	if err != nil {
		fmt.Fprintf(stderr, "entdb-schema snapshot: %v\n", err)
		return 2
	}

	body, err := reg.CanonicalJSON()
	if err != nil {
		fmt.Fprintf(stderr, "entdb-schema snapshot: canonical encode: %v\n", err)
		return 2
	}
	// We need the envelope to be canonical too — sorted keys, no
	// whitespace — so the fingerprint round-trips. Build the envelope
	// as a map, then canonical-encode.
	var bodyAny any
	if err := json.Unmarshal(body, &bodyAny); err != nil {
		fmt.Fprintf(stderr, "entdb-schema snapshot: re-decode body: %v\n", err)
		return 2
	}
	envelope := map[string]any{
		"version":     supportedSnapshotVersion,
		"fingerprint": reg.Fingerprint(),
		"schema":      bodyAny,
	}

	var out []byte
	if *pretty {
		out, err = json.MarshalIndent(envelope, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "entdb-schema snapshot: marshal pretty: %v\n", err)
			return 2
		}
	} else {
		// Route through encoding/json once so the value graph contains
		// only the float64/string/bool/map/slice shapes the canonical
		// encoder understands (it doesn't accept raw Go ints).
		raw, err := json.Marshal(envelope)
		if err != nil {
			fmt.Fprintf(stderr, "entdb-schema snapshot: marshal envelope: %v\n", err)
			return 2
		}
		var generic any
		if err := json.Unmarshal(raw, &generic); err != nil {
			fmt.Fprintf(stderr, "entdb-schema snapshot: re-decode envelope: %v\n", err)
			return 2
		}
		out, err = schema.CanonicalEncodeJSON(generic)
		if err != nil {
			fmt.Fprintf(stderr, "entdb-schema snapshot: canonical envelope: %v\n", err)
			return 2
		}
	}

	if *output == "" {
		if _, err := stdout.Write(out); err != nil {
			fmt.Fprintf(stderr, "entdb-schema snapshot: write stdout: %v\n", err)
			return 2
		}
		// Trailing newline for friendliness; doesn't break byte
		// determinism because the canonical envelope itself doesn't
		// include one. Tests that need exact bytes call the
		// subcommand with --output PATH and read the file.
		_, _ = stdout.Write([]byte("\n"))
		return 0
	}
	if err := os.WriteFile(*output, out, 0o644); err != nil {
		fmt.Fprintf(stderr, "entdb-schema snapshot: write %q: %v\n", *output, err)
		return 2
	}
	return 0
}
