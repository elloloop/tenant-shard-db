// --from-server: pull the registry from a running entdb-server via the
// GetSchema RPC. The server already serialises the registry into a
// google.protobuf.Struct that round-trips through the canonical JSON
// body. We marshal the Struct back to JSON, then feed that into the
// same loader the file path uses.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
	"github.com/elloloop/tenant-shard-db/server/go/internal/schema"
)

const defaultServerTimeout = 10 * time.Second

// loadFromServer dials addr (e.g. "localhost:50051"), calls GetSchema,
// and reconstructs the registry. Returns the registry plus the
// server-reported fingerprint.
func loadFromServer(addr string) (*schema.Registry, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultServerTimeout)
	defer cancel()
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, "", fmt.Errorf("dial %s: %w", addr, err)
	}
	defer func() { _ = conn.Close() }()

	client := pb.NewEntDBServiceClient(conn)
	resp, err := client.GetSchema(ctx, &pb.GetSchemaRequest{})
	if err != nil {
		return nil, "", fmt.Errorf("GetSchema against %s: %w", addr, err)
	}
	if resp.GetSchema() == nil {
		return nil, "", fmt.Errorf("server returned empty schema struct")
	}
	body, err := json.Marshal(resp.GetSchema().AsMap())
	if err != nil {
		return nil, "", fmt.Errorf("marshal server schema: %w", err)
	}
	r, err := schema.LoadFromJSON(body)
	if err != nil {
		return nil, "", fmt.Errorf("load server schema: %w", err)
	}
	if _, err := r.Freeze(); err != nil {
		return nil, "", fmt.Errorf("freeze server schema: %w", err)
	}
	return r, resp.GetFingerprint(), nil
}
