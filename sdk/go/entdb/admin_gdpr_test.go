package entdb

import (
	"context"
	"testing"

	pb "github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/pb"
)

// Tests for the v1.6 GDPR user-lifecycle surface on the Admin
// namespace: DeleteUser, CancelUserDeletion, ExportUserData,
// FreezeUser. Each gets request-shape + response-decode coverage.

func TestAdmin_DeleteUser_DefaultGracePropagatesAsZero(t *testing.T) {
	// graceDays=0 ⇒ the SDK forwards 0 to the server, which the
	// server interprets as "use the default". The SDK does NOT
	// substitute 30 — that would silently override an operator
	// who deliberately wants the server-side default.
	svc := &fakeServer{
		deleteUserResp: &pb.DeleteUserResponse{
			Success:     true,
			RequestedAt: 1_700_000_000_000,
			ExecuteAt:   1_700_000_000_000 + 30*86_400_000,
			Status:      "deletion_pending",
		},
	}
	tr := startFakeServer(t, svc)
	c := newClientWithTransport("addr", tr)

	res, err := c.Admin().DeleteUser(context.Background(), "system:admin", "alice", 0)
	if err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	if res.Status != "deletion_pending" {
		t.Errorf("status = %q, want deletion_pending", res.Status)
	}
	if res.ExecuteAtMs <= res.RequestedAtMs {
		t.Errorf("execute_at must be in the future relative to requested_at: %+v", res)
	}
	if svc.deleteUserReq.GetGraceDays() != 0 {
		t.Errorf("expected graceDays=0 forwarded, got %d", svc.deleteUserReq.GetGraceDays())
	}
}

func TestAdmin_DeleteUser_CustomGraceForwarded(t *testing.T) {
	svc := &fakeServer{
		deleteUserResp: &pb.DeleteUserResponse{Success: true, Status: "deletion_pending"},
	}
	tr := startFakeServer(t, svc)
	c := newClientWithTransport("addr", tr)

	if _, err := c.Admin().DeleteUser(context.Background(), "system:admin", "alice", 7); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	if svc.deleteUserReq.GetGraceDays() != 7 {
		t.Errorf("graceDays = %d, want 7", svc.deleteUserReq.GetGraceDays())
	}
}

func TestAdmin_CancelUserDeletion_HappyPath(t *testing.T) {
	svc := &fakeServer{
		cancelDeletionResp: &pb.CancelUserDeletionResponse{Success: true},
	}
	tr := startFakeServer(t, svc)
	c := newClientWithTransport("addr", tr)

	if err := c.Admin().CancelUserDeletion(context.Background(), "system:admin", "alice"); err != nil {
		t.Fatalf("CancelUserDeletion: %v", err)
	}
	if svc.cancelDeletionReq.GetUserId() != "alice" {
		t.Errorf("request shape wrong: %+v", svc.cancelDeletionReq)
	}
}

func TestAdmin_ExportUserData_ReturnsRawJSON(t *testing.T) {
	// The SDK is intentionally a pass-through on the export bundle
	// — the server defines the shape, the SDK returns it as a
	// string for the caller to feed to whatever exporter they
	// already have (zip, S3 upload, etc.).
	svc := &fakeServer{
		exportUserResp: &pb.ExportUserDataResponse{
			Success:    true,
			ExportJson: `{"user_id":"alice","tenants":{"acme":[]}}`,
		},
	}
	tr := startFakeServer(t, svc)
	c := newClientWithTransport("addr", tr)

	bundle, err := c.Admin().ExportUserData(context.Background(), "system:admin", "alice")
	if err != nil {
		t.Fatalf("ExportUserData: %v", err)
	}
	if bundle != `{"user_id":"alice","tenants":{"acme":[]}}` {
		t.Errorf("bundle mismatch: %q", bundle)
	}
}

func TestDbClient_Health_DecodesComponents(t *testing.T) {
	svc := &fakeServer{
		healthResp: &pb.HealthResponse{
			Healthy: true,
			Version: "1.6.0",
			Components: map[string]string{
				"wal":          "ok",
				"global_store": "ok",
			},
		},
	}
	tr := startFakeServer(t, svc)
	c := newClientWithTransport("addr", tr)

	h, err := c.Health(context.Background())
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if !h.Healthy || h.Version != "1.6.0" {
		t.Errorf("status mismatch: %+v", h)
	}
	if h.Components["wal"] != "ok" || h.Components["global_store"] != "ok" {
		t.Errorf("components mismatch: %+v", h.Components)
	}
}

func TestAdmin_FreezeUser_EnableThenDisable(t *testing.T) {
	svc := &fakeServer{
		freezeUserResp: &pb.FreezeUserResponse{Success: true, Status: "frozen"},
	}
	tr := startFakeServer(t, svc)
	c := newClientWithTransport("addr", tr)

	status, err := c.Admin().FreezeUser(context.Background(), "system:admin", "alice", true)
	if err != nil {
		t.Fatalf("FreezeUser(true): %v", err)
	}
	if status != "frozen" {
		t.Errorf("status = %q, want frozen", status)
	}
	if !svc.freezeUserReq.GetEnabled() {
		t.Error("expected enabled=true forwarded")
	}

	// Now unfreeze. Hand the fake a new response.
	svc.freezeUserResp = &pb.FreezeUserResponse{Success: true, Status: "active"}
	status, err = c.Admin().FreezeUser(context.Background(), "system:admin", "alice", false)
	if err != nil {
		t.Fatalf("FreezeUser(false): %v", err)
	}
	if status != "active" {
		t.Errorf("status = %q, want active", status)
	}
	if svc.freezeUserReq.GetEnabled() {
		t.Error("expected enabled=false forwarded for unfreeze")
	}
}
