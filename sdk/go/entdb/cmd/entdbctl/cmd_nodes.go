package main

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	pb "github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/pb"
)

func newNodesCmd() *cobra.Command {
	nodes := &cobra.Command{
		Use:   "nodes",
		Short: "List and fetch nodes",
	}

	var (
		typeArg string
		limit   int32
		offset  int32
	)
	listCmd := &cobra.Command{
		Use:   "list <tenant-id>",
		Short: "List nodes of a given type",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNodesList(args[0], typeArg, limit, offset)
		},
	}
	listCmd.Flags().StringVar(&typeArg, "type", "", "Node type (numeric type_id or registered name) [required]")
	listCmd.Flags().Int32Var(&limit, "limit", 50, "Maximum rows to return")
	listCmd.Flags().Int32Var(&offset, "offset", 0, "Offset into the result set")
	_ = listCmd.MarkFlagRequired("type")
	nodes.AddCommand(listCmd)

	getCmd := &cobra.Command{
		Use:   "get <tenant-id> <node-id>",
		Short: "Fetch a single node by id (always JSON)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNodesGet(args[0], args[1], typeArg)
		},
	}
	getCmd.Flags().StringVar(&typeArg, "type", "", "Node type (numeric type_id or registered name); optional but speeds up lookup")
	nodes.AddCommand(getCmd)

	return nodes
}

// resolveTypeID converts a CLI --type value into the numeric
// type_id the gRPC layer expects. A purely numeric input is
// passed through; a name is looked up via GetSchema.
func resolveTypeID(ctx context.Context, cli pb.EntDBServiceClient, tenantID, raw string) (int32, error) {
	if raw == "" {
		return 0, nil
	}
	if n, err := strconv.Atoi(raw); err == nil {
		return int32(n), nil
	}
	resp, err := cli.GetSchema(ctx, &pb.GetSchemaRequest{TenantId: tenantID})
	if err != nil {
		return 0, fmt.Errorf("resolve type %q via GetSchema: %w", raw, err)
	}
	schema := resp.GetSchema()
	if schema == nil {
		return 0, fmt.Errorf("schema is empty — cannot resolve type name %q", raw)
	}
	for _, item := range asSlice(schema.AsMap()["node_types"]) {
		nt, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if asString(nt["name"]) == raw {
			id, err := strconv.Atoi(asString(nt["type_id"]))
			if err != nil {
				return 0, fmt.Errorf("type %q has invalid type_id in schema", raw)
			}
			return int32(id), nil
		}
	}
	return 0, fmt.Errorf("type %q not found in tenant schema", raw)
}

func runNodesList(tenantID, typeArg string, limit, offset int32) error {
	cli, ctx, cancel, err := newClient()
	if err != nil {
		return err
	}
	defer cancel()

	typeID, err := resolveTypeID(ctx, cli, tenantID, typeArg)
	if err != nil {
		return err
	}

	resp, err := cli.QueryNodes(ctx, &pb.QueryNodesRequest{
		Context: reqContext(tenantID),
		TypeId:  typeID,
		Limit:   limit,
		Offset:  offset,
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
			payloadPreview(structpbAsMap(n), 80),
		)
	}
	if err := t.Flush(); err != nil {
		return err
	}
	if resp.GetHasMore() {
		fmt.Fprintln(stdout, "(more results — increase --limit or use --offset)")
	}
	return nil
}

// structpbAsMap pulls the payload out of a Node as a plain
// map[string]any. Returns nil if the node has no payload.
func structpbAsMap(n *pb.Node) map[string]any {
	if n == nil || n.GetPayload() == nil {
		return nil
	}
	return n.GetPayload().AsMap()
}

func runNodesGet(tenantID, nodeID, typeArg string) error {
	cli, ctx, cancel, err := newClient()
	if err != nil {
		return err
	}
	defer cancel()

	typeID, err := resolveTypeID(ctx, cli, tenantID, typeArg)
	if err != nil {
		return err
	}

	resp, err := cli.GetNode(ctx, &pb.GetNodeRequest{
		Context: reqContext(tenantID),
		TypeId:  typeID,
		NodeId:  nodeID,
	})
	if err != nil {
		return err
	}
	if !resp.GetFound() {
		return fmt.Errorf("node %q not found", nodeID)
	}
	// nodes get always emits JSON — payload is structured and
	// not amenable to a single-line tabular preview.
	return printJSON(stdout, resp)
}
