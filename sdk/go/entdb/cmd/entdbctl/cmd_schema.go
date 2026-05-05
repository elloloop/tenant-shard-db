package main

import (
	"fmt"

	"github.com/spf13/cobra"

	pb "github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/pb"
)

func newSchemaCmd() *cobra.Command {
	schema := &cobra.Command{
		Use:   "schema",
		Short: "Inspect tenant schemas",
	}
	schema.AddCommand(&cobra.Command{
		Use:   "show <tenant-id>",
		Short: "Show node and edge types registered for a tenant",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSchemaShow(args[0])
		},
	})
	return schema
}

func runSchemaShow(tenantID string) error {
	cli, ctx, cancel, err := newClient()
	if err != nil {
		return err
	}
	defer cancel()

	resp, err := cli.GetSchema(ctx, &pb.GetSchemaRequest{
		TenantId: tenantID,
	})
	if err != nil {
		return err
	}
	if flags.format == "json" {
		return printJSON(stdout, resp)
	}

	fp := resp.GetFingerprint()
	if fp != "" {
		fmt.Fprintf(stdout, "fingerprint: %s\n\n", fp)
	}

	t := newTable(stdout, "KIND", "TYPE_ID", "NAME")
	schema := resp.GetSchema()
	if schema == nil {
		return t.Flush()
	}
	m := schema.AsMap()

	for _, raw := range asSlice(m["node_types"]) {
		nt, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		t.Row("node", asString(nt["type_id"]), asString(nt["name"]))
	}
	for _, raw := range asSlice(m["edge_types"]) {
		et, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		// Edge ids land under one of two keys depending on
		// the schema source — registry uses ``edge_id``,
		// data-driven fallback also uses ``edge_id``. Be
		// liberal in what we accept.
		id := et["edge_id"]
		if id == nil {
			id = et["edge_type_id"]
		}
		t.Row("edge", asString(id), asString(et["name"]))
	}
	return t.Flush()
}

func asSlice(v any) []any {
	s, _ := v.([]any)
	return s
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case float64:
		// structpb stores all numbers as float64 — render
		// integral values without the trailing ``.0``.
		if x == float64(int64(x)) {
			return fmt.Sprintf("%d", int64(x))
		}
		return fmt.Sprintf("%g", x)
	case bool:
		return fmt.Sprintf("%t", x)
	default:
		return fmt.Sprintf("%v", v)
	}
}
