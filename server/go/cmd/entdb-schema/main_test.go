// Happy-path smoke tests for entdb-schema. These exercise the
// subcommand handlers directly (not via os.Exec) so they run cheaply
// and integrate with `go test ./...`.
//
// Coverage:
//   - snapshot --from-file → produces a valid envelope.
//   - check --baseline X --from-file X → exit 0 (no changes).
//   - check against a breaking-change registry → exit 1.
//   - diff between two snapshots → reports the expected breaking changes.
//   - validate over a clean snapshot → exit 0.

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleSchema = `{
  "node_types": [
    {
      "type_id": 1,
      "name": "User",
      "fields": [
        {"field_id": 1, "name": "email", "kind": "str", "required": true, "unique": true},
        {"field_id": 2, "name": "name", "kind": "str"}
      ]
    },
    {
      "type_id": 2,
      "name": "Task",
      "fields": [
        {"field_id": 1, "name": "title", "kind": "str", "required": true},
        {"field_id": 2, "name": "owner_id", "kind": "ref", "ref_type_id": 1}
      ]
    }
  ],
  "edge_types": [
    {"edge_id": 10, "name": "AssignedTo", "from_type_id": 2, "to_type_id": 1, "on_subject_exit": "both"}
  ]
}`

// breakingSchema flips field_id 1 on User from `str` to `int` — a
// genuine FIELD_KIND_CHANGED break (type-coercion of stored bytes is
// unsafe). Field renames alone are non-breaking per CLAUDE.md
// invariant #6 (field_id, not name, is the on-disk key), so this
// fixture deliberately combines a rename with a kind change to exercise
// the "really breaking" path through `check` / `diff`.
const breakingSchema = `{
  "node_types": [
    {
      "type_id": 1,
      "name": "User",
      "fields": [
        {"field_id": 1, "name": "phone", "kind": "int", "required": true, "unique": true},
        {"field_id": 2, "name": "name", "kind": "str"}
      ]
    },
    {
      "type_id": 2,
      "name": "Task",
      "fields": [
        {"field_id": 1, "name": "title", "kind": "str", "required": true},
        {"field_id": 2, "name": "owner_id", "kind": "ref", "ref_type_id": 1}
      ]
    }
  ],
  "edge_types": [
    {"edge_id": 10, "name": "AssignedTo", "from_type_id": 2, "to_type_id": 1, "on_subject_exit": "both"}
  ]
}`

func writeFile(t *testing.T, name, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

func TestSnapshotHappyPath(t *testing.T) {
	in := writeFile(t, "baseline.json", sampleSchema)
	out := filepath.Join(t.TempDir(), "snap.json")
	var stdout, stderr bytes.Buffer
	rc := cmdSnapshot([]string{"--from-file", in, "--output", out}, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("cmdSnapshot rc=%d stderr=%q", rc, stderr.String())
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var env map[string]any
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if env["version"] != float64(1) {
		t.Errorf("version = %v, want 1", env["version"])
	}
	fp, _ := env["fingerprint"].(string)
	if !strings.HasPrefix(fp, "sha256:") {
		t.Errorf("fingerprint = %q, want sha256: prefix", fp)
	}
	if _, ok := env["schema"]; !ok {
		t.Errorf("envelope missing schema body")
	}
}

func TestSnapshotIsDeterministic(t *testing.T) {
	in := writeFile(t, "baseline.json", sampleSchema)
	dir := t.TempDir()
	a := filepath.Join(dir, "a.json")
	b := filepath.Join(dir, "b.json")
	var stdout, stderr bytes.Buffer
	if rc := cmdSnapshot([]string{"--from-file", in, "--output", a}, &stdout, &stderr); rc != 0 {
		t.Fatalf("snapshot a: rc=%d %s", rc, stderr.String())
	}
	if rc := cmdSnapshot([]string{"--from-file", in, "--output", b}, &stdout, &stderr); rc != 0 {
		t.Fatalf("snapshot b: rc=%d %s", rc, stderr.String())
	}
	ab, _ := os.ReadFile(a)
	bb, _ := os.ReadFile(b)
	if !bytes.Equal(ab, bb) {
		t.Fatalf("snapshots differ:\n a=%s\n b=%s", ab, bb)
	}
}

func TestCheckCompatibleAgainstSelf(t *testing.T) {
	baseline := writeFile(t, "baseline.json", sampleSchema)
	var stdout, stderr bytes.Buffer
	rc := cmdCheck([]string{"--baseline", baseline, "--from-file", baseline}, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("check rc=%d stderr=%q stdout=%q", rc, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "OK") {
		t.Errorf("expected OK in output, got %q", stdout.String())
	}
}

func TestCheckDetectsBreakingChange(t *testing.T) {
	baseline := writeFile(t, "baseline.json", sampleSchema)
	current := writeFile(t, "current.json", breakingSchema)
	var stdout, stderr bytes.Buffer
	rc := cmdCheck([]string{"--baseline", baseline, "--from-file", current, "--format", "json"}, &stdout, &stderr)
	if rc != 1 {
		t.Fatalf("check rc=%d (want 1) stderr=%q stdout=%q", rc, stderr.String(), stdout.String())
	}
	var out struct {
		Compatible    bool `json:"compatible"`
		BreakingCount int  `json:"breaking_count"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("decode json output: %v\n%s", err, stdout.String())
	}
	if out.Compatible {
		t.Error("expected compatible=false")
	}
	if out.BreakingCount == 0 {
		t.Error("expected breaking_count > 0")
	}
}

func TestCheckAllowBreaking(t *testing.T) {
	baseline := writeFile(t, "baseline.json", sampleSchema)
	current := writeFile(t, "current.json", breakingSchema)
	var stdout, stderr bytes.Buffer
	rc := cmdCheck([]string{"--baseline", baseline, "--from-file", current, "--allow-breaking"}, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("check --allow-breaking rc=%d (want 0); stderr=%q", rc, stderr.String())
	}
}

func TestDiffReportsBreakingChange(t *testing.T) {
	oldP := writeFile(t, "old.json", sampleSchema)
	newP := writeFile(t, "new.json", breakingSchema)
	var stdout, stderr bytes.Buffer
	// Default: do not fail.
	rc := cmdDiff([]string{"--old", oldP, "--new", newP}, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("diff rc=%d (want 0 without --fail-on-breaking); stderr=%q", rc, stderr.String())
	}
	if !strings.Contains(stdout.String(), "BREAKING") {
		t.Errorf("expected BREAKING in diff output, got %q", stdout.String())
	}
	// With --fail-on-breaking.
	stdout.Reset()
	stderr.Reset()
	rc = cmdDiff([]string{"--old", oldP, "--new", newP, "--fail-on-breaking"}, &stdout, &stderr)
	if rc != 1 {
		t.Fatalf("diff --fail-on-breaking rc=%d (want 1)", rc)
	}
}

func TestValidateClean(t *testing.T) {
	in := writeFile(t, "snap.json", sampleSchema)
	var stdout, stderr bytes.Buffer
	rc := cmdValidate([]string{"--from-file", in}, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("validate rc=%d stderr=%q stdout=%q", rc, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "OK") {
		t.Errorf("expected OK, got %q", stdout.String())
	}
}

func TestCheckMissingBaseline(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := cmdCheck([]string{"--from-file", "irrelevant.json"}, &stdout, &stderr)
	if rc != 2 {
		t.Fatalf("expected rc=2 for missing --baseline, got %d", rc)
	}
}

func TestSnapshotMissingSource(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := cmdSnapshot(nil, &stdout, &stderr)
	if rc != 2 {
		t.Fatalf("expected rc=2 for missing source, got %d", rc)
	}
}
