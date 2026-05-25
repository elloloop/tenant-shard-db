// `breaking` subcommand — the buf-breaking-style gate (ADR-032).
//
// This is the single command a downstream app team runs IDENTICALLY in
// local dev and in CI to catch dangerous schema changes before deploy,
// the runtime-schema analogue of `buf breaking`:
//
//	entdb-schema breaking --baseline schema.lock.json --from-file new.json
//
// It is intentionally the same engine as `check` (schema.Check), with the
// verb chosen to mirror buf so the muscle memory and CI step look the
// same. Like `check` it exits NON-ZERO on any breaking change and prints
// a human-readable report of each offending change.
//
// Exit codes (stable CI contract):
//
//	0  compatible — no breaking changes (non-breaking changes still printed)
//	1  one or more BREAKING changes detected
//	2  usage / I/O error

package main

import (
	"flag"
	"fmt"
	"io"

	"github.com/elloloop/tenant-shard-db/server/go/internal/schema"
)

func cmdBreaking(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("breaking", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		baseline        = fs.String("baseline", "", "path to the committed baseline snapshot (lock file) — required")
		baselineShort   = fs.String("b", "", "alias for --baseline")
		fromFile        = fs.String("from-file", "", "current schema: snapshot file generated from your proto")
		fromServer      = fs.String("from-server", "", "current schema: gRPC server URL (calls GetSchema)")
		fromDescriptors = fs.String("from-descriptors", "", "current schema: protoc/buf descriptor set (v1.1)")
		format          = fs.String("format", "text", "output format: text|json")
		allowBreaking   = fs.Bool("allow-breaking", false, "exit 0 even if breaking changes found (still prints)")
	)
	fs.Usage = func() {
		fmt.Fprintln(stderr, `Usage: entdb-schema breaking [flags]

Gate a schema change against a committed baseline — the runtime-schema
analogue of `+"`buf breaking`"+`. Run this IDENTICALLY in local dev and CI.

  entdb-schema breaking --baseline schema.lock.json --from-file new.json

Flags:
  --baseline, -b PATH       Committed baseline (lock file) — required
  --from-file PATH          Current schema (snapshot generated from your proto)
  --from-server URL         Current schema (running entdb-server)
  --from-descriptors PATH   Current schema (descriptor set; v1.1)
  --format text|json        Output format (default: text)
  --allow-breaking          Exit 0 even if breaking changes found (still prints)

Exit codes:
  0  compatible (no breaking changes)
  1  breaking change(s) detected
  2  usage / I/O error`)
	}
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if *baselineShort != "" && *baseline == "" {
		*baseline = *baselineShort
	}
	if *baseline == "" {
		fmt.Fprintln(stderr, "entdb-schema breaking: --baseline is required")
		return 2
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "entdb-schema breaking: --format must be text or json, got %q\n", *format)
		return 2
	}

	oldReg, _, err := loadFromFile(*baseline)
	if err != nil {
		fmt.Fprintf(stderr, "entdb-schema breaking: baseline: %v\n", err)
		return 2
	}
	newReg, _, err := loadRegistry(*fromFile, *fromServer, *fromDescriptors)
	if err != nil {
		fmt.Fprintf(stderr, "entdb-schema breaking: %v\n", err)
		return 2
	}

	changes := schema.Check(oldReg, newReg)
	if err := writeChanges(stdout, changes, *baseline, *format); err != nil {
		fmt.Fprintf(stderr, "entdb-schema breaking: write output: %v\n", err)
		return 2
	}
	if schema.HasBreaking(changes) && !*allowBreaking {
		return 1
	}
	return 0
}
