package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runCapture runs the CLI with args, returning stdout, stderr, and exit code.
func runCapture(args ...string) (string, string, int) {
	var out, errb bytes.Buffer
	code := run(args, &out, &errb)
	return out.String(), errb.String(), code
}

func TestVersionCommand(t *testing.T) {
	out, _, code := runCapture("version")
	if code != 0 {
		t.Fatalf("version exit=%d, want 0", code)
	}
	if !strings.Contains(out, "entdb ") {
		t.Errorf("version output missing banner: %q", out)
	}
	if !strings.Contains(out, Version) {
		t.Errorf("version output missing %q: %q", Version, out)
	}
}

func TestHelpCommand(t *testing.T) {
	for _, a := range []string{"help", "-h", "--help"} {
		out, _, code := runCapture(a)
		if code != 0 {
			t.Errorf("help(%s) exit=%d, want 0", a, code)
		}
		if !strings.Contains(out, "Usage:") {
			t.Errorf("help(%s) missing 'Usage:' in output", a)
		}
		if !strings.Contains(out, "lint") || !strings.Contains(out, "ping") {
			t.Errorf("help(%s) missing expected subcommands", a)
		}
	}
}

func TestNoArgsPrintsHelpAndExits2(t *testing.T) {
	_, errs, code := runCapture()
	if code != 2 {
		t.Errorf("no args exit=%d, want 2", code)
	}
	if !strings.Contains(errs, "Usage:") {
		t.Errorf("no args missing help in stderr: %q", errs)
	}
}

func TestUnknownCommand(t *testing.T) {
	_, errs, code := runCapture("frobnicate")
	if code != 2 {
		t.Errorf("unknown exit=%d, want 2", code)
	}
	if !strings.Contains(errs, "unknown command") {
		t.Errorf("unknown missing message: %q", errs)
	}
}

func TestLintMissingFile(t *testing.T) {
	_, errs, code := runCapture("lint", "/tmp/does-not-exist-entdb.proto")
	if code != 1 {
		t.Errorf("lint missing file exit=%d, want 1", code)
	}
	if !strings.Contains(errs, "lint") {
		t.Errorf("lint missing file stderr should mention lint: %q", errs)
	}
}

func TestLintMissingArg(t *testing.T) {
	_, errs, code := runCapture("lint")
	if code != 2 {
		t.Errorf("lint no arg exit=%d, want 2", code)
	}
	if !strings.Contains(errs, "missing") {
		t.Errorf("lint no arg should complain: %q", errs)
	}
}

// writeFixture writes a minimal proto fixture used by lint/check tests.
func writeFixture(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture.proto")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

const fixtureProto = `syntax = "proto3";

package fixture;

message User {
  option (entdb.node) = { type_id: 1 };
  string name = 1;
  int32 age = 2;
}

message Post {
  option (entdb.node) = { type_id: 2 };
  string title = 1;
}
`

const fixtureNoAnnotation = `syntax = "proto3";

package fixture;

message Plain {
  string name = 1;
}
`

func TestLintValidFixtureGracefulWithoutProtoc(t *testing.T) {
	path := writeFixture(t, fixtureProto)
	out, errs, code := runCapture("lint", path)

	// protoc may or may not be available in CI. Either outcome is valid
	// so long as the message is helpful.
	if code == 0 {
		if !strings.Contains(out, "node type") {
			t.Errorf("lint pass should print node-type summary: %q", out)
		}
	} else {
		if !strings.Contains(errs, "protoc") && !strings.Contains(errs, "entdb") {
			t.Errorf("lint fail should be actionable: %q", errs)
		}
	}
}

func TestLintNoAnnotationFails(t *testing.T) {
	path := writeFixture(t, fixtureNoAnnotation)
	_, errs, code := runCapture("lint", path)
	// Only assert on the "no annotation" branch when protoc is available;
	// otherwise we fail earlier with a protoc-missing error which is also
	// a valid exit=1.
	if code == 0 {
		t.Errorf("lint of unannotated proto should fail, got 0")
	}
	if !strings.Contains(errs, "entdb") {
		t.Errorf("lint stderr should be helpful: %q", errs)
	}
}

func TestCheckMissingFile(t *testing.T) {
	_, errs, code := runCapture("check", "/tmp/does-not-exist-entdb.proto")
	if code != 1 {
		t.Errorf("check missing file exit=%d, want 1", code)
	}
	if !strings.Contains(errs, "check") {
		t.Errorf("check missing file stderr should mention check: %q", errs)
	}
}

func TestCheckFixture(t *testing.T) {
	path := writeFixture(t, fixtureProto)
	_, _, code := runCapture("check", path)
	// Without protoc, this will return 1 with a helpful message — that's
	// expected. We just assert we don't crash.
	if code != 0 && code != 1 {
		t.Errorf("check exit=%d, want 0 or 1", code)
	}
}

func TestSummarizeFieldsParsesMessages(t *testing.T) {
	var buf bytes.Buffer
	summarizeFields(fixtureProto, &buf)
	s := buf.String()
	for _, want := range []string{"message User", "name", "age", "message Post", "title"} {
		if !strings.Contains(s, want) {
			t.Errorf("summarizeFields missing %q in %q", want, s)
		}
	}
}

func TestPingMissingAddress(t *testing.T) {
	_, errs, code := runCapture("ping")
	if code != 2 {
		t.Errorf("ping no address exit=%d, want 2", code)
	}
	if !strings.Contains(errs, "address") {
		t.Errorf("ping no address missing helpful stderr: %q", errs)
	}
}

func TestPingParsesFlags(t *testing.T) {
	// Use an address that should fail to dial quickly. We only care about
	// flag parsing — the exact error doesn't matter. Use a loopback port
	// unlikely to be listening.
	_, errs, code := runCapture(
		"ping",
		"127.0.0.1:1",
		"--api-key=abc",
		"--secure",
	)
	// We may succeed at "NewClient" + lazy-dial since grpc.NewClient is
	// non-blocking. Accept code 0 or 1 but make sure we didn't hit a
	// flag parse error (which would be exit 2).
	if code == 2 {
		t.Errorf("ping flag parse failed: %q", errs)
	}
}

func TestGetRequiresFlags(t *testing.T) {
	_, errs, code := runCapture("get", "127.0.0.1:1")
	if code != 2 {
		t.Errorf("get missing flags exit=%d, want 2", code)
	}
	if !strings.Contains(errs, "required") {
		t.Errorf("get missing flags should list required: %q", errs)
	}
}

func TestGetFlagParsing(t *testing.T) {
	// All flags present; descriptor file is missing so we expect
	// exit 1 (not a flag parse error). The key check is that
	// ``--type`` parses (not the removed ``--type-id``).
	_, _, code := runCapture(
		"get",
		"127.0.0.1:1",
		"--tenant=acme",
		"--actor=user:bob",
		"--proto-descriptor=/tmp/does-not-exist.protoset",
		"--type=Product",
		"--node-id=node-123",
	)
	if code == 2 {
		t.Errorf("get flag parse failed, exit=2")
	}
}

func TestGetRejectsRemovedTypeIDFlag(t *testing.T) {
	// The --type-id=N flag was removed as part of the 2026-04-14
	// SDK v0.3 decision: the CLI must not let users type raw
	// numeric type ids. Parsing it should fail with exit 2.
	_, _, code := runCapture(
		"get",
		"127.0.0.1:1",
		"--tenant=acme",
		"--actor=user:bob",
		"--type-id=201",
		"--node-id=node-123",
	)
	if code != 2 {
		t.Errorf("get with removed --type-id flag exit=%d, want 2", code)
	}
}

func TestQueryRequiresFlags(t *testing.T) {
	_, errs, code := runCapture("query", "127.0.0.1:1")
	if code != 2 {
		t.Errorf("query missing flags exit=%d, want 2", code)
	}
	if !strings.Contains(errs, "required") {
		t.Errorf("query missing flags should list required: %q", errs)
	}
}

func TestGitSHADoesNotPanic(t *testing.T) {
	sha := gitSHA()
	if sha == "" {
		t.Errorf("gitSHA returned empty string")
	}
}
