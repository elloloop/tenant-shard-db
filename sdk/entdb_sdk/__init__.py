"""
EntDB Python SDK — single-shape v0.3 API.

The SDK exposes exactly one way to perform every operation:

- ``Plan.create(msg, *, acl=None, storage=Tenant())`` — pass a proto
  message instance. ``type_id`` and the payload come from the proto
  descriptor; the user never types a numeric id.
- ``Plan.update(node_id, msg)`` — partial update; only fields set on
  ``msg`` (per ``ListFields()``) become the patch.
- ``Plan.delete(NodeType, node_id)`` — proto message *class* as the
  type witness.
- ``Scope.get_by_key(UniqueKey, value)`` — typed unique-key lookup
  using a token from the ``protoc-gen-entdb-keys`` codegen sidecar.

Example::

    from entdb_sdk import DbClient, register_proto_schema, Actor
    from schema_entdb import ProductKeys  # codegen sidecar
    import schema_pb2

    async with DbClient("localhost:50051") as db:
        register_proto_schema(schema_pb2)
        scope = db.tenant("acme").actor(Actor.user("alice"))

        plan = scope.plan()
        plan.create(schema_pb2.Product(sku="WIDGET-1", name="Widget"))
        await plan.commit(wait_applied=True)

        product = await scope.get_by_key(ProductKeys.sku, "WIDGET-1")

See ``docs/decisions/sdk_api.md`` for the design rationale.
"""

from ._redirect_cache import (
    DNSTemplateResolver,
    NodeResolver,
    StaticMapResolver,
)
from ._version import __version__
from .client import DbClient, Plan, Receipt
from .codegen import register_proto_schema
from .errors import (
    ConnectionError,
    EntDbError,
    RateLimitError,
    SchemaError,
    UniqueConstraintError,
    UnknownFieldError,
    ValidationError,
)
from .keys import Mailbox, Public, Storage, Tenant, UniqueKey
from .registry import (
    SchemaRegistry,
    get_registry,
    register_edge_type,
    register_node_type,
)
from .schema import (
    AclDefaults,
    DataPolicy,
    EdgeTypeDef,
    FieldDef,
    FieldKind,
    NodeTypeDef,
    SubjectExitPolicy,
    field,
)
from .scope import ActorScope, ScopedPlan, TenantScope
from .typed import ACLEntry, Actor, AliasRef, NodeRef, Permission, TypedEdge, TypedNode

__all__ = [
    # Version
    "__version__",
    # Schema types
    "AclDefaults",
    "DataPolicy",
    "SubjectExitPolicy",
    "NodeTypeDef",
    "EdgeTypeDef",
    "FieldDef",
    "FieldKind",
    "field",
    "TypedNode",
    "TypedEdge",
    "Permission",
    "ACLEntry",
    "NodeRef",
    "AliasRef",
    "Actor",
    # Proto-first registration
    "register_proto_schema",
    # Hierarchical scopes
    "TenantScope",
    "ActorScope",
    "ScopedPlan",
    # Registry
    "SchemaRegistry",
    "get_registry",
    "register_node_type",
    "register_edge_type",
    # Client
    "DbClient",
    "Plan",
    "Receipt",
    # Errors
    "EntDbError",
    "ConnectionError",
    "ValidationError",
    "SchemaError",
    "UnknownFieldError",
    "RateLimitError",
    "UniqueConstraintError",
    # SDK v0.3 typed unique keys + storage descriptors
    "UniqueKey",
    "Tenant",
    "Mailbox",
    "Public",
    "Storage",
    # Tenant redirect cache (Option D, PR-C)
    "NodeResolver",
    "DNSTemplateResolver",
    "StaticMapResolver",
]
