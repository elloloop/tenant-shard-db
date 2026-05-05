package main

import (
	"strings"
	"testing"

	pb "github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/pb"
)

func TestHealth_Table(t *testing.T) {
	resetFlags()
	out, _, restore := captureOutput()
	defer restore()
	startFake(t, &fakeCLIServer{
		healthResp: &pb.HealthResponse{
			Healthy:    true,
			Version:    "1.2.3",
			Components: map[string]string{"wal": "kafka", "store": "sqlite"},
		},
	})

	if err := runHealth(); err != nil {
		t.Fatalf("runHealth: %v", err)
	}
	got := out.String()
	for _, want := range []string{"OK", "1.2.3", "wal: kafka", "store: sqlite"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q: %s", want, got)
		}
	}
}

func TestHealth_JSON(t *testing.T) {
	resetFlags()
	flags.format = "json"
	out, _, restore := captureOutput()
	defer restore()
	startFake(t, &fakeCLIServer{healthResp: &pb.HealthResponse{Healthy: true, Version: "9.9.9"}})

	if err := runHealth(); err != nil {
		t.Fatalf("runHealth: %v", err)
	}
	if !strings.Contains(out.String(), "\"version\"") || !strings.Contains(out.String(), "\"9.9.9\"") {
		t.Errorf("expected json version field; got %s", out.String())
	}
}
