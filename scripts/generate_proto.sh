#!/bin/bash
# Generate Python protobuf code from .proto files
#
# This script generates:
#   - Server stubs in dbaas/entdb_server/api/generated/
#   - SDK stubs in sdk/entdb_sdk/_generated/
#
# Requirements:
#   pip install grpcio-tools
#
# Usage:
#   ./scripts/generate_proto.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"

PROTO_DIR="$ROOT_DIR/dbaas/entdb_server/api/proto"
SERVER_OUT="$ROOT_DIR/dbaas/entdb_server/api/generated"
SDK_OUT="$ROOT_DIR/sdk/entdb_sdk/_generated"

echo "Generating protobuf code..."
echo "  Proto source: $PROTO_DIR"
echo "  Server output: $SERVER_OUT"
echo "  SDK output: $SDK_OUT"

# Create output directories
mkdir -p "$SERVER_OUT"
mkdir -p "$SDK_OUT"

# Generate for server
python -m grpc_tools.protoc \
    -I"$PROTO_DIR" \
    --python_out="$SERVER_OUT" \
    --grpc_python_out="$SERVER_OUT" \
    "$PROTO_DIR/entdb.proto"

# Generate for SDK
python -m grpc_tools.protoc \
    -I"$PROTO_DIR" \
    --python_out="$SDK_OUT" \
    --grpc_python_out="$SDK_OUT" \
    "$PROTO_DIR/entdb.proto"

# Fix imports in generated files (grpc_tools generates absolute imports)
# Change "import entdb_pb2" to "from . import entdb_pb2"
for dir in "$SERVER_OUT" "$SDK_OUT"; do
    if [[ -f "$dir/entdb_pb2_grpc.py" ]]; then
        sed -i 's/^import entdb_pb2/from . import entdb_pb2/' "$dir/entdb_pb2_grpc.py"
    fi
done

# Create __init__.py files
cat > "$SERVER_OUT/__init__.py" << 'EOF'
"""Generated protobuf code for EntDB server.

Do not edit manually - regenerate with scripts/generate_proto.sh
"""

from .entdb_pb2 import (
    # Common
    RequestContext,
    Receipt,
    # Execute
    ExecuteAtomicRequest,
    ExecuteAtomicResponse,
    Operation,
    CreateNodeOp,
    UpdateNodeOp,
    DeleteNodeOp,
    CreateEdgeOp,
    DeleteEdgeOp,
    NodeRef,
    TypedNodeRef,
    # Receipt
    GetReceiptStatusRequest,
    GetReceiptStatusResponse,
    ReceiptStatus,
    # Nodes
    GetNodeRequest,
    GetNodeResponse,
    GetNodesRequest,
    GetNodesResponse,
    QueryNodesRequest,
    QueryNodesResponse,
    Node,
    # Edges
    GetEdgesRequest,
    GetEdgesResponse,
    Edge,
    # Mailbox
    SearchMailboxRequest,
    SearchMailboxResponse,
    MailboxSearchResult,
    GetMailboxRequest,
    GetMailboxResponse,
    MailboxItem,
    # Health/Schema
    HealthRequest,
    HealthResponse,
    GetSchemaRequest,
    GetSchemaResponse,
)
from .entdb_pb2_grpc import (
    EntDBServiceServicer,
    EntDBServiceStub,
    add_EntDBServiceServicer_to_server,
)

__all__ = [
    # Common
    "RequestContext",
    "Receipt",
    # Execute
    "ExecuteAtomicRequest",
    "ExecuteAtomicResponse",
    "Operation",
    "CreateNodeOp",
    "UpdateNodeOp",
    "DeleteNodeOp",
    "CreateEdgeOp",
    "DeleteEdgeOp",
    "NodeRef",
    "TypedNodeRef",
    # Receipt
    "GetReceiptStatusRequest",
    "GetReceiptStatusResponse",
    "ReceiptStatus",
    # Nodes
    "GetNodeRequest",
    "GetNodeResponse",
    "GetNodesRequest",
    "GetNodesResponse",
    "QueryNodesRequest",
    "QueryNodesResponse",
    "Node",
    # Edges
    "GetEdgesRequest",
    "GetEdgesResponse",
    "Edge",
    # Mailbox
    "SearchMailboxRequest",
    "SearchMailboxResponse",
    "MailboxSearchResult",
    "GetMailboxRequest",
    "GetMailboxResponse",
    "MailboxItem",
    # Health/Schema
    "HealthRequest",
    "HealthResponse",
    "GetSchemaRequest",
    "GetSchemaResponse",
    # gRPC
    "EntDBServiceServicer",
    "EntDBServiceStub",
    "add_EntDBServiceServicer_to_server",
]
EOF

cat > "$SDK_OUT/__init__.py" << 'EOF'
"""Generated protobuf code for EntDB SDK.

Do not edit manually - regenerate with scripts/generate_proto.sh

This module is internal to the SDK. Users should not import from here.
"""

from .entdb_pb2 import (
    # Common
    RequestContext,
    Receipt,
    # Execute
    ExecuteAtomicRequest,
    ExecuteAtomicResponse,
    Operation,
    CreateNodeOp,
    UpdateNodeOp,
    DeleteNodeOp,
    CreateEdgeOp,
    DeleteEdgeOp,
    NodeRef,
    TypedNodeRef,
    # Receipt
    GetReceiptStatusRequest,
    GetReceiptStatusResponse,
    ReceiptStatus,
    # Nodes
    GetNodeRequest,
    GetNodeResponse,
    GetNodesRequest,
    GetNodesResponse,
    QueryNodesRequest,
    QueryNodesResponse,
    Node,
    # Edges
    GetEdgesRequest,
    GetEdgesResponse,
    Edge,
    # Mailbox
    SearchMailboxRequest,
    SearchMailboxResponse,
    MailboxSearchResult,
    GetMailboxRequest,
    GetMailboxResponse,
    MailboxItem,
    # Health/Schema
    HealthRequest,
    HealthResponse,
    GetSchemaRequest,
    GetSchemaResponse,
)
from .entdb_pb2_grpc import EntDBServiceStub

__all__ = [
    "RequestContext",
    "Receipt",
    "ExecuteAtomicRequest",
    "ExecuteAtomicResponse",
    "Operation",
    "CreateNodeOp",
    "UpdateNodeOp",
    "DeleteNodeOp",
    "CreateEdgeOp",
    "DeleteEdgeOp",
    "NodeRef",
    "TypedNodeRef",
    "GetReceiptStatusRequest",
    "GetReceiptStatusResponse",
    "ReceiptStatus",
    "GetNodeRequest",
    "GetNodeResponse",
    "GetNodesRequest",
    "GetNodesResponse",
    "QueryNodesRequest",
    "QueryNodesResponse",
    "Node",
    "GetEdgesRequest",
    "GetEdgesResponse",
    "Edge",
    "SearchMailboxRequest",
    "SearchMailboxResponse",
    "MailboxSearchResult",
    "GetMailboxRequest",
    "GetMailboxResponse",
    "MailboxItem",
    "HealthRequest",
    "HealthResponse",
    "GetSchemaRequest",
    "GetSchemaResponse",
    "EntDBServiceStub",
]
EOF

echo "Done! Generated files:"
ls -la "$SERVER_OUT"
echo ""
ls -la "$SDK_OUT"
