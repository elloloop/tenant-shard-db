# mypy: ignore-errors
"""Generated protobuf code for EntDB server.

Do not edit manually - regenerate with scripts/generate_proto.sh
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
    # Tenants
    "ListTenantsRequest",
    "ListTenantsResponse",
    "TenantInfo",
    "ListMailboxUsersRequest",
    "ListMailboxUsersResponse",
    # gRPC
    "EntDBServiceServicer",
    "EntDBServiceStub",
    "add_EntDBServiceServicer_to_server",
]
