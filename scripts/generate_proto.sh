#!/bin/bash
# Generate the Python SDK protobuf stubs from proto/entdb/v1/*.proto.
#
# This script generates ONLY the Python SDK stubs in
# sdk/python/entdb_sdk/_generated/. The historical Python server was
# retired (ADR-017) and its generated package no longer exists.
#
# The Go stubs are generated separately with `buf generate` (see
# server/go/internal/pb/generate.go and sdk/go/entdb/internal/pb/generate.go);
# `make proto` runs both this script and the Go regeneration.
#
# Requirements:
#   pip install grpcio-tools
#
# Usage:
#   ./scripts/generate_proto.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

PROTO_DIR="$ROOT_DIR/proto/entdb/v1"
SDK_OUT="$ROOT_DIR/sdk/python/entdb_sdk/_generated"

echo "Generating Python SDK protobuf code..."
echo "  Proto source: $PROTO_DIR"
echo "  SDK output:   $SDK_OUT"

mkdir -p "$SDK_OUT"

# Generate the service + message stubs for the SDK.
python -m grpc_tools.protoc \
    -I"$PROTO_DIR" \
    --python_out="$SDK_OUT" \
    --grpc_python_out="$SDK_OUT" \
    "$PROTO_DIR/entdb.proto"

# Generate entdb_options for the SDK (used by codegen to parse the entdb
# extension options from user .proto files).
SDK_OPTIONS_PROTO_DIR="$ROOT_DIR/sdk/python/entdb_sdk/proto"
python -m grpc_tools.protoc \
    -I"$SDK_OPTIONS_PROTO_DIR" \
    --python_out="$SDK_OUT" \
    "$SDK_OPTIONS_PROTO_DIR/entdb_options.proto"

# Fix imports in generated files (grpc_tools emits absolute imports).
# Change "import entdb_pb2" to "from . import entdb_pb2". sed form works
# on both macOS and Linux.
if [[ -f "$SDK_OUT/entdb_pb2_grpc.py" ]]; then
    sed -i '' 's/^import entdb_pb2/from . import entdb_pb2/' "$SDK_OUT/entdb_pb2_grpc.py" 2>/dev/null \
        || sed -i 's/^import entdb_pb2/from . import entdb_pb2/' "$SDK_OUT/entdb_pb2_grpc.py"
fi

# Note: __init__.py is maintained manually to track proto message exports.
# The generate script only regenerates the _pb2 and _pb2_grpc files. If you
# add new messages to the proto, update __init__.py manually.

echo "Done! Generated files:"
ls -la "$SDK_OUT"
