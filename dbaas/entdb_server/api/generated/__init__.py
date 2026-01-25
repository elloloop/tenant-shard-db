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
