// Cross-implementation contract test for the schema-snapshot envelope.
//
// The snapshot envelope on disk has the shape:
//
//	{
//	  "version": 1,
//	  "fingerprint": "sha256:…",
//	  "schema": { "node_types": [...], "edge_types": [...] }
//	}
//
// ADR-031 made the schema JSON contract NAME-FREE (ids at rest and on the
// wire — no field/type/constraint names; `subject_field` is a field_id;
// composite uniqueness is a field_id tuple). That was a deliberate
// breaking change, so it SUPERSEDES the issue #488 §Backward-compat
// "load a Python-era name-ful snapshot with zero edits" guarantee — a
// name-ful snapshot no longer parses, by design.
//
// What this test still pins, end-to-end against the built CLI binary:
//  1. round-trip parity — re-emitting the fixture's `schema` body via
//     `snapshot --from-file` recomputes the SAME `fingerprint` the
//     envelope carries (the Go canonical encoder is stable); and
//  2. self-consistency — `check`/`diff` of the fixture against itself
//     report compatible / exit 0.
//
// The fixture `fixtures/python-snapshot-v1.json` is the canonical
// NAME-FREE envelope for the representative two-node + one-edge schema
// used by `pythonSampleJSON` in server/go/internal/schema/schema_test.go;
// its fingerprint matches that file's `pythonReferenceFingerprint`
// (`sha256:6179549ada4a9357…`). Regenerate both together if the
// canonical encoder changes.
//
// Lives in `tests/contract/` (a sibling Go module) so cross-
// implementation contract tests are physically separated from the
// server's own unit tests, per issue #488 §Test surface. The test
// drives `entdb-schema` as a black-box subprocess rather than
// importing the server's internal/ packages directly (which Go's
// internal-package rule forbids from a sibling module — that
// boundary is intentional: contract tests should consume the same
// public CLI users do).
package contract

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// snapshotEnvelope is the on-disk shape `entdb-schema snapshot` writes
// and that Python-era `.schema-snapshot.json` files use verbatim.
type snapshotEnvelope struct {
	Version     int             `json:"version"`
	Fingerprint string          `json:"fingerprint"`
	Schema      json.RawMessage `json:"schema"`
}

var (
	buildBinaryOnce sync.Once
	binaryPath      string
	binaryErr       error
)

// entdbSchemaBinary returns the path to a freshly-built `entdb-schema`
// binary, building it from server/go/cmd/entdb-schema on first call.
// The result is cached for the lifetime of the test process.
//
// An ENTDB_SCHEMA_BIN env var overrides the build step (useful when
// running against a release artifact in CI — see issue #488 AC7).
func entdbSchemaBinary(t *testing.T) string {
	t.Helper()
	buildBinaryOnce.Do(func() {
		if p := os.Getenv("ENTDB_SCHEMA_BIN"); p != "" {
			binaryPath = p
			return
		}
		// Find the repo root by walking up from the test file. The
		// test file lives at <repo>/tests/contract/schema_python_parity_test.go.
		_, thisFile, _, ok := runtime.Caller(0)
		if !ok {
			binaryErr = errors.New("cannot resolve caller file path")
			return
		}
		repoRoot, err := filepath.Abs(filepath.Join(filepath.Dir(thisFile), "..", ".."))
		if err != nil {
			binaryErr = fmt.Errorf("resolve repo root: %w", err)
			return
		}
		dir, err := os.MkdirTemp("", "entdb-schema-bin-")
		if err != nil {
			binaryErr = fmt.Errorf("mkdir tmp: %w", err)
			return
		}
		out := filepath.Join(dir, "entdb-schema")
		cmd := exec.Command("go", "build", "-o", out, "./cmd/entdb-schema")
		cmd.Dir = filepath.Join(repoRoot, "server", "go")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			binaryErr = fmt.Errorf("go build entdb-schema: %w\n%s", err, stderr.String())
			return
		}
		binaryPath = out
	})
	if binaryErr != nil {
		t.Fatalf("entdb-schema binary unavailable: %v", binaryErr)
	}
	return binaryPath
}

// fixturePath returns an absolute path to a file under tests/contract/fixtures/.
func fixturePath(t *testing.T, name string) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("fixtures", name))
	if err != nil {
		t.Fatalf("resolve fixture %q: %v", name, err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("fixture %q not readable: %v", p, err)
	}
	return p
}

// runEntdbSchema invokes the entdb-schema binary with args and returns
// (stdout, stderr, exit code).
func runEntdbSchema(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	bin := entdbSchemaBinary(t)
	cmd := exec.Command(bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			code = ee.ExitCode()
		} else {
			t.Fatalf("run entdb-schema %v: %v\n%s", args, err, stderr.String())
		}
	}
	return stdout.String(), stderr.String(), code
}

// TestPythonSnapshotFingerprintParity loads the Python-era envelope
// fixture, replays its `schema` body through the Go CLI's
// `snapshot --from-file` (which itself goes through LoadFromJSON +
// Freeze + canonical-encode), and asserts the Go-computed
// fingerprint byte-equals the fingerprint the Python tool wrote into
// the envelope.
//
// This is the migration-path guarantee from issue #488 §Backward
// compatibility (rule 12).
func TestPythonSnapshotFingerprintParity(t *testing.T) {
	path := fixturePath(t, "python-snapshot-v1.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var env snapshotEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if env.Version != 1 {
		t.Fatalf("fixture envelope version = %d, want 1", env.Version)
	}
	if env.Fingerprint == "" {
		t.Fatal("fixture envelope missing fingerprint")
	}
	if !strings.HasPrefix(env.Fingerprint, "sha256:") {
		t.Fatalf("fixture envelope fingerprint missing sha256: prefix: %q", env.Fingerprint)
	}

	// Re-emit the snapshot using the Go CLI. The output is a fresh
	// envelope whose `fingerprint` field is recomputed by the Go
	// canonical encoder.
	stdout, stderr, code := runEntdbSchema(t, "snapshot", "--from-file", path)
	if code != 0 {
		t.Fatalf("entdb-schema snapshot exit=%d, stderr=%s", code, stderr)
	}
	var re snapshotEnvelope
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &re); err != nil {
		t.Fatalf("decode re-emitted envelope: %v\noutput=%s", err, stdout)
	}
	if re.Fingerprint != env.Fingerprint {
		t.Errorf(
			"fingerprint mismatch:\n  Go     = %s\n  Python = %s\n"+
				"This means the Go canonical encoder has drifted from\n"+
				"Python's json.dumps(sort_keys=True, separators=(\",\", \":\")).\n"+
				"See server/go/internal/schema/fingerprint.go.",
			re.Fingerprint, env.Fingerprint,
		)
	}
}

// TestPythonSnapshotIsCompatibleWithItself sanity-checks the schema
// compatibility engine end-to-end: invoking `entdb-schema check`
// with the same fixture as both baseline and current source must
// exit 0 (compatible). This is the simplest form of the "no breaking
// changes on a no-op PR" CI contract from issue #488 §CI check design,
// and the migration-path guarantee from rule 11.
func TestPythonSnapshotIsCompatibleWithItself(t *testing.T) {
	path := fixturePath(t, "python-snapshot-v1.json")
	stdout, stderr, code := runEntdbSchema(t,
		"check",
		"--baseline", path,
		"--from-file", path,
		"--format", "json",
	)
	if code != 0 {
		t.Fatalf("entdb-schema check exit=%d, want 0\nstderr=%s\nstdout=%s",
			code, stderr, stdout)
	}
	var result struct {
		Compatible       bool `json:"compatible"`
		BreakingCount    int  `json:"breaking_count"`
		NonBreakingCount int  `json:"non_breaking_count"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode check output: %v\nstdout=%s", err, stdout)
	}
	if !result.Compatible {
		t.Errorf("compatible=false on identical baseline, want true")
	}
	if result.BreakingCount != 0 {
		t.Errorf("breaking_count=%d, want 0", result.BreakingCount)
	}
	if result.NonBreakingCount != 0 {
		t.Errorf("non_breaking_count=%d, want 0 (identical input)", result.NonBreakingCount)
	}
}

// TestPythonSnapshotDiffAgainstItselfExitsZero is the file-vs-file
// counterpart: `entdb-schema diff --old X --new X --fail-on-breaking`
// must exit 0 against an identical pair. This is the exact incantation
// the schema-compat CI workflow runs on every PR.
func TestPythonSnapshotDiffAgainstItselfExitsZero(t *testing.T) {
	path := fixturePath(t, "python-snapshot-v1.json")
	_, stderr, code := runEntdbSchema(t,
		"diff",
		"--old", path,
		"--new", path,
		"--format", "text",
		"--fail-on-breaking",
	)
	if code != 0 {
		t.Fatalf("entdb-schema diff exit=%d, want 0\nstderr=%s", code, stderr)
	}
}
