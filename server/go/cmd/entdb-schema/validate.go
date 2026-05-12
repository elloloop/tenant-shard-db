// `validate` subcommand — internal-consistency check over a single
// snapshot. Wraps schema.ValidateAll: every edge from/to type_id and
// every FieldDef.RefTypeID must reference a registered node.
//
// Useful for users who hand-edit their `.schema-snapshot.json`.

package main

import (
	"flag"
	"fmt"
	"io"
)

func cmdValidate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		fromFile        = fs.String("from-file", "", "source: snapshot file")
		fromServer      = fs.String("from-server", "", "source: gRPC server URL")
		fromDescriptors = fs.String("from-descriptors", "", "source: protoc descriptor set (v1.1)")
	)
	fs.Usage = func() {
		fmt.Fprintln(stderr, `Usage: entdb-schema validate [flags]

Run cross-reference validation over a snapshot.

Flags:
  --from-file PATH          Source snapshot file
  --from-server URL         Source gRPC server URL
  --from-descriptors PATH   Source descriptor set (v1.1)

Reports each cross-reference error on stderr. Exit 0 if clean, 1 if
any errors, 2 on usage / I/O error.`)
	}
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	reg, _, err := loadRegistry(*fromFile, *fromServer, *fromDescriptors)
	if err != nil {
		fmt.Fprintf(stderr, "entdb-schema validate: %v\n", err)
		return 2
	}
	errs := reg.ValidateAll()
	if len(errs) == 0 {
		fmt.Fprintln(stdout, "OK: registry is internally consistent.")
		return 0
	}
	fmt.Fprintf(stdout, "INVALID: %d cross-reference error(s)\n", len(errs))
	for _, e := range errs {
		fmt.Fprintf(stdout, "  - %s\n", e)
	}
	return 1
}
