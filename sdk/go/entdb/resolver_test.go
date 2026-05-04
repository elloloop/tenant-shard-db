package entdb

import (
	"strings"
	"testing"
)

func TestDNSTemplateResolver_DefaultPort(t *testing.T) {
	r := &DNSTemplateResolver{BaseDomain: "entdb.svc.cluster.local"}
	got, err := r.Resolve("node-a")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != "node-a.entdb.svc.cluster.local:50051" {
		t.Errorf("got %q, want node-a.entdb.svc.cluster.local:50051", got)
	}
}

func TestDNSTemplateResolver_ExplicitPort(t *testing.T) {
	r := &DNSTemplateResolver{BaseDomain: "internal.example.com", Port: 9000}
	got, err := r.Resolve("node-b")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != "node-b.internal.example.com:9000" {
		t.Errorf("got %q, want node-b.internal.example.com:9000", got)
	}
}

func TestDNSTemplateResolver_RejectsEmptyNode(t *testing.T) {
	r := &DNSTemplateResolver{BaseDomain: "example.com"}
	if _, err := r.Resolve(""); err == nil {
		t.Fatal("expected error for empty node id")
	}
}

func TestDNSTemplateResolver_RejectsEmptyDomain(t *testing.T) {
	r := &DNSTemplateResolver{}
	if _, err := r.Resolve("node-a"); err == nil {
		t.Fatal("expected error for empty base domain")
	}
}

func TestStaticMapResolver_Hit(t *testing.T) {
	r := &StaticMapResolver{Endpoints: map[string]string{
		"node-a": "10.0.0.1:50051",
		"node-b": "10.0.0.2:50051",
	}}
	got, err := r.Resolve("node-b")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != "10.0.0.2:50051" {
		t.Errorf("got %q, want 10.0.0.2:50051", got)
	}
}

func TestStaticMapResolver_Miss(t *testing.T) {
	r := &StaticMapResolver{Endpoints: map[string]string{
		"node-a": "10.0.0.1:50051",
	}}
	_, err := r.Resolve("node-z")
	if err == nil {
		t.Fatal("expected error for unmapped node")
	}
	if !strings.Contains(err.Error(), "node-z") {
		t.Errorf("error should mention node-z: %v", err)
	}
}
