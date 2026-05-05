package main

import (
	"fmt"

	"github.com/spf13/cobra"

	pb "github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/pb"
)

func newTenantsCmd() *cobra.Command {
	tenants := &cobra.Command{
		Use:   "tenants",
		Short: "List and inspect tenants",
	}
	tenants.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all tenants visible to the caller",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTenantsList()
		},
	})
	tenants.AddCommand(&cobra.Command{
		Use:   "get <tenant-id>",
		Short: "Show a single tenant's details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTenantsGet(args[0])
		},
	})
	return tenants
}

func runTenantsList() error {
	cli, ctx, cancel, err := newClient()
	if err != nil {
		return err
	}
	defer cancel()

	resp, err := cli.ListTenants(ctx, &pb.ListTenantsRequest{})
	if err != nil {
		return err
	}
	if flags.format == "json" {
		return printJSON(stdout, resp)
	}
	t := newTable(stdout, "TENANT_ID")
	for _, ti := range resp.GetTenants() {
		t.Row(ti.GetTenantId())
	}
	return t.Flush()
}

func runTenantsGet(tenantID string) error {
	cli, ctx, cancel, err := newClient()
	if err != nil {
		return err
	}
	defer cancel()

	resp, err := cli.GetTenant(ctx, &pb.GetTenantRequest{
		Actor:    flags.actor,
		TenantId: tenantID,
	})
	if err != nil {
		return err
	}
	if flags.format == "json" {
		return printJSON(stdout, resp)
	}
	if !resp.GetFound() {
		return fmt.Errorf("tenant %q not found", tenantID)
	}
	td := resp.GetTenant()
	fmt.Fprintf(stdout, "tenant_id:  %s\n", td.GetTenantId())
	fmt.Fprintf(stdout, "name:       %s\n", td.GetName())
	fmt.Fprintf(stdout, "status:     %s\n", td.GetStatus())
	fmt.Fprintf(stdout, "region:     %s\n", td.GetRegion())
	fmt.Fprintf(stdout, "created_at: %s\n", formatTime(td.GetCreatedAt()))
	return nil
}
