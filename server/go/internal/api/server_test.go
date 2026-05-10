package api

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/elloloop/tenant-shard-db/server/go/internal/pb"
)

// In Phase 0 every RPC must return codes.Unimplemented. This test
// pins that contract for a representative method so the next PR
// (which actually implements one) flips the assertion deliberately
// rather than silently regressing other methods.
func TestServer_HealthReturnsUnimplemented(t *testing.T) {
	t.Parallel()
	srv := New()

	_, err := srv.Health(context.Background(), &pb.HealthRequest{})
	if err == nil {
		t.Fatalf("Health: expected error, got nil")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("Health: error is not a grpc status: %v", err)
	}
	if st.Code() != codes.Unimplemented {
		t.Fatalf("Health: expected code Unimplemented, got %v", st.Code())
	}
}
