"""
EntDB Python SDK - Client library for EntDB database service.

This SDK provides a type-safe interface to the EntDB graph database:
- Type definitions (NodeTypeDef, EdgeTypeDef, FieldDef)
- Schema registry for type management
- DbClient for connecting to the server
- Plan builder for atomic transactions

Example:
    >>> from entdb_sdk import DbClient, NodeTypeDef, field
    >>>
    >>> # Define types
    >>> Task = NodeTypeDef(
    ...     type_id=101,
    ...     name="Task",
    ...     fields=(
    ...         field(1, "title", "str", required=True),
    ...         field(2, "status", "enum", enum_values=("todo", "done")),
    ...     ),
    ... )
    >>>
    >>> # Connect and create
    >>> async with DbClient("localhost:50051") as db:
    ...     plan = db.atomic("tenant_1", "user:42")
    ...     plan.create(Task, {"title": "My Task", "status": "todo"})
    ...     result = await plan.commit()

Invariants:
    - Type IDs are immutable after first use
    - All operations require tenant_id and actor
    - Writes are atomic per commit()

Version: 1.0.0
"""

__version__ = "1.0.0"

from .client import DbClient, Plan, Receipt
from .errors import (
    ConnectionError,
    EntDbError,
    SchemaError,
    UnknownFieldError,
    ValidationError,
)
from .registry import (
    SchemaRegistry,
    get_registry,
    register_edge_type,
    register_node_type,
)
from .schema import (
    EdgeTypeDef,
    FieldDef,
    FieldKind,
    NodeTypeDef,
    field,
)

__all__ = [
    # Version
    "__version__",
    # Schema types
    "NodeTypeDef",
    "EdgeTypeDef",
    "FieldDef",
    "FieldKind",
    "field",
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
]
