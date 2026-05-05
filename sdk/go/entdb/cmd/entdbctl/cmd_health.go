package main

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	pb "github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/pb"
)

func newHealthCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Check server health",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHealth()
		},
	}
}

func runHealth() error {
	cli, ctx, cancel, err := newClient()
	if err != nil {
		return err
	}
	defer cancel()

	resp, err := cli.Health(ctx, &pb.HealthRequest{})
	if err != nil {
		return err
	}
	if flags.format == "json" {
		return printJSON(stdout, resp)
	}
	statusStr := "DOWN"
	if resp.GetHealthy() {
		statusStr = "OK"
	}
	fmt.Fprintf(stdout, "status:  %s\n", statusStr)
	if v := resp.GetVersion(); v != "" {
		fmt.Fprintf(stdout, "version: %s\n", v)
	}
	comps := resp.GetComponents()
	if len(comps) > 0 {
		fmt.Fprintln(stdout, "components:")
		keys := make([]string, 0, len(comps))
		for k := range comps {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(stdout, "  %s: %s\n", k, comps[k])
		}
	}
	return nil
}
