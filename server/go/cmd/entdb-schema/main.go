// entdb-schema — CLI for snapshotting and checking EntDB schema
// registries. See `entdb-schema --help` for usage and
// docs/guides/schema-lockdown.md for the design.
//
// Exit codes (stable contract for CI):
//   - 0: success (compatible / no breaking changes)
//   - 1: breaking change detected
//   - 2: usage / I/O error
//
// stdout carries machine output (snapshot JSON, check/diff results in
// --format json). stderr carries progress and human-readable errors.
// CI scripts can rely on this separation.
package main

import (
	"fmt"
	"os"
)

// version is the tool version. Overridden at link time via
// -ldflags="-X main.version=...". Default reflects the v1 contract.
var version = "v1.0.0"

const usage = `entdb-schema — manage EntDB schema snapshots and check compatibility.

Usage:
  entdb-schema <command> [flags]

Commands:
  snapshot   Emit the current schema as a deterministic JSON document.
  breaking   Gate a schema change against a baseline (buf-breaking style).
  check      Alias of breaking — verify compatibility with a baseline.
  diff       Show differences between two snapshot files.
  validate   Run cross-reference validation over a snapshot.
  version    Print version and exit.
  help       Print this help message.

Run 'entdb-schema <command> --help' for command-specific flags.

Exit codes:
  0  success / compatible
  1  breaking change detected
  2  usage or I/O error
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	cmd := os.Args[1]
	args := os.Args[2:]
	switch cmd {
	case "snapshot":
		os.Exit(cmdSnapshot(args, os.Stdout, os.Stderr))
	case "breaking":
		os.Exit(cmdBreaking(args, os.Stdout, os.Stderr))
	case "check":
		os.Exit(cmdCheck(args, os.Stdout, os.Stderr))
	case "diff":
		os.Exit(cmdDiff(args, os.Stdout, os.Stderr))
	case "validate":
		os.Exit(cmdValidate(args, os.Stdout, os.Stderr))
	case "version", "--version", "-v":
		fmt.Fprintln(os.Stdout, "entdb-schema "+version)
		os.Exit(0)
	case "help", "--help", "-h":
		fmt.Fprint(os.Stdout, usage)
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "entdb-schema: unknown command %q\n\n%s", cmd, usage)
		os.Exit(2)
	}
}
