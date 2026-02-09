# mypy: ignore-errors
"""Generated protobuf code for EntDB SDK.

Do not edit manually - regenerate with scripts/generate_proto.sh

This module is internal to the SDK. Users should not import from here.
"""

from .entdb_pb2 import (
    CreateEdgeOp,
    CreateNodeOp,
    DeleteEdgeOp,
    DeleteNodeOp,
    Edge,
    # Execute
    ExecuteAtomicRequest,
    ExecuteAtomicResponse,
    # Edges
    GetEdgesRequest,
    GetEdgesResponse,
    GetMailboxRequest,
    GetMailboxResponse,
    # Nodes
    GetNodeRequest,
    GetNodeResponse,
    GetNodesRequest,
    GetNodesResponse,
    # Receipt
    GetReceiptStatusRequest,
    GetReceiptStatusResponse,
    GetSchemaRequest,
    GetSchemaResponse,
    # Health/Schema
    HealthRequest,
    HealthResponse,
    # Tenants
    ListMailboxUsersRequest,
    ListMailboxUsersResponse,
    ListTenantsRequest,
    ListTenantsResponse,
    MailboxItem,
    MailboxSearchResult,
    Node,
    NodeRef,
    Operation,
    QueryNodesRequest,
    QueryNodesResponse,
    Receipt,
    ReceiptStatus,
    # Common
    RequestContext,
    # Mailbox
    SearchMailboxRequest,
    SearchMailboxResponse,
    TenantInfo,
    TypedNodeRef,
    UpdateNodeOp,
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
    "ListTenantsRequest",
    "ListTenantsResponse",
    "TenantInfo",
    "ListMailboxUsersRequest",
    "ListMailboxUsersResponse",
    "EntDBServiceStub",
]
