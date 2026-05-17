# EntDB examples

Runnable, CI-tested examples for the EntDB Python SDK. Every script
here is executed end-to-end against a real `entdb-server` in CI
(`pytest examples/`), so the snippet you copy is the snippet that was
verified — no stale docs (issue #40).

## Run one

```bash
# from the repo root, with the SDK importable
python examples/quickstart.py
```

Each script boots a throwaway Go `entdb-server` (in-memory WAL,
`contract` seed profile), runs against it, and tears it down. Set
`ENTDB_GO_BINARY=/path/to/entdb-server` to skip the per-run `go build`.

## Run them all (as tests)

```bash
pytest examples/ --timeout=120
```

One server boot is shared across all examples.

## The examples

| File | Shows |
|---|---|
| `quickstart.py` | connect, atomic create, read-by-id, query |
| `read_after_write.py` | `wait_applied=True` vs explicit `after_offset` |
| `acl_sharing.py` | share / `shared_with_me` / revoke (ADR-003) |
| `graph_edges.py` | atomic node+node+edge, `edges_out`/`edges_in`/`connected` |

## Schema

`example_schema.proto` mirrors the server's `contract` seed registry
(`User` type 1, `Task` type 2, `AssignedTo` edge 100) so the
high-level proto-message SDK API works against the test harness.
Regenerate the Python stub after editing it:

```bash
python -m grpc_tools.protoc \
  --python_out=examples \
  --proto_path=examples \
  --proto_path=sdk/python/entdb_sdk/proto \
  examples/example_schema.proto
# then rewrite the entdb_options import to the SDK-shipped module:
#   from entdb_sdk._generated import entdb_options_pb2 as entdb__options__pb2
```

(`fastapi_app/` is a separate, larger sample app — not part of the
tested example suite.)
