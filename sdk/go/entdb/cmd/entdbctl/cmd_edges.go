package main

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	pb "github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/pb"
)

func newEdgesCmd() *cobra.Command {
	var (
		dirIn   bool
		dirOut  bool
		typeArg string
		limit   int32
	)
	cmd := &cobra.Command{
		Use:   "edges <tenant-id> <node-id>",
		Short: "List edges incident to a node",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dirIn && dirOut {
				return fmt.Errorf("pass at most one of --in or --out")
			}
			direction := "out"
			if dirIn {
				direction = "in"
			}
			return runEdges(args[0], args[1], direction, typeArg, limit)
		},
	}
	cmd.Flags().BoolVar(&dirOut, "out", false, "Outgoing edges (default)")
	cmd.Flags().BoolVar(&dirIn, "in", false, "Incoming edges")
	cmd.Flags().StringVar(&typeArg, "type", "", "Edge type (numeric edge_type_id or registered name)")
	cmd.Flags().Int32Var(&limit, "limit", 50, "Maximum rows to return")
	return cmd
}

// resolveEdgeTypeID mirrors resolveTypeID for edge types,
// which live under “edge_types“ in the schema struct.
func resolveEdgeTypeID(ctx context.Context, cli pb.EntDBServiceClient, tenantID, raw string) (int32, error) {
	if raw == "" {
		return 0, nil
	}
	if n, err := strconv.Atoi(raw); err == nil {
		return int32(n), nil
	}
	resp, err := cli.GetSchema(ctx, &pb.GetSchemaRequest{TenantId: tenantID})
	if err != nil {
		return 0, fmt.Errorf("resolve edge type %q: %w", raw, err)
	}
	schema := resp.GetSchema()
	if schema == nil {
		return 0, fmt.Errorf("schema empty — cannot resolve edge type %q", raw)
	}
	for _, item := range asSlice(schema.AsMap()["edge_types"]) {
		et, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if asString(et["name"]) == raw {
			idStr := asString(et["edge_id"])
			if idStr == "" {
				idStr = asString(et["edge_type_id"])
			}
			id, err := strconv.Atoi(idStr)
			if err != nil {
				return 0, fmt.Errorf("edge type %q has invalid id in schema", raw)
			}
			return int32(id), nil
		}
	}
	return 0, fmt.Errorf("edge type %q not found in tenant schema", raw)
}

func runEdges(tenantID, nodeID, direction, typeArg string, limit int32) error {
	cli, ctx, cancel, err := newClient()
	if err != nil {
		return err
	}
	defer cancel()

	edgeTypeID, err := resolveEdgeTypeID(ctx, cli, tenantID, typeArg)
	if err != nil {
		return err
	}

	req := &pb.GetEdgesRequest{
		Context:    reqContext(tenantID),
		NodeId:     nodeID,
		EdgeTypeId: edgeTypeID,
		Limit:      limit,
	}

	var resp *pb.GetEdgesResponse
	if direction == "in" {
		resp, err = cli.GetEdgesTo(ctx, req)
	} else {
		resp, err = cli.GetEdgesFrom(ctx, req)
	}
	if err != nil {
		return err
	}
	if flags.format == "json" {
		return printJSON(stdout, resp)
	}
	t := newTable(stdout, "FROM", "TO", "EDGE_TYPE", "CREATED_AT", "PROPS")
	for _, e := range resp.GetEdges() {
		var props map[string]any
		if e.GetProps() != nil {
			props = e.GetProps().AsMap()
		}
		t.Row(
			e.GetFromNodeId(),
			e.GetToNodeId(),
			fmt.Sprintf("%d", e.GetEdgeTypeId()),
			formatTime(e.GetCreatedAt()),
			payloadPreview(props, 60),
		)
	}
	if err := t.Flush(); err != nil {
		return err
	}
	if resp.GetHasMore() {
		fmt.Fprintln(stdout, "(more results — increase --limit)")
	}
	return nil
}
