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
