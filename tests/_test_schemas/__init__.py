"""Test fixtures: compiled proto schemas used by the SDK v0.3 test suite.

The tests under ``tests/`` exercise the single-shape SDK API which
takes proto messages and proto classes everywhere. To avoid pulling
in the playground app or generating files at test-run time, we keep
a small ``test_schema.proto`` here with a couple of node + edge types
and check the generated ``test_schema_pb2.py`` into the tree.

Regenerating::

    cd tests/_test_schemas
    protoc \
      --python_out=. \
      --proto_path=. \
      --proto_path=../../sdk/entdb_sdk/proto \
      test_schema.proto
"""
