package main

import (
	"strings"
	"testing"

	pb "github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/pb"
)

func TestSearch_Table(t *testing.T) {
	resetFlags()
	out, _, restore := captureOutput()
	defer restore()
	svc := startFake(t, &fakeCLIServer{
		searchResp: &pb.SearchMailboxResponse{
			Results: []*pb.MailboxSearchResult{
				{
					Item: &pb.MailboxItem{
						ItemId:       "m-1",
						SourceTypeId: 401,
						SourceNodeId: "msg-1",
						Snippet:      "Hello world",
						Ts:           1_700_000_000_000,
					},
					Rank: 0.875,
				},
			},
		},
	})

	if err := runSearch("acme", "hello", "alice", 25, 0); err != nil {
		t.Fatalf("runSearch: %v", err)
	}
	if svc.searchReq.GetUserId() != "alice" || svc.searchReq.GetQuery() != "hello" {
		t.Errorf("server saw req = %+v", svc.searchReq)
	}
	got := out.String()
	for _, want := range []string{"m-1", "msg-1", "Hello world", "0.875"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q: %s", want, got)
		}
	}
}
