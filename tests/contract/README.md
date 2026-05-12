# Cross-implementation contract tests

This directory is a standalone Go module (`tests/contract`) that holds
contract tests pinning byte-level / behavioural compatibility between
the Go server's public artifacts (right now the `entdb-schema` CLI) and
the historical Python implementations they replaced.

Tests here drive published CLIs as black-box subprocesses rather than
importing the server's `internal/` packages directly — the
internal-package rule would forbid the import from a sibling module
anyway, and the deliberate boundary keeps these tests honest:
"contract" means what users actually see.

## Running

From this directory:

```bash
go test ./...
```

By default each test builds `entdb-schema` from `../../server/go/cmd/entdb-schema`
on first call (cached for the test process). To run against a release
artifact instead:

```bash
ENTDB_SCHEMA_BIN=/path/to/entdb-schema go test ./...
```

## Fixtures

`fixtures/` holds canonical-shape snapshots captured against
reference schemas. They are committed verbatim and never regenerated
in test setup — any drift is a contract break and should fail loudly.

- `fixtures/python-snapshot-v1.json` — the Python-era snapshot envelope
  shape (`{version, fingerprint, schema}`) over the two-node + one-edge
  representative schema (User/Task/AssignedTo). Its `fingerprint`
  field is the SHA256 the deleted Python `schema_cli.py` computed for
  the same logical content. Issue #488 §Backward compatibility (rules
  11/12) requires this file validate as a baseline against the Go tool
  with zero edits and an identical Go-side fingerprint.
