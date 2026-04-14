package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// TestPascalCase covers the snake_case → PascalCase conversion and
// initialism promotion that the Go emitter uses for variable names.
func TestPascalCase(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"sku", "SKU"},
		{"external_id", "ExternalID"},
		{"api_key", "APIKey"},
		{"http_url", "HTTPURL"},
		{"order_number", "OrderNumber"},
		{"customer_uuid", "CustomerUUID"},
		{"name", "Name"},
		{"json_payload", "JSONPayload"},
		{"http", "HTTP"},
		{"", ""},
		{"__double__", "Double"},
		{"user_id_key", "UserIDKey"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got := pascalCase(c.in)
			if got != c.want {
				t.Errorf("pascalCase(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// TestKindToPyGo exercises every scalar type in the mapping table.
func TestKindToPyGo(t *testing.T) {
	cases := []struct {
		kind    protoreflect.Kind
		py, go_ string
	}{
		{protoreflect.StringKind, "str", "string"},
		{protoreflect.Int32Kind, "int", "int32"},
		{protoreflect.Sint32Kind, "int", "int32"},
		{protoreflect.Sfixed32Kind, "int", "int32"},
		{protoreflect.Uint32Kind, "int", "uint32"},
		{protoreflect.Fixed32Kind, "int", "uint32"},
		{protoreflect.Int64Kind, "int", "int64"},
		{protoreflect.Sint64Kind, "int", "int64"},
		{protoreflect.Sfixed64Kind, "int", "int64"},
		{protoreflect.Uint64Kind, "int", "uint64"},
		{protoreflect.Fixed64Kind, "int", "uint64"},
		{protoreflect.FloatKind, "float", "float32"},
		{protoreflect.DoubleKind, "float", "float64"},
		{protoreflect.BoolKind, "bool", "bool"},
		{protoreflect.BytesKind, "bytes", "[]byte"},
	}
	for _, c := range cases {
		t.Run(c.kind.String(), func(t *testing.T) {
			py, gt, ok := kindToPyGo(c.kind)
			if !ok {
				t.Fatalf("kindToPyGo(%s) not ok", c.kind)
			}
			if py != c.py || gt != c.go_ {
				t.Errorf("kindToPyGo(%s) = (%q,%q), want (%q,%q)", c.kind, py, gt, c.py, c.go_)
			}
		})
	}

	// Unsupported kinds
	for _, k := range []protoreflect.Kind{protoreflect.MessageKind, protoreflect.EnumKind, protoreflect.GroupKind} {
		t.Run("unsupported_"+k.String(), func(t *testing.T) {
			if _, _, ok := kindToPyGo(k); ok {
				t.Errorf("kindToPyGo(%s) unexpectedly ok", k)
			}
		})
	}
}

// requireProtoc locates a protoc binary or skips the test.
func requireProtoc(t *testing.T) string {
	t.Helper()
	p, err := exec.LookPath("protoc")
	if err != nil {
		t.Skip("protoc not found in PATH; skipping end-to-end plugin tests")
	}
	return p
}

// buildPlugin builds the plugin binary into a tempdir and returns its path.
func buildPlugin(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "protoc-gen-entdb-keys")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
	return bin
}

// runPlugin invokes protoc with the plugin against the named proto files
// and returns (outputDir, combinedStdout+stderr, exitErr).
func runPlugin(t *testing.T, protoc, plugin string, protoFiles ...string) (string, string, error) {
	t.Helper()
	outDir := t.TempDir()
	args := []string{
		"--proto_path=testdata",
		"--plugin=protoc-gen-entdb-keys=" + plugin,
		"--entdb-keys_out=" + outDir,
	}
	args = append(args, protoFiles...)
	cmd := exec.Command(protoc, args...)
	out, err := cmd.CombinedOutput()
	return outDir, string(out), err
}

func readGolden(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read golden %s: %v", name, err)
	}
	return string(b)
}

func readOutput(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read output %s: %v", name, err)
	}
	return string(b)
}

// TestSampleProto verifies the end-to-end plugin output matches the
// golden files exactly. Exercises the primary example from the ADR.
// Also exercises skipping: NotANode (no entdb.node) and Category (node
// with no unique fields) must not appear in the output.
func TestSampleProto(t *testing.T) {
	protoc := requireProtoc(t)
	plugin := buildPlugin(t)
	outDir, out, err := runPlugin(t, protoc, plugin, "testdata/sample.proto")
	if err != nil {
		t.Fatalf("protoc failed: %v\n%s", err, out)
	}

	for _, f := range []string{"sample_entdb.py", "sample_entdb.go"} {
		got := readOutput(t, outDir, f)
		want := readGolden(t, f+".golden")
		if got != want {
			t.Errorf("%s mismatch\n--- got ---\n%s\n--- want ---\n%s", f, got, want)
		}
	}

	// Skip-check: output must not mention the non-node or the
	// empty-unique-set node.
	both := readOutput(t, outDir, "sample_entdb.py") + readOutput(t, outDir, "sample_entdb.go")
	for _, forbidden := range []string{"NotANode", "Category"} {
		if strings.Contains(both, forbidden) {
			t.Errorf("output should not contain %q; got:\n%s", forbidden, both)
		}
	}
}

// TestTypesProto checks that every scalar in the mapping table comes
// out the other side with the right Python and Go type parameter.
func TestTypesProto(t *testing.T) {
	protoc := requireProtoc(t)
	plugin := buildPlugin(t)
	outDir, out, err := runPlugin(t, protoc, plugin, "testdata/types.proto")
	if err != nil {
		t.Fatalf("protoc failed: %v\n%s", err, out)
	}
	for _, f := range []string{"types_entdb.py", "types_entdb.go"} {
		got := readOutput(t, outDir, f)
		want := readGolden(t, f+".golden")
		if got != want {
			t.Errorf("%s mismatch\n--- got ---\n%s\n--- want ---\n%s", f, got, want)
		}
	}
}

// TestInitialismsProto checks that snake_case field names with
// initialisms (SKU, ID, API, HTTP, URL, UUID) are rendered correctly
// in Go variable names.
func TestInitialismsProto(t *testing.T) {
	protoc := requireProtoc(t)
	plugin := buildPlugin(t)
	outDir, out, err := runPlugin(t, protoc, plugin, "testdata/initialisms.proto")
	if err != nil {
		t.Fatalf("protoc failed: %v\n%s", err, out)
	}
	for _, f := range []string{"initialisms_entdb.go", "initialisms_entdb.py"} {
		got := readOutput(t, outDir, f)
		want := readGolden(t, f+".golden")
		if got != want {
			t.Errorf("%s mismatch\n--- got ---\n%s\n--- want ---\n%s", f, got, want)
		}
	}
}

// TestBadNonScalarFails checks that marking a message-typed field
// unique causes the plugin to fail with a clear error message.
func TestBadNonScalarFails(t *testing.T) {
	protoc := requireProtoc(t)
	plugin := buildPlugin(t)
	_, out, err := runPlugin(t, protoc, plugin, "testdata/bad_nonscalar.proto")
	if err == nil {
		t.Fatalf("expected protoc to fail on non-scalar unique field, but it succeeded\n%s", out)
	}
	if !strings.Contains(out, "message-typed fields") {
		t.Errorf("expected error mentioning 'message-typed fields', got:\n%s", out)
	}
	if !strings.Contains(out, "BadMessageField.inner") {
		t.Errorf("expected error to name the offending field 'BadMessageField.inner', got:\n%s", out)
	}
}
