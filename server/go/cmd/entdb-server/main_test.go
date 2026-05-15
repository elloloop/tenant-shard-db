package main

import (
	"strings"
	"testing"

	"github.com/elloloop/tenant-shard-db/server/go/internal/testseed"
)

func TestSchemaRegistryForProfileNoneIsSchemaless(t *testing.T) {
	reg, err := schemaRegistryForProfile("none")
	if err != nil {
		t.Fatalf("schemaRegistryForProfile(none): %v", err)
	}
	if reg != nil {
		t.Fatalf("profile=none registry = %v, want nil for schema-less mode", reg)
	}
}

func TestSchemaRegistryForProfileContract(t *testing.T) {
	reg, err := schemaRegistryForProfile("contract")
	if err != nil {
		t.Fatalf("schemaRegistryForProfile(contract): %v", err)
	}
	if reg == nil {
		t.Fatal("profile=contract registry = nil, want populated registry")
	}
	if !reg.Frozen() {
		t.Fatal("profile=contract registry is not frozen")
	}
	if got := reg.NodeTypeByID(testseed.UserTypeID); got == nil || got.Name != "User" {
		t.Fatalf("contract User type = %#v, want User node type", got)
	}
	if got := reg.NodeTypeByID(999999); got != nil {
		t.Fatalf("unexpected unknown type in contract registry: %#v", got)
	}
}

func TestSchemaRegistryForProfileE2E(t *testing.T) {
	reg, err := schemaRegistryForProfile("e2e")
	if err != nil {
		t.Fatalf("schemaRegistryForProfile(e2e): %v", err)
	}
	if reg == nil {
		t.Fatal("profile=e2e registry = nil, want populated registry")
	}
	if !reg.Frozen() {
		t.Fatal("profile=e2e registry is not frozen")
	}
	if got := reg.NodeTypeByID(testseed.E2EUserTypeID); got == nil || got.Name != "User" {
		t.Fatalf("e2e User type = %#v, want User node type", got)
	}
}

func TestSchemaRegistryForProfileRejectsInvalidProfile(t *testing.T) {
	_, err := schemaRegistryForProfile("bogus")
	if err == nil {
		t.Fatal("schemaRegistryForProfile(bogus) err = nil, want error")
	}
	if !strings.Contains(err.Error(), `invalid profile "bogus"`) {
		t.Fatalf("schemaRegistryForProfile(bogus) err = %q, want invalid profile error", err)
	}
}
