package api

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

// TestServer_UnimplementedRPC_Default pins the Wave-1 contract: any RPC
// that hasn't been ported yet still returns codes.Unimplemented via the
// embedded UnimplementedEntDBServiceServer. We probe a representative
// not-yet-ported method (GetSchema) rather than Health because Health
// has now landed (W2.01) and returns OK.
func TestServer_UnimplementedRPC_Default(t *testing.T) {
	t.Parallel()
	srv := New()

	_, err := srv.GetSchema(context.Background(), &pb.GetSchemaRequest{})
	if err == nil {
		t.Fatalf("GetSchema: expected Unimplemented error, got nil")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("GetSchema: error is not a grpc status: %v", err)
	}
	if st.Code() != codes.Unimplemented {
		t.Fatalf("GetSchema: expected code Unimplemented, got %v", st.Code())
	}
}
