// `diff` subcommand — file-vs-file schema diff. Same output shape as
// `check`, but never returns non-zero for breaking changes unless the
// user opts in with --fail-on-breaking. Designed for informational use
// (e.g. local exploration); CI should use `check`.

package main

import (
	"flag"
	"fmt"
	"io"

	"github.com/elloloop/tenant-shard-db/server/go/internal/schema"
)

func cmdDiff(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		oldPath        = fs.String("old", "", "path to the older snapshot (required)")
		newPath        = fs.String("new", "", "path to the newer snapshot (required)")
		format         = fs.String("format", "text", "output format: text|json")
		failOnBreaking = fs.Bool("fail-on-breaking", false, "exit 1 if breaking changes are present")
	)
	fs.Usage = func() {
		fmt.Fprintln(stderr, `Usage: entdb-schema diff [flags]

Show differences between two snapshot files.

Flags:
  --old PATH                Older snapshot (required)
  --new PATH                Newer snapshot (required)
  --format text|json        Output format (default: text)
  --fail-on-breaking        Exit 1 if any breaking change is present`)
	}
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if *oldPath == "" || *newPath == "" {
		fmt.Fprintln(stderr, "entdb-schema diff: --old and --new are required")
		return 2
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "entdb-schema diff: --format must be text or json, got %q\n", *format)
		return 2
	}

	oldReg, _, err := loadFromFile(*oldPath)
	if err != nil {
		fmt.Fprintf(stderr, "entdb-schema diff: --old: %v\n", err)
		return 2
	}
	newReg, _, err := loadFromFile(*newPath)
	if err != nil {
		fmt.Fprintf(stderr, "entdb-schema diff: --new: %v\n", err)
		return 2
	}

	changes := schema.Check(oldReg, newReg)
	if err := writeChanges(stdout, changes, *oldPath, *format); err != nil {
		fmt.Fprintf(stderr, "entdb-schema diff: write output: %v\n", err)
		return 2
	}
	if *failOnBreaking && schema.HasBreaking(changes) {
		return 1
	}
	return 0
}
