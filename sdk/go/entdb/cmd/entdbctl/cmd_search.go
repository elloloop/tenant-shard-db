package main

import (
	"fmt"

	"github.com/spf13/cobra"

	pb "github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/pb"
)

func newSearchCmd() *cobra.Command {
	var (
		userID string
		limit  int32
		offset int32
	)
	cmd := &cobra.Command{
		Use:   "search <tenant-id> <query>",
		Short: "Full-text search a user's mailbox",
		Long: `Search runs against the per-user mailbox FTS index — the only
search RPC the server currently exposes. Pass --user to scope
the query; the actor (--actor / $ENTDB_ACTOR) is checked for
permission to read that user's mailbox.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if userID == "" {
				return fmt.Errorf("--user is required (mailbox search is per-user)")
			}
			return runSearch(args[0], args[1], userID, limit, offset)
		},
	}
	cmd.Flags().StringVar(&userID, "user", "", "User whose mailbox to search [required]")
	cmd.Flags().Int32Var(&limit, "limit", 25, "Maximum results")
	cmd.Flags().Int32Var(&offset, "offset", 0, "Pagination offset")
	return cmd
}

func runSearch(tenantID, query, userID string, limit, offset int32) error {
	cli, ctx, cancel, err := newClient()
	if err != nil {
		return err
	}
	defer cancel()

	resp, err := cli.SearchMailbox(ctx, &pb.SearchMailboxRequest{
		Context: reqContext(tenantID),
		UserId:  userID,
		Query:   query,
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		return err
	}
	if flags.format == "json" {
		return printJSON(stdout, resp)
	}
	t := newTable(stdout, "ITEM_ID", "SOURCE_TYPE", "SOURCE_NODE", "TS", "RANK", "SNIPPET")
	for _, r := range resp.GetResults() {
		item := r.GetItem()
		t.Row(
			item.GetItemId(),
			fmt.Sprintf("%d", item.GetSourceTypeId()),
			item.GetSourceNodeId(),
			formatTime(item.GetTs()),
			fmt.Sprintf("%.3f", r.GetRank()),
			truncate(item.GetSnippet(), 80),
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
