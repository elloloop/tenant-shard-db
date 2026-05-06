// Command entdbctl is the kubectl-style read CLI for EntDB.
//
// It is a thin wrapper around the raw gRPC client at
// internal/pb so ops/dev users can inspect a deployed
// EntDB instance from the command line — list tenants,
// browse nodes/edges, view schemas, etc. The CLI is
// intentionally read-only in v1: every subcommand maps
// to exactly one server RPC.
//
// Distribution: a static Go binary released alongside
// the existing Python and Go SDKs. Install with
//
//	go install github.com/elloloop/tenant-shard-db/sdk/go/entdb/cmd/entdbctl@latest
//
// or grab a prebuilt archive from the GitHub release page.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

// globalFlags holds the values bound to the persistent root flags.
// Subcommands read these (instead of looking them up via cobra)
// so they can be unit-tested without spinning up cobra.
type globalFlags struct {
	addr    string
	apiKey  string
	actor   string
	format  string
	timeout time.Duration
}

var flags = &globalFlags{}

// newRootCmd builds the root cobra command. It is constructed
// lazily so tests can build their own root and exercise the
// subcommands in isolation.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "entdbctl",
		Short: "Inspect a deployed EntDB instance from the command line",
		Long: `entdbctl is a read-only CLI for EntDB — list tenants, browse nodes
and edges, view schemas, and search mailboxes. Output defaults to a
human-readable table; pass --format=json to pipe into jq.

Connection settings can be supplied via flags or environment variables:
  --addr     ENTDB_ADDR     (default localhost:50051)
  --api-key  ENTDB_API_KEY
  --actor    ENTDB_ACTOR    (most read RPCs require an actor)`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().StringVar(&flags.addr, "addr", envOr("ENTDB_ADDR", "localhost:50051"), "EntDB gRPC address (host:port)")
	root.PersistentFlags().StringVar(&flags.apiKey, "api-key", os.Getenv("ENTDB_API_KEY"), "API key (Bearer token)")
	root.PersistentFlags().StringVar(&flags.actor, "actor", os.Getenv("ENTDB_ACTOR"), "Actor performing the request (e.g. user:alice)")
	root.PersistentFlags().StringVar(&flags.format, "format", "table", "Output format: table|json")
	root.PersistentFlags().DurationVar(&flags.timeout, "timeout", 30*time.Second, "Per-request timeout")

	root.AddCommand(newHealthCmd())
	root.AddCommand(newTenantsCmd())
	root.AddCommand(newSchemaCmd())
	root.AddCommand(newNodesCmd())
	root.AddCommand(newEdgesCmd())
	root.AddCommand(newGraphCmd())
	root.AddCommand(newSearchCmd())

	return root
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", friendlyError(err))
		os.Exit(1)
	}
}
