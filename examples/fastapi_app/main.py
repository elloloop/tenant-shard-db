"""
FastAPI Sample Application for EntDB.

This sample application demonstrates how to use the EntDB SDK
to build a task management API.

Features:
- Task CRUD with automatic assignment
- Team management
- Message creation with mailbox fanout
- Full-text search

Usage:
    uvicorn examples.fastapi_app.main:app --reload

Environment:
    ENTDB_ADDRESS: EntDB server address (default: localhost:8081)
    TENANT_ID: Default tenant ID (default: demo_tenant)
"""

from __future__ import annotations

import os

# Import SDK
import sys
from contextlib import asynccontextmanager

from fastapi import FastAPI, Header, HTTPException, Query
from pydantic import BaseModel

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "../.."))

from sdk.entdb_sdk import (
    DbClient,
    EdgeTypeDef,
    NodeTypeDef,
    field,
    get_registry,
)

# =============================================================================
# Schema Definitions
# =============================================================================

User = NodeTypeDef(
    type_id=1,
    name="User",
    fields=(
        field(1, "email", "str", required=True, indexed=True),
        field(2, "display_name", "str", required=True, searchable=True),
        field(3, "avatar_url", "str"),
    ),
)

Team = NodeTypeDef(
    type_id=2,
    name="Team",
    fields=(
        field(1, "name", "str", required=True, searchable=True),
        field(2, "description", "str", searchable=True),
    ),
)

Task = NodeTypeDef(
    type_id=101,
    name="Task",
    fields=(
        field(1, "title", "str", required=True, searchable=True),
        field(2, "description", "str", searchable=True),
        field(3, "status", "enum", enum_values=("todo", "in_progress", "done"), required=True),
        field(4, "priority", "enum", enum_values=("low", "medium", "high")),
        field(5, "due_date", "timestamp"),
    ),
)

Message = NodeTypeDef(
    type_id=201,
    name="Message",
    fields=(
        field(1, "subject", "str", required=True, searchable=True),
        field(2, "body", "str", required=True, searchable=True),
        field(3, "thread_id", "str", indexed=True),
    ),
)

# Edge types
AssignedTo = EdgeTypeDef(
    edge_id=401,
    name="AssignedTo",
    from_type=Task,
    to_type=User,
    props=(field(1, "role", "enum", enum_values=("primary", "reviewer", "observer")),),
)

MemberOf = EdgeTypeDef(
    edge_id=402,
    name="MemberOf",
    from_type=User,
    to_type=Team,
    props=(field(1, "role", "enum", enum_values=("owner", "admin", "member")),),
)

SentTo = EdgeTypeDef(
    edge_id=403,
    name="SentTo",
    from_type=Message,
    to_type=User,
)

# Register types
registry = get_registry()
registry.register_node_type(User)
registry.register_node_type(Team)
registry.register_node_type(Task)
registry.register_node_type(Message)
registry.register_edge_type(AssignedTo)
registry.register_edge_type(MemberOf)
registry.register_edge_type(SentTo)

# =============================================================================
# Pydantic Models
# =============================================================================


class CreateTaskRequest(BaseModel):
    title: str
    description: str | None = None
    status: str = "todo"
    priority: str | None = "medium"
    assigned_to: str | None = None
    role: str | None = "primary"


class UpdateTaskRequest(BaseModel):
    title: str | None = None
    description: str | None = None
    status: str | None = None
    priority: str | None = None


class CreateTeamRequest(BaseModel):
    name: str
    description: str | None = None
    members: list[str] | None = None


class AddMemberRequest(BaseModel):
    user_id: str
    role: str = "member"


class CreateMessageRequest(BaseModel):
    subject: str
    body: str
    to_user_ids: list[str]
    thread_id: str | None = None


class TaskResponse(BaseModel):
    id: str
    title: str
    description: str | None
    status: str
    priority: str | None
    created_at: int
    updated_at: int


class TeamResponse(BaseModel):
    id: str
    name: str
    description: str | None
    created_at: int


class SearchResultResponse(BaseModel):
    id: str
    type: str
    snippet: str
    rank: float


# =============================================================================
# Application
# =============================================================================

# Configuration
ENTDB_ADDRESS = os.getenv("ENTDB_ADDRESS", "http://localhost:8081")
DEFAULT_TENANT = os.getenv("TENANT_ID", "demo_tenant")

# Global client
db: DbClient | None = None


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Application lifespan handler."""
    global db
    db = DbClient(ENTDB_ADDRESS, use_http=True, registry=registry)
    await db.connect()
    yield
    await db.close()


app = FastAPI(
    title="EntDB Sample API",
    description="Task management API built on EntDB",
    version="1.0.0",
    lifespan=lifespan,
)


def get_context(
    tenant_id: str = Header(default=DEFAULT_TENANT, alias="X-Tenant-ID"),
    actor: str = Header(..., alias="X-Actor"),
) -> tuple[str, str]:
    """Extract request context from headers."""
    return tenant_id, actor


# =============================================================================
# Task Endpoints
# =============================================================================


@app.post("/tasks", response_model=TaskResponse, tags=["Tasks"])
async def create_task(
    request: CreateTaskRequest,
    tenant_id: str = Header(default=DEFAULT_TENANT, alias="X-Tenant-ID"),
    actor: str = Header(..., alias="X-Actor"),
):
    """Create a new task, optionally assigning it to a user."""
    plan = db.atomic(tenant_id, actor)

    # Create task
    task_data = {
        "title": request.title,
        "status": request.status,
    }
    if request.description:
        task_data["description"] = request.description
    if request.priority:
        task_data["priority"] = request.priority

    # If assigning, fanout to that user's mailbox
    fanout = []
    if request.assigned_to:
        fanout = [request.assigned_to]

    plan.create(Task, task_data, as_="task", fanout_to=fanout)

    # Create assignment edge if specified
    if request.assigned_to:
        plan.edge_create(
            AssignedTo,
            from_="$task.id",
            to={"type_id": User.type_id, "id": request.assigned_to},
            props={"role": request.role or "primary"},
        )

    result = await plan.commit(wait_applied=True)

    if not result.success:
        raise HTTPException(status_code=500, detail=result.error)

    task_id = result.created_node_ids[0] if result.created_node_ids else "unknown"

    return TaskResponse(
        id=task_id,
        title=request.title,
        description=request.description,
        status=request.status,
        priority=request.priority,
        created_at=0,
        updated_at=0,
    )


@app.get("/tasks/{task_id}", response_model=TaskResponse, tags=["Tasks"])
async def get_task(
    task_id: str,
    tenant_id: str = Header(default=DEFAULT_TENANT, alias="X-Tenant-ID"),
    actor: str = Header(..., alias="X-Actor"),
):
    """Get a task by ID."""
    node = await db.get(Task, task_id, tenant_id, actor)

    if not node:
        raise HTTPException(status_code=404, detail="Task not found")

    return TaskResponse(
        id=node.node_id,
        title=node.payload.get("title", ""),
        description=node.payload.get("description"),
        status=node.payload.get("status", "todo"),
        priority=node.payload.get("priority"),
        created_at=node.created_at,
        updated_at=node.updated_at,
    )


@app.put("/tasks/{task_id}", response_model=TaskResponse, tags=["Tasks"])
async def update_task(
    task_id: str,
    request: UpdateTaskRequest,
    tenant_id: str = Header(default=DEFAULT_TENANT, alias="X-Tenant-ID"),
    actor: str = Header(..., alias="X-Actor"),
):
    """Update a task."""
    patch = {}
    if request.title is not None:
        patch["title"] = request.title
    if request.description is not None:
        patch["description"] = request.description
    if request.status is not None:
        patch["status"] = request.status
    if request.priority is not None:
        patch["priority"] = request.priority

    if not patch:
        raise HTTPException(status_code=400, detail="No fields to update")

    plan = db.atomic(tenant_id, actor)
    plan.update(Task, task_id, patch)
    result = await plan.commit(wait_applied=True)

    if not result.success:
        raise HTTPException(status_code=500, detail=result.error)

    # Fetch updated task
    node = await db.get(Task, task_id, tenant_id, actor)
    if not node:
        raise HTTPException(status_code=404, detail="Task not found")

    return TaskResponse(
        id=node.node_id,
        title=node.payload.get("title", ""),
        description=node.payload.get("description"),
        status=node.payload.get("status", "todo"),
        priority=node.payload.get("priority"),
        created_at=node.created_at,
        updated_at=node.updated_at,
    )


@app.get("/tasks", response_model=list[TaskResponse], tags=["Tasks"])
async def list_tasks(
    limit: int = Query(default=20, le=100),
    offset: int = Query(default=0, ge=0),
    tenant_id: str = Header(default=DEFAULT_TENANT, alias="X-Tenant-ID"),
    actor: str = Header(..., alias="X-Actor"),
):
    """List all tasks."""
    nodes = await db.query(Task, tenant_id, actor, limit=limit, offset=offset)

    return [
        TaskResponse(
            id=n.node_id,
            title=n.payload.get("title", ""),
            description=n.payload.get("description"),
            status=n.payload.get("status", "todo"),
            priority=n.payload.get("priority"),
            created_at=n.created_at,
            updated_at=n.updated_at,
        )
        for n in nodes
    ]


# =============================================================================
# Team Endpoints
# =============================================================================


@app.post("/teams", response_model=TeamResponse, tags=["Teams"])
async def create_team(
    request: CreateTeamRequest,
    tenant_id: str = Header(default=DEFAULT_TENANT, alias="X-Tenant-ID"),
    actor: str = Header(..., alias="X-Actor"),
):
    """Create a new team with optional initial members."""
    plan = db.atomic(tenant_id, actor)

    team_data = {"name": request.name}
    if request.description:
        team_data["description"] = request.description

    plan.create(Team, team_data, as_="team")

    # Add members
    if request.members:
        for user_id in request.members:
            plan.edge_create(
                MemberOf,
                from_={"type_id": User.type_id, "id": user_id},
                to="$team.id",
                props={"role": "member"},
            )

    result = await plan.commit(wait_applied=True)

    if not result.success:
        raise HTTPException(status_code=500, detail=result.error)

    team_id = result.created_node_ids[0] if result.created_node_ids else "unknown"

    return TeamResponse(
        id=team_id,
        name=request.name,
        description=request.description,
        created_at=0,
    )


@app.post("/teams/{team_id}/members", tags=["Teams"])
async def add_team_member(
    team_id: str,
    request: AddMemberRequest,
    tenant_id: str = Header(default=DEFAULT_TENANT, alias="X-Tenant-ID"),
    actor: str = Header(..., alias="X-Actor"),
):
    """Add a member to a team."""
    plan = db.atomic(tenant_id, actor)
    plan.edge_create(
        MemberOf,
        from_={"type_id": User.type_id, "id": request.user_id},
        to={"type_id": Team.type_id, "id": team_id},
        props={"role": request.role},
    )
    result = await plan.commit(wait_applied=True)

    if not result.success:
        raise HTTPException(status_code=500, detail=result.error)

    return {"status": "added"}


# =============================================================================
# Message Endpoints
# =============================================================================


@app.post("/messages", tags=["Messages"])
async def create_message(
    request: CreateMessageRequest,
    tenant_id: str = Header(default=DEFAULT_TENANT, alias="X-Tenant-ID"),
    actor: str = Header(..., alias="X-Actor"),
):
    """Create a message and fanout to recipients' mailboxes."""
    plan = db.atomic(tenant_id, actor)

    message_data = {
        "subject": request.subject,
        "body": request.body,
    }
    if request.thread_id:
        message_data["thread_id"] = request.thread_id

    # Fanout to all recipients
    fanout_to = [f"user:{uid}" for uid in request.to_user_ids]

    plan.create(Message, message_data, as_="msg", fanout_to=fanout_to)

    # Create edges to recipients
    for user_id in request.to_user_ids:
        plan.edge_create(
            SentTo,
            from_="$msg.id",
            to={"type_id": User.type_id, "id": user_id},
        )

    result = await plan.commit(wait_applied=True)

    if not result.success:
        raise HTTPException(status_code=500, detail=result.error)

    msg_id = result.created_node_ids[0] if result.created_node_ids else "unknown"

    return {
        "id": msg_id,
        "subject": request.subject,
        "recipients": request.to_user_ids,
    }


# =============================================================================
# Search Endpoints
# =============================================================================


@app.get("/search", response_model=list[SearchResultResponse], tags=["Search"])
async def search(
    q: str = Query(..., min_length=1),
    user_id: str = Query(...),
    limit: int = Query(default=20, le=100),
    tenant_id: str = Header(default=DEFAULT_TENANT, alias="X-Tenant-ID"),
    actor: str = Header(..., alias="X-Actor"),
):
    """Search user's mailbox."""
    results = await db.search(
        query=q,
        user_id=user_id,
        tenant_id=tenant_id,
        actor=actor,
        limit=limit,
    )

    return [
        SearchResultResponse(
            id=r.get("item", {}).get("source_node_id", ""),
            type=str(r.get("item", {}).get("source_type_id", 0)),
            snippet=r.get("highlights") or r.get("item", {}).get("snippet", ""),
            rank=r.get("rank", 0.0),
        )
        for r in results
    ]


# =============================================================================
# Health Endpoint
# =============================================================================


@app.get("/health", tags=["Health"])
async def health():
    """Health check endpoint."""
    return {"status": "healthy", "service": "fastapi-sample"}


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, host="0.0.0.0", port=8000)
