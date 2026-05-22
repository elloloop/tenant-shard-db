// `check` subcommand — verify the current schema is compatible with a
// baseline snapshot. Exits 0 on compatible, 1 on breaking, 2 on error.
//
// The classification rules live in schema.Check; this subcommand is
// purely flag parsing + formatting.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/elloloop/tenant-shard-db/server/go/internal/schema"
)

func cmdCheck(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		baseline        = fs.String("baseline", "", "path to baseline snapshot (required)")
		baselineShort   = fs.String("b", "", "alias for --baseline")
		fromFile        = fs.String("from-file", "", "source: snapshot file")
		fromServer      = fs.String("from-server", "", "source: gRPC server URL")
		fromDescriptors = fs.String("from-descriptors", "", "source: protoc descriptor set (v1.1)")
		format          = fs.String("format", "text", "output format: text|json")
		allowBreaking   = fs.Bool("allow-breaking", false, "exit 0 even if breaking changes found")
	)
	fs.Usage = func() {
		fmt.Fprintln(stderr, `Usage: entdb-schema check [flags]

Verify the current schema is compatible with a baseline.

Flags:
  --baseline, -b PATH       Path to baseline snapshot (required)
  --from-file PATH          Source of current schema
  --from-server URL         Source of current schema
  --from-descriptors PATH   Source of current schema (v1.1)
  --format text|json        Output format (default: text)
  --allow-breaking          Exit 0 even if breaking changes found (still prints)

Exit codes:
  0  compatible
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
		fmt.Fprintln(stderr, "entdb-schema check: --baseline is required")
		return 2
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "entdb-schema check: --format must be text or json, got %q\n", *format)
		return 2
	}

	oldReg, _, err := loadFromFile(*baseline)
	if err != nil {
		fmt.Fprintf(stderr, "entdb-schema check: baseline: %v\n", err)
		return 2
	}
	newReg, _, err := loadRegistry(*fromFile, *fromServer, *fromDescriptors)
	if err != nil {
		fmt.Fprintf(stderr, "entdb-schema check: %v\n", err)
		return 2
	}

	changes := schema.Check(oldReg, newReg)
	if err := writeChanges(stdout, changes, *baseline, *format); err != nil {
		fmt.Fprintf(stderr, "entdb-schema check: write output: %v\n", err)
		return 2
	}
	if schema.HasBreaking(changes) && !*allowBreaking {
		return 1
	}
	return 0
}

// writeChanges renders the change list in the requested format.
func writeChanges(w io.Writer, changes []schema.Change, baselineLabel, format string) error {
	switch format {
	case "json":
		out := map[string]any{
			"compatible":         !schema.HasBreaking(changes),
			"breaking_count":     schema.CountBreaking(changes),
			"non_breaking_count": len(changes) - schema.CountBreaking(changes),
			"changes":            changes,
		}
		buf, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return err
		}
		if _, err := w.Write(buf); err != nil {
			return err
		}
		_, err = w.Write([]byte("\n"))
		return err
	default:
		_, err := w.Write([]byte(schema.RenderText(changes, baselineLabel)))
		return err
	}
}
