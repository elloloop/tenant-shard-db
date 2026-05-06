package main

import (
	"strings"
	"testing"

	pb "github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/pb"
)

func TestTenantsList_Table(t *testing.T) {
	resetFlags()
	out, _, restore := captureOutput()
	defer restore()
	svc := startFake(t, &fakeCLIServer{
		listTResp: &pb.ListTenantsResponse{
			Tenants: []*pb.TenantInfo{
				{TenantId: "acme"},
				{TenantId: "globex"},
			},
		},
	})

	if err := runTenantsList(); err != nil {
		t.Fatalf("runTenantsList: %v", err)
	}
	if svc.listTReq == nil {
		t.Fatal("server saw no ListTenants request")
	}
	got := out.String()
	if !strings.Contains(got, "acme") || !strings.Contains(got, "globex") {
		t.Errorf("missing tenant ids in output: %s", got)
	}
	if !strings.Contains(got, "TENANT_ID") {
		t.Errorf("missing header in output: %s", got)
	}
}

func TestTenantsGet_FoundAndDetails(t *testing.T) {
	resetFlags()
	out, _, restore := captureOutput()
	defer restore()
	svc := startFake(t, &fakeCLIServer{
		getTResp: &pb.GetTenantResponse{
			Found: true,
			Tenant: &pb.TenantDetail{
				TenantId:  "acme",
				Name:      "Acme Corp",
				Status:    "active",
				Region:    "us-east-1",
				CreatedAt: 1_700_000_000_000,
			},
		},
	})

	if err := runTenantsGet("acme"); err != nil {
		t.Fatalf("runTenantsGet: %v", err)
	}
	if svc.getTReq.GetTenantId() != "acme" {
		t.Errorf("server saw tenant_id = %q", svc.getTReq.GetTenantId())
	}
	got := out.String()
	for _, want := range []string{"acme", "Acme Corp", "active", "us-east-1"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q: %s", want, got)
		}
	}
}

func TestTenantsGet_NotFoundIsError(t *testing.T) {
	resetFlags()
	_, _, restore := captureOutput()
	defer restore()
	startFake(t, &fakeCLIServer{getTResp: &pb.GetTenantResponse{Found: false}})

	err := runTenantsGet("missing")
	if err == nil {
		t.Fatal("expected not-found error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("err = %v", err)
	}
}
