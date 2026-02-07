"""
Demo schema for EntDB Playground.

This defines a simple project management schema that developers
can use to experiment with EntDB. The schema includes:
- User: People in the system
- Project: Containers for tasks
- Task: Work items with status
- Comment: Notes on tasks

Edges connect these entities:
- User OWNS Project
- Project CONTAINS Task
- Task ASSIGNED_TO User
- Comment ON Task
- Comment BY User
"""

from entdb_sdk import EdgeTypeDef, NodeTypeDef, field

# --- Node Types ---

User = NodeTypeDef(
    type_id=1,
    name="User",
    description="A person in the system",
    fields=(
        field(1, "email", "str", required=True, indexed=True),
        field(2, "name", "str", required=True, searchable=True),
        field(3, "role", "enum", enum_values=("admin", "member", "guest"), default="member"),
        field(4, "avatar_url", "str"),
        field(5, "created_at", "timestamp"),
    ),
)

Project = NodeTypeDef(
    type_id=2,
    name="Project",
    description="A container for tasks",
    fields=(
        field(1, "name", "str", required=True, searchable=True),
        field(2, "description", "str", searchable=True),
        field(3, "color", "str", default="#3B82F6"),
        field(4, "status", "enum", enum_values=("active", "archived"), default="active"),
        field(5, "created_at", "timestamp"),
    ),
)

Task = NodeTypeDef(
    type_id=3,
    name="Task",
    description="A work item",
    fields=(
        field(1, "title", "str", required=True, searchable=True),
        field(2, "description", "str", searchable=True),
        field(
            3,
            "status",
            "enum",
            enum_values=("todo", "in_progress", "review", "done"),
            default="todo",
        ),
        field(
            4, "priority", "enum", enum_values=("low", "medium", "high", "urgent"), default="medium"
        ),
        field(5, "due_date", "timestamp"),
        field(6, "created_at", "timestamp"),
        field(7, "completed_at", "timestamp"),
    ),
)

Comment = NodeTypeDef(
    type_id=4,
    name="Comment",
    description="A comment on a task",
    fields=(
        field(1, "body", "str", required=True, searchable=True),
        field(2, "created_at", "timestamp"),
        field(3, "edited_at", "timestamp"),
    ),
)

# --- Edge Types ---

Owns = EdgeTypeDef(
    edge_id=101,
    name="OWNS",
    description="User owns a project",
    from_type=User,
    to_type=Project,
)

Contains = EdgeTypeDef(
    edge_id=102,
    name="CONTAINS",
    description="Project contains tasks",
    from_type=Project,
    to_type=Task,
)

AssignedTo = EdgeTypeDef(
    edge_id=103,
    name="ASSIGNED_TO",
    description="Task is assigned to a user",
    from_type=Task,
    to_type=User,
    props=(field(1, "role", "enum", enum_values=("assignee", "reviewer")),),
)

CommentOn = EdgeTypeDef(
    edge_id=104,
    name="ON",
    description="Comment is on a task",
    from_type=Comment,
    to_type=Task,
)

CommentBy = EdgeTypeDef(
    edge_id=105,
    name="BY",
    description="Comment is by a user",
    from_type=Comment,
    to_type=User,
)

# --- Registry ---

ALL_NODE_TYPES = [User, Project, Task, Comment]
ALL_EDGE_TYPES = [Owns, Contains, AssignedTo, CommentOn, CommentBy]

# Map type_id to NodeTypeDef for lookup
NODE_TYPE_MAP = {t.type_id: t for t in ALL_NODE_TYPES}
EDGE_TYPE_MAP = {e.edge_id: e for e in ALL_EDGE_TYPES}


def get_node_type(type_id: int) -> NodeTypeDef | None:
    """Get node type by ID."""
    return NODE_TYPE_MAP.get(type_id)


def get_edge_type(edge_id: int) -> EdgeTypeDef | None:
    """Get edge type by ID."""
    return EDGE_TYPE_MAP.get(edge_id)
