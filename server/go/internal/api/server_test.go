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
// embedded UnimplementedEntDBServiceServer. We probe `ExecuteAtomic`
// because it's still Unimplemented as of this PR; rotate when it ships.
func TestServer_UnimplementedRPC_Default(t *testing.T) {
	t.Parallel()
	srv := New()

	_, err := srv.ExecuteAtomic(context.Background(), &pb.ExecuteAtomicRequest{})
	if err == nil {
		t.Fatalf("ExecuteAtomic: expected Unimplemented error, got nil")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("ExecuteAtomic: error is not a grpc status: %v", err)
	}
	if st.Code() != codes.Unimplemented {
		t.Fatalf("ExecuteAtomic: expected code Unimplemented, got %v", st.Code())
	}
}
