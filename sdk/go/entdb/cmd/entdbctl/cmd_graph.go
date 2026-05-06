package main

import (
	"fmt"

	"github.com/spf13/cobra"

	pb "github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/pb"
)

func newGraphCmd() *cobra.Command {
	var (
		typeArg string
		limit   int32
	)
	cmd := &cobra.Command{
		Use:   "graph <tenant-id> <node-id>",
		Short: "Show nodes connected to <node-id> (one hop)",
		Long: `Show nodes connected to <node-id> via the GetConnectedNodes RPC.
The server currently returns one-hop neighbours; --depth is reserved
for future server support and is silently ignored today.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGraph(args[0], args[1], typeArg, limit)
		},
	}
	cmd.Flags().StringVar(&typeArg, "type", "", "Filter by edge type (numeric edge_type_id or name)")
	cmd.Flags().Int32Var(&limit, "limit", 50, "Maximum nodes to return")
	// --depth accepted but unused; keep the flag so scripts that
	// pass it don't break the moment we add multi-hop support.
	cmd.Flags().Int32("depth", 1, "(reserved) traversal depth — currently always 1")
	return cmd
}

func runGraph(tenantID, nodeID, typeArg string, limit int32) error {
	cli, ctx, cancel, err := newClient()
	if err != nil {
		return err
	}
	defer cancel()

	edgeTypeID, err := resolveEdgeTypeID(ctx, cli, tenantID, typeArg)
	if err != nil {
		return err
	}

	resp, err := cli.GetConnectedNodes(ctx, &pb.GetConnectedNodesRequest{
		Context:    reqContext(tenantID),
		NodeId:     nodeID,
		EdgeTypeId: edgeTypeID,
		Limit:      limit,
	})
	if err != nil {
		return err
	}
	if flags.format == "json" {
		return printJSON(stdout, resp)
	}
	t := newTable(stdout, "NODE_ID", "TYPE", "OWNER", "CREATED_AT", "PAYLOAD")
	for _, n := range resp.GetNodes() {
		t.Row(
			n.GetNodeId(),
			fmt.Sprintf("%d", n.GetTypeId()),
			n.GetOwnerActor(),
			formatTime(n.GetCreatedAt()),
			payloadPreview(structpbAsMap(n), 60),
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
