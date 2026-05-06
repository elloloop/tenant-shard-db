"""Test fixtures: proto schemas used by the SDK v0.3 test suite and codegen tests.

The tests under ``tests/`` exercise the single-shape SDK API which
takes proto messages and proto classes everywhere. To avoid generating
files at test-run time, we keep a small ``test_schema.proto`` here with
a couple of node + edge types and check the generated
``test_schema_pb2.py`` into the tree.

``playground_schema.proto`` is a larger sample (User/Project/Task/Comment
+ several edge types) used by the proto-options and codegen test suites
as a representative real-world schema. It carries no application
behaviour — it's a fixture only.

Regenerating ``test_schema_pb2.py``::

    cd tests/_test_schemas
    protoc \
      --python_out=. \
      --proto_path=. \
      --proto_path=../../sdk/entdb_sdk/proto \
      test_schema.proto
"""
