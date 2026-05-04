package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/testpb"
)

// writeTestProtoset materialises the in-package testpb file
// descriptor into a binary FileDescriptorSet on disk — same shape
// ``protoc --descriptor_set_out=FILE`` produces.
func writeTestProtoset(t *testing.T) string {
	t.Helper()
	fp := testpb.ProductDesc.ParentFile()
	fdp := protodesc.ToFileDescriptorProto(fp)
	fds := &descriptorpb.FileDescriptorSet{
		File: []*descriptorpb.FileDescriptorProto{fdp},
	}
	data, err := proto.Marshal(fds)
	if err != nil {
		t.Fatalf("marshal fds: %v", err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "schema.protoset")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write protoset: %v", err)
	}
	return path
}

// ── Happy path: stdout output ────────────────────────────────────────

func TestCmdSnapshot_StdoutHappyPath(t *testing.T) {
	protoset := writeTestProtoset(t)
	stdout, stderr, code := runCapture(
		"snapshot", "--proto-descriptor="+protoset,
	)
	if code != 0 {
		t.Fatalf("snapshot exit=%d, stderr=%s", code, stderr)
	}
	if stdout == "" {
		t.Fatal("snapshot wrote no stdout")
	}
	// Must be valid JSON in the wrapped envelope shape — same shape
	// the Python ``entdb-schema snapshot`` produces, so the same
	// schema.lock.json works on either side of the pipeline.
	var env map[string]any
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("snapshot stdout is not valid JSON: %v\n%s", err, stdout)
	}
	if env["version"] == nil || env["schema"] == nil {
		t.Errorf("missing wrapper keys: %v", env)
	}
	schema := env["schema"].(map[string]any)
	if len(schema["node_types"].([]any)) != 1 {
		t.Error("expected Product node in output")
	}
	if len(schema["edge_types"].([]any)) != 1 {
		t.Error("expected PurchaseEdge in output")
	}
}

// ── Happy path: -o writes to file ────────────────────────────────────

func TestCmdSnapshot_WritesOutputFile(t *testing.T) {
	protoset := writeTestProtoset(t)
	out := filepath.Join(t.TempDir(), "schema.lock.json")
	_, stderr, code := runCapture(
		"snapshot", "--proto-descriptor="+protoset, "-o", out,
	)
	if code != 0 {
		t.Fatalf("snapshot exit=%d, stderr=%s", code, stderr)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	var env map[string]any
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if !strings.Contains(stderr, out) {
		t.Errorf("stderr should announce the output path: %q", stderr)
	}
}

// ── Error: missing required flag ─────────────────────────────────────

func TestCmdSnapshot_RequiresProtoDescriptor(t *testing.T) {
	_, stderr, code := runCapture("snapshot")
	if code != 2 {
		t.Errorf("missing flag should exit 2, got %d", code)
	}
	if !strings.Contains(stderr, "--proto-descriptor") {
		t.Errorf("stderr should mention --proto-descriptor: %q", stderr)
	}
}

// ── Error: descriptor file doesn't exist ─────────────────────────────

func TestCmdSnapshot_NonexistentDescriptorFile(t *testing.T) {
	_, stderr, code := runCapture(
		"snapshot", "--proto-descriptor=/no/such/file.protoset",
	)
	if code != 1 {
		t.Errorf("missing file should exit 1, got %d", code)
	}
	if !strings.Contains(stderr, "read descriptor") {
		t.Errorf("stderr should mention read failure: %q", stderr)
	}
}

// ── Error: descriptor file is corrupt ────────────────────────────────

func TestCmdSnapshot_CorruptDescriptorFile(t *testing.T) {
	bad := filepath.Join(t.TempDir(), "garbage.protoset")
	_ = os.WriteFile(bad, []byte("this is not a protoset"), 0o644)

	_, stderr, code := runCapture(
		"snapshot", "--proto-descriptor="+bad,
	)
	if code != 1 {
		t.Errorf("corrupt file should exit 1, got %d", code)
	}
	if !strings.Contains(stderr, "parse descriptor") {
		t.Errorf("stderr should mention parse failure: %q", stderr)
	}
}

// ── Help text mentions the new subcommand ────────────────────────────

func TestHelpListsSnapshotCommand(t *testing.T) {
	out, _, _ := runCapture("help")
	if !strings.Contains(out, "snapshot") {
		t.Errorf("help output should list 'snapshot' subcommand: %q", out)
	}
}
