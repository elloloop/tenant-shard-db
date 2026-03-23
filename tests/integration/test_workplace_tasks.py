"""
Workplace communication tests: Tasks, Assignments, Projects.

Simulates workspace tools with:
  - Tasks (type_id=5): title, description, status, due_date, assignee
  - Projects (type_id=8): name, description, owner
  - Users (type_id=1): name, email, role
  - Edges: project->task (edge_type=30), user->task assignment (edge_type=31),
           task->task dependency (edge_type=32), user->project membership (edge_type=33)
"""

from __future__ import annotations

import asyncio
import time

import pytest

from dbaas.entdb_server.apply.canonical_store import CanonicalStore

TENANT = "workspace-tasks"
USER_TYPE = 1
TASK_TYPE = 5
PROJECT_TYPE = 8
EDGE_PROJ_TASK = 30
EDGE_USER_TASK = 31
EDGE_TASK_DEP = 32
EDGE_USER_PROJ = 33


@pytest.fixture
def store(tmp_path):
    s = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
    asyncio.get_event_loop().run_until_complete(s.initialize_tenant(TENANT))
    return s


@pytest.fixture
def multi_store(tmp_path):
    s = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
    loop = asyncio.get_event_loop()
    for t in [TENANT, "workspace-other"]:
        loop.run_until_complete(s.initialize_tenant(t))
    return s


# =========================================================================
# TASK LIFECYCLE
# =========================================================================


@pytest.mark.integration
class TestTaskLifecycle:
    @pytest.mark.asyncio
    async def test_create_task(self, store):
        task = await store.create_node(
            TENANT,
            TASK_TYPE,
            {"title": "Build login page", "status": "todo", "description": "OAuth flow"},
            "user:1",
        )
        assert task.node_id is not None
        assert task.payload["title"] == "Build login page"
        assert task.payload["status"] == "todo"

    @pytest.mark.asyncio
    async def test_read_task(self, store):
        task = await store.create_node(
            TENANT, TASK_TYPE, {"title": "Read me", "status": "todo"}, "user:1"
        )
        fetched = await store.get_node(TENANT, task.node_id)
        assert fetched is not None
        assert fetched.payload["title"] == "Read me"

    @pytest.mark.asyncio
    async def test_update_status_transitions(self, store):
        task = await store.create_node(
            TENANT, TASK_TYPE, {"title": "Flow", "status": "todo"}, "user:1"
        )
        tid = task.node_id
        # todo -> in_progress
        u1 = await store.update_node(TENANT, tid, {"status": "in_progress"})
        assert u1.payload["status"] == "in_progress"
        # in_progress -> done
        u2 = await store.update_node(TENANT, tid, {"status": "done"})
        assert u2.payload["status"] == "done"

    @pytest.mark.asyncio
    async def test_update_assignee(self, store):
        task = await store.create_node(
            TENANT,
            TASK_TYPE,
            {"title": "Assign me", "assignee": "user:alice"},
            "user:1",
        )
        updated = await store.update_node(TENANT, task.node_id, {"assignee": "user:bob"})
        assert updated.payload["assignee"] == "user:bob"

    @pytest.mark.asyncio
    async def test_delete_task(self, store):
        task = await store.create_node(TENANT, TASK_TYPE, {"title": "Trash"}, "user:1")
        assert await store.delete_node(TENANT, task.node_id) is True
        assert await store.get_node(TENANT, task.node_id) is None

    @pytest.mark.asyncio
    async def test_task_with_all_fields(self, store):
        now_ms = int(time.time() * 1000)
        task = await store.create_node(
            TENANT,
            TASK_TYPE,
            {
                "title": "Complete task",
                "description": "With every field",
                "status": "in_progress",
                "due_date": now_ms + 86400000,
                "assignee": "user:charlie",
                "priority": "high",
                "tags": ["backend", "urgent"],
                "estimated_hours": 8,
            },
            "user:pm",
        )
        fetched = await store.get_node(TENANT, task.node_id)
        assert fetched.payload["priority"] == "high"
        assert fetched.payload["tags"] == ["backend", "urgent"]
        assert fetched.payload["estimated_hours"] == 8

    @pytest.mark.asyncio
    async def test_task_with_no_assignee(self, store):
        task = await store.create_node(
            TENANT, TASK_TYPE, {"title": "Unassigned", "status": "todo"}, "user:1"
        )
        fetched = await store.get_node(TENANT, task.node_id)
        assert "assignee" not in fetched.payload

    @pytest.mark.asyncio
    async def test_task_with_past_due_date(self, store):
        past = int(time.time() * 1000) - 86400000
        task = await store.create_node(
            TENANT,
            TASK_TYPE,
            {"title": "Overdue", "status": "todo", "due_date": past},
            "user:1",
        )
        fetched = await store.get_node(TENANT, task.node_id)
        assert fetched.payload["due_date"] < int(time.time() * 1000)

    @pytest.mark.asyncio
    async def test_task_with_future_due_date(self, store):
        future = int(time.time() * 1000) + 86400000 * 30
        task = await store.create_node(
            TENANT,
            TASK_TYPE,
            {"title": "Future", "status": "todo", "due_date": future},
            "user:1",
        )
        fetched = await store.get_node(TENANT, task.node_id)
        assert fetched.payload["due_date"] > int(time.time() * 1000)

    @pytest.mark.asyncio
    async def test_overdue_detection(self, store):
        now_ms = int(time.time() * 1000)
        past = now_ms - 86400000
        future = now_ms + 86400000
        await store.create_node(
            TENANT, TASK_TYPE, {"title": "Late", "due_date": past, "status": "todo"}, "user:1"
        )
        await store.create_node(
            TENANT, TASK_TYPE, {"title": "OK", "due_date": future, "status": "todo"}, "user:1"
        )
        tasks = await store.get_nodes_by_type(TENANT, TASK_TYPE)
        overdue = [t for t in tasks if t.payload.get("due_date", float("inf")) < now_ms]
        assert len(overdue) == 1
        assert overdue[0].payload["title"] == "Late"

    @pytest.mark.asyncio
    async def test_task_priority_field(self, store):
        for priority in ["low", "medium", "high", "critical"]:
            await store.create_node(
                TENANT,
                TASK_TYPE,
                {"title": f"{priority} task", "priority": priority},
                "user:1",
            )
        tasks = await store.get_nodes_by_type(TENANT, TASK_TYPE)
        priorities = {t.payload["priority"] for t in tasks}
        assert priorities == {"low", "medium", "high", "critical"}

    @pytest.mark.asyncio
    async def test_multiple_status_transitions(self, store):
        task = await store.create_node(
            TENANT, TASK_TYPE, {"title": "Many states", "status": "todo"}, "user:1"
        )
        tid = task.node_id
        statuses = ["in_progress", "blocked", "in_progress", "review", "done"]
        for s in statuses:
            await store.update_node(TENANT, tid, {"status": s})
        fetched = await store.get_node(TENANT, tid)
        assert fetched.payload["status"] == "done"

    @pytest.mark.asyncio
    async def test_task_description_update(self, store):
        task = await store.create_node(
            TENANT,
            TASK_TYPE,
            {"title": "Desc", "description": "Original desc"},
            "user:1",
        )
        updated = await store.update_node(
            TENANT, task.node_id, {"description": "Updated description with more detail"}
        )
        assert updated.payload["description"] == "Updated description with more detail"
        assert updated.payload["title"] == "Desc"

    @pytest.mark.asyncio
    async def test_task_with_tags_list(self, store):
        task = await store.create_node(
            TENANT,
            TASK_TYPE,
            {"title": "Tagged", "tags": ["bug", "p0", "frontend"]},
            "user:1",
        )
        fetched = await store.get_node(TENANT, task.node_id)
        assert "bug" in fetched.payload["tags"]
        assert len(fetched.payload["tags"]) == 3

    @pytest.mark.asyncio
    async def test_full_crud_cycle(self, store):
        task = await store.create_node(
            TENANT,
            TASK_TYPE,
            {"title": "Lifecycle", "status": "todo", "description": "v1"},
            "user:1",
        )
        tid = task.node_id
        fetched = await store.get_node(TENANT, tid)
        assert fetched.payload["status"] == "todo"
        await store.update_node(TENANT, tid, {"status": "done", "description": "v2"})
        fetched2 = await store.get_node(TENANT, tid)
        assert fetched2.payload["status"] == "done"
        assert fetched2.payload["description"] == "v2"
        assert await store.delete_node(TENANT, tid) is True
        assert await store.get_node(TENANT, tid) is None


# =========================================================================
# PROJECT MANAGEMENT
# =========================================================================


@pytest.mark.integration
class TestProjectManagement:
    @pytest.mark.asyncio
    async def test_create_project(self, store):
        proj = await store.create_node(
            TENANT,
            PROJECT_TYPE,
            {"name": "Website Redesign", "description": "Q1 project", "owner": "user:pm"},
            "user:pm",
            node_id="proj-web",
        )
        assert proj.payload["name"] == "Website Redesign"

    @pytest.mark.asyncio
    async def test_add_task_to_project(self, store):
        await store.create_node(
            TENANT, PROJECT_TYPE, {"name": "Proj A"}, "user:pm", node_id="proj-a"
        )
        task = await store.create_node(
            TENANT, TASK_TYPE, {"title": "Task 1", "status": "todo"}, "user:1"
        )
        edge = await store.create_edge(TENANT, EDGE_PROJ_TASK, "proj-a", task.node_id)
        assert edge.edge_type_id == EDGE_PROJ_TASK

    @pytest.mark.asyncio
    async def test_list_tasks_in_project(self, store):
        await store.create_node(
            TENANT, PROJECT_TYPE, {"name": "Proj B"}, "user:pm", node_id="proj-b"
        )
        for i in range(8):
            t = await store.create_node(
                TENANT, TASK_TYPE, {"title": f"T{i}", "status": "todo"}, "user:1"
            )
            await store.create_edge(TENANT, EDGE_PROJ_TASK, "proj-b", t.node_id)
        edges = await store.get_edges_from(TENANT, "proj-b", edge_type_id=EDGE_PROJ_TASK)
        assert len(edges) == 8

    @pytest.mark.asyncio
    async def test_remove_task_from_project(self, store):
        await store.create_node(
            TENANT, PROJECT_TYPE, {"name": "Proj C"}, "user:pm", node_id="proj-c"
        )
        await store.create_node(
            TENANT, TASK_TYPE, {"title": "Remove me"}, "user:1", node_id="task-rm"
        )
        await store.create_edge(TENANT, EDGE_PROJ_TASK, "proj-c", "task-rm")
        result = await store.delete_edge(TENANT, EDGE_PROJ_TASK, "proj-c", "task-rm")
        assert result is True
        edges = await store.get_edges_from(TENANT, "proj-c", edge_type_id=EDGE_PROJ_TASK)
        assert len(edges) == 0

    @pytest.mark.asyncio
    async def test_add_member_to_project(self, store):
        await store.create_node(
            TENANT, PROJECT_TYPE, {"name": "Team proj"}, "user:pm", node_id="proj-team"
        )
        edge = await store.create_edge(TENANT, EDGE_USER_PROJ, "user:alice", "proj-team")
        assert edge.from_node_id == "user:alice"

    @pytest.mark.asyncio
    async def test_list_project_members(self, store):
        await store.create_node(
            TENANT, PROJECT_TYPE, {"name": "Members"}, "user:pm", node_id="proj-mem"
        )
        for i in range(4):
            await store.create_edge(TENANT, EDGE_USER_PROJ, f"user:{i}", "proj-mem")
        edges = await store.get_edges_to(TENANT, "proj-mem", edge_type_id=EDGE_USER_PROJ)
        assert len(edges) == 4

    @pytest.mark.asyncio
    async def test_project_with_no_tasks(self, store):
        await store.create_node(
            TENANT, PROJECT_TYPE, {"name": "Empty"}, "user:pm", node_id="proj-empty"
        )
        edges = await store.get_edges_from(TENANT, "proj-empty", edge_type_id=EDGE_PROJ_TASK)
        assert len(edges) == 0

    @pytest.mark.asyncio
    async def test_project_with_50_tasks(self, store):
        await store.create_node(
            TENANT, PROJECT_TYPE, {"name": "Big proj"}, "user:pm", node_id="proj-big"
        )
        for i in range(50):
            t = await store.create_node(
                TENANT, TASK_TYPE, {"title": f"Task {i}", "status": "todo"}, "user:1"
            )
            await store.create_edge(TENANT, EDGE_PROJ_TASK, "proj-big", t.node_id)
        edges = await store.get_edges_from(TENANT, "proj-big", edge_type_id=EDGE_PROJ_TASK)
        assert len(edges) == 50

    @pytest.mark.asyncio
    async def test_delete_project_cascades_task_edges(self, store):
        await store.create_node(
            TENANT, PROJECT_TYPE, {"name": "Doomed"}, "user:pm", node_id="proj-doom"
        )
        for i in range(3):
            t = await store.create_node(
                TENANT, TASK_TYPE, {"title": f"T{i}"}, "user:1", node_id=f"doom-t-{i}"
            )
            await store.create_edge(TENANT, EDGE_PROJ_TASK, "proj-doom", t.node_id)
        await store.delete_node(TENANT, "proj-doom")
        edges = await store.get_edges_from(TENANT, "proj-doom", edge_type_id=EDGE_PROJ_TASK)
        assert len(edges) == 0
        # Tasks themselves still exist
        for i in range(3):
            t = await store.get_node(TENANT, f"doom-t-{i}")
            assert t is not None

    @pytest.mark.asyncio
    async def test_rename_project(self, store):
        await store.create_node(
            TENANT, PROJECT_TYPE, {"name": "Old name"}, "user:pm", node_id="proj-rename"
        )
        updated = await store.update_node(TENANT, "proj-rename", {"name": "New name"})
        assert updated.payload["name"] == "New name"

    @pytest.mark.asyncio
    async def test_project_task_count(self, store):
        await store.create_node(
            TENANT, PROJECT_TYPE, {"name": "Count"}, "user:pm", node_id="proj-count"
        )
        for i in range(12):
            t = await store.create_node(TENANT, TASK_TYPE, {"title": f"C{i}"}, "user:1")
            await store.create_edge(TENANT, EDGE_PROJ_TASK, "proj-count", t.node_id)
        edges = await store.get_edges_from(TENANT, "proj-count", edge_type_id=EDGE_PROJ_TASK)
        assert len(edges) == 12

    @pytest.mark.asyncio
    async def test_project_member_count(self, store):
        await store.create_node(
            TENANT, PROJECT_TYPE, {"name": "Team"}, "user:pm", node_id="proj-mc"
        )
        for i in range(7):
            await store.create_edge(TENANT, EDGE_USER_PROJ, f"user:{i}", "proj-mc")
        members = await store.get_edges_to(TENANT, "proj-mc", edge_type_id=EDGE_USER_PROJ)
        assert len(members) == 7

    @pytest.mark.asyncio
    async def test_cross_project_task_sharing(self, store):
        await store.create_node(
            TENANT, PROJECT_TYPE, {"name": "P1"}, "user:pm", node_id="proj-share-1"
        )
        await store.create_node(
            TENANT, PROJECT_TYPE, {"name": "P2"}, "user:pm", node_id="proj-share-2"
        )
        task = await store.create_node(
            TENANT, TASK_TYPE, {"title": "Shared task"}, "user:1", node_id="shared-task"
        )
        await store.create_edge(TENANT, EDGE_PROJ_TASK, "proj-share-1", task.node_id)
        await store.create_edge(TENANT, EDGE_PROJ_TASK, "proj-share-2", task.node_id)
        incoming = await store.get_edges_to(TENANT, "shared-task", edge_type_id=EDGE_PROJ_TASK)
        assert len(incoming) == 2
        projects = {e.from_node_id for e in incoming}
        assert projects == {"proj-share-1", "proj-share-2"}

    @pytest.mark.asyncio
    async def test_project_owner_as_metadata(self, store):
        proj = await store.create_node(
            TENANT,
            PROJECT_TYPE,
            {"name": "Owned", "owner": "user:cto", "department": "engineering"},
            "user:cto",
        )
        fetched = await store.get_node(TENANT, proj.node_id)
        assert fetched.payload["owner"] == "user:cto"
        assert fetched.owner_actor == "user:cto"


# =========================================================================
# TASK ASSIGNMENT
# =========================================================================


@pytest.mark.integration
class TestTaskAssignment:
    @pytest.mark.asyncio
    async def test_assign_user_to_task(self, store):
        task = await store.create_node(
            TENANT, TASK_TYPE, {"title": "Assign"}, "user:pm", node_id="assign-t"
        )
        edge = await store.create_edge(TENANT, EDGE_USER_TASK, "user:alice", task.node_id)
        assert edge.edge_type_id == EDGE_USER_TASK

    @pytest.mark.asyncio
    async def test_unassign(self, store):
        await store.create_node(
            TENANT, TASK_TYPE, {"title": "Unassign"}, "user:pm", node_id="unassign-t"
        )
        await store.create_edge(TENANT, EDGE_USER_TASK, "user:bob", "unassign-t")
        result = await store.delete_edge(TENANT, EDGE_USER_TASK, "user:bob", "unassign-t")
        assert result is True

    @pytest.mark.asyncio
    async def test_reassign(self, store):
        await store.create_node(
            TENANT, TASK_TYPE, {"title": "Reassign"}, "user:pm", node_id="reassign-t"
        )
        await store.create_edge(TENANT, EDGE_USER_TASK, "user:alice", "reassign-t")
        await store.delete_edge(TENANT, EDGE_USER_TASK, "user:alice", "reassign-t")
        await store.create_edge(TENANT, EDGE_USER_TASK, "user:bob", "reassign-t")
        edges = await store.get_edges_to(TENANT, "reassign-t", edge_type_id=EDGE_USER_TASK)
        assert len(edges) == 1
        assert edges[0].from_node_id == "user:bob"

    @pytest.mark.asyncio
    async def test_list_user_tasks(self, store):
        for i in range(5):
            await store.create_node(
                TENANT, TASK_TYPE, {"title": f"UT{i}"}, "user:pm", node_id=f"ut-{i}"
            )
            await store.create_edge(TENANT, EDGE_USER_TASK, "user:alice", f"ut-{i}")
        edges = await store.get_edges_from(TENANT, "user:alice", edge_type_id=EDGE_USER_TASK)
        assert len(edges) == 5

    @pytest.mark.asyncio
    async def test_list_task_assignees(self, store):
        await store.create_node(
            TENANT, TASK_TYPE, {"title": "Multi-assign"}, "user:pm", node_id="ma-t"
        )
        for user in ["user:alice", "user:bob", "user:charlie"]:
            await store.create_edge(TENANT, EDGE_USER_TASK, user, "ma-t")
        edges = await store.get_edges_to(TENANT, "ma-t", edge_type_id=EDGE_USER_TASK)
        assert len(edges) == 3
        assignees = {e.from_node_id for e in edges}
        assert assignees == {"user:alice", "user:bob", "user:charlie"}

    @pytest.mark.asyncio
    async def test_multiple_assignees_one_task(self, store):
        await store.create_node(
            TENANT, TASK_TYPE, {"title": "Pair program"}, "user:pm", node_id="pair-t"
        )
        await store.create_edge(TENANT, EDGE_USER_TASK, "user:dev1", "pair-t")
        await store.create_edge(TENANT, EDGE_USER_TASK, "user:dev2", "pair-t")
        edges = await store.get_edges_to(TENANT, "pair-t", edge_type_id=EDGE_USER_TASK)
        assert len(edges) == 2

    @pytest.mark.asyncio
    async def test_user_with_no_tasks(self, store):
        edges = await store.get_edges_from(TENANT, "user:lonely", edge_type_id=EDGE_USER_TASK)
        assert len(edges) == 0

    @pytest.mark.asyncio
    async def test_assign_to_nonexistent_user(self, store):
        await store.create_node(
            TENANT, TASK_TYPE, {"title": "Ghost assign"}, "user:pm", node_id="ghost-t"
        )
        edge = await store.create_edge(TENANT, EDGE_USER_TASK, "user:nonexistent", "ghost-t")
        assert edge.from_node_id == "user:nonexistent"

    @pytest.mark.asyncio
    async def test_bulk_assign_10_tasks(self, store):
        task_ids = []
        for i in range(10):
            t = await store.create_node(
                TENANT, TASK_TYPE, {"title": f"Bulk {i}"}, "user:pm", node_id=f"bulk-t-{i}"
            )
            task_ids.append(t.node_id)
        for tid in task_ids:
            await store.create_edge(TENANT, EDGE_USER_TASK, "user:worker", tid)
        edges = await store.get_edges_from(TENANT, "user:worker", edge_type_id=EDGE_USER_TASK)
        assert len(edges) == 10

    @pytest.mark.asyncio
    async def test_unassign_all_from_task(self, store):
        await store.create_node(TENANT, TASK_TYPE, {"title": "Clear"}, "user:pm", node_id="clear-t")
        users = ["user:a", "user:b", "user:c"]
        for u in users:
            await store.create_edge(TENANT, EDGE_USER_TASK, u, "clear-t")
        for u in users:
            await store.delete_edge(TENANT, EDGE_USER_TASK, u, "clear-t")
        edges = await store.get_edges_to(TENANT, "clear-t", edge_type_id=EDGE_USER_TASK)
        assert len(edges) == 0

    @pytest.mark.asyncio
    async def test_assignment_history_via_edges(self, store):
        await store.create_node(
            TENANT, TASK_TYPE, {"title": "History"}, "user:pm", node_id="hist-t"
        )
        # Assign, unassign, reassign
        await store.create_edge(TENANT, EDGE_USER_TASK, "user:alice", "hist-t")
        await store.delete_edge(TENANT, EDGE_USER_TASK, "user:alice", "hist-t")
        await store.create_edge(TENANT, EDGE_USER_TASK, "user:bob", "hist-t")
        edges = await store.get_edges_to(TENANT, "hist-t", edge_type_id=EDGE_USER_TASK)
        assert len(edges) == 1
        assert edges[0].from_node_id == "user:bob"

    @pytest.mark.asyncio
    async def test_self_assignment(self, store):
        await store.create_node(
            TENANT, TASK_TYPE, {"title": "Self task"}, "user:alice", node_id="self-t"
        )
        edge = await store.create_edge(TENANT, EDGE_USER_TASK, "user:alice", "self-t")
        assert edge.from_node_id == "user:alice"

    @pytest.mark.asyncio
    async def test_assignment_edge_has_timestamp(self, store):
        await store.create_node(TENANT, TASK_TYPE, {"title": "Timed"}, "user:pm", node_id="timed-t")
        before = int(time.time() * 1000)
        edge = await store.create_edge(TENANT, EDGE_USER_TASK, "user:alice", "timed-t")
        after = int(time.time() * 1000)
        assert before <= edge.created_at <= after

    @pytest.mark.asyncio
    async def test_assignment_with_props(self, store):
        await store.create_node(
            TENANT, TASK_TYPE, {"title": "Role assign"}, "user:pm", node_id="role-t"
        )
        edge = await store.create_edge(
            TENANT,
            EDGE_USER_TASK,
            "user:alice",
            "role-t",
            props={"role": "reviewer", "assigned_by": "user:pm"},
        )
        assert edge.props["role"] == "reviewer"

    @pytest.mark.asyncio
    async def test_delete_task_removes_assignments(self, store):
        await store.create_node(
            TENANT, TASK_TYPE, {"title": "Del assign"}, "user:pm", node_id="del-assign-t"
        )
        await store.create_edge(TENANT, EDGE_USER_TASK, "user:alice", "del-assign-t")
        await store.create_edge(TENANT, EDGE_USER_TASK, "user:bob", "del-assign-t")
        await store.delete_node(TENANT, "del-assign-t")
        edges = await store.get_edges_to(TENANT, "del-assign-t", edge_type_id=EDGE_USER_TASK)
        assert len(edges) == 0


# =========================================================================
# TASK DEPENDENCIES
# =========================================================================


@pytest.mark.integration
class TestTaskDependencies:
    @pytest.mark.asyncio
    async def test_task_depends_on_task(self, store):
        await store.create_node(TENANT, TASK_TYPE, {"title": "Task A"}, "user:1", node_id="dep-a")
        await store.create_node(TENANT, TASK_TYPE, {"title": "Task B"}, "user:1", node_id="dep-b")
        edge = await store.create_edge(TENANT, EDGE_TASK_DEP, "dep-b", "dep-a")
        assert edge.from_node_id == "dep-b"
        assert edge.to_node_id == "dep-a"

    @pytest.mark.asyncio
    async def test_dependency_chain(self, store):
        for label in ["A", "B", "C"]:
            await store.create_node(
                TENANT, TASK_TYPE, {"title": f"Chain {label}"}, "user:1", node_id=f"chain-{label}"
            )
        # C depends on B, B depends on A
        await store.create_edge(TENANT, EDGE_TASK_DEP, "chain-C", "chain-B")
        await store.create_edge(TENANT, EDGE_TASK_DEP, "chain-B", "chain-A")

        c_deps = await store.get_edges_from(TENANT, "chain-C", edge_type_id=EDGE_TASK_DEP)
        b_deps = await store.get_edges_from(TENANT, "chain-B", edge_type_id=EDGE_TASK_DEP)
        a_deps = await store.get_edges_from(TENANT, "chain-A", edge_type_id=EDGE_TASK_DEP)
        assert len(c_deps) == 1
        assert len(b_deps) == 1
        assert len(a_deps) == 0

    @pytest.mark.asyncio
    async def test_circular_dependency_allowed(self, store):
        await store.create_node(TENANT, TASK_TYPE, {"title": "Circ A"}, "user:1", node_id="circ-a")
        await store.create_node(TENANT, TASK_TYPE, {"title": "Circ B"}, "user:1", node_id="circ-b")
        await store.create_edge(TENANT, EDGE_TASK_DEP, "circ-a", "circ-b")
        await store.create_edge(TENANT, EDGE_TASK_DEP, "circ-b", "circ-a")
        a_deps = await store.get_edges_from(TENANT, "circ-a", edge_type_id=EDGE_TASK_DEP)
        b_deps = await store.get_edges_from(TENANT, "circ-b", edge_type_id=EDGE_TASK_DEP)
        assert len(a_deps) == 1
        assert len(b_deps) == 1

    @pytest.mark.asyncio
    async def test_list_blockers_for_task(self, store):
        await store.create_node(
            TENANT, TASK_TYPE, {"title": "Blocked"}, "user:1", node_id="blocked"
        )
        for i in range(3):
            await store.create_node(
                TENANT, TASK_TYPE, {"title": f"Blocker {i}"}, "user:1", node_id=f"blocker-{i}"
            )
            await store.create_edge(TENANT, EDGE_TASK_DEP, "blocked", f"blocker-{i}")
        deps = await store.get_edges_from(TENANT, "blocked", edge_type_id=EDGE_TASK_DEP)
        assert len(deps) == 3

    @pytest.mark.asyncio
    async def test_list_dependents(self, store):
        await store.create_node(
            TENANT, TASK_TYPE, {"title": "Foundation"}, "user:1", node_id="foundation"
        )
        for i in range(4):
            await store.create_node(
                TENANT, TASK_TYPE, {"title": f"Dependent {i}"}, "user:1", node_id=f"dependent-{i}"
            )
            await store.create_edge(TENANT, EDGE_TASK_DEP, f"dependent-{i}", "foundation")
        dependents = await store.get_edges_to(TENANT, "foundation", edge_type_id=EDGE_TASK_DEP)
        assert len(dependents) == 4

    @pytest.mark.asyncio
    async def test_remove_dependency(self, store):
        await store.create_node(
            TENANT, TASK_TYPE, {"title": "Rm dep A"}, "user:1", node_id="rm-dep-a"
        )
        await store.create_node(
            TENANT, TASK_TYPE, {"title": "Rm dep B"}, "user:1", node_id="rm-dep-b"
        )
        await store.create_edge(TENANT, EDGE_TASK_DEP, "rm-dep-b", "rm-dep-a")
        result = await store.delete_edge(TENANT, EDGE_TASK_DEP, "rm-dep-b", "rm-dep-a")
        assert result is True
        deps = await store.get_edges_from(TENANT, "rm-dep-b", edge_type_id=EDGE_TASK_DEP)
        assert len(deps) == 0

    @pytest.mark.asyncio
    async def test_multiple_dependencies(self, store):
        await store.create_node(
            TENANT, TASK_TYPE, {"title": "Multi dep"}, "user:1", node_id="multi-dep"
        )
        for i in range(5):
            await store.create_node(
                TENANT, TASK_TYPE, {"title": f"Dep {i}"}, "user:1", node_id=f"md-{i}"
            )
            await store.create_edge(TENANT, EDGE_TASK_DEP, "multi-dep", f"md-{i}")
        deps = await store.get_edges_from(TENANT, "multi-dep", edge_type_id=EDGE_TASK_DEP)
        assert len(deps) == 5

    @pytest.mark.asyncio
    async def test_dependency_across_projects(self, store):
        await store.create_node(
            TENANT, PROJECT_TYPE, {"name": "P1"}, "user:pm", node_id="dep-proj-1"
        )
        await store.create_node(
            TENANT, PROJECT_TYPE, {"name": "P2"}, "user:pm", node_id="dep-proj-2"
        )
        t1 = await store.create_node(
            TENANT, TASK_TYPE, {"title": "T in P1"}, "user:1", node_id="dep-t1"
        )
        t2 = await store.create_node(
            TENANT, TASK_TYPE, {"title": "T in P2"}, "user:1", node_id="dep-t2"
        )
        await store.create_edge(TENANT, EDGE_PROJ_TASK, "dep-proj-1", t1.node_id)
        await store.create_edge(TENANT, EDGE_PROJ_TASK, "dep-proj-2", t2.node_id)
        # t2 depends on t1 (cross-project)
        await store.create_edge(TENANT, EDGE_TASK_DEP, t2.node_id, t1.node_id)
        deps = await store.get_edges_from(TENANT, "dep-t2", edge_type_id=EDGE_TASK_DEP)
        assert len(deps) == 1
        assert deps[0].to_node_id == "dep-t1"

    @pytest.mark.asyncio
    async def test_orphan_task_no_dependencies(self, store):
        await store.create_node(
            TENANT, TASK_TYPE, {"title": "Orphan"}, "user:1", node_id="orphan-t"
        )
        deps_from = await store.get_edges_from(TENANT, "orphan-t", edge_type_id=EDGE_TASK_DEP)
        deps_to = await store.get_edges_to(TENANT, "orphan-t", edge_type_id=EDGE_TASK_DEP)
        assert len(deps_from) == 0
        assert len(deps_to) == 0

    @pytest.mark.asyncio
    async def test_self_dependency(self, store):
        await store.create_node(
            TENANT, TASK_TYPE, {"title": "Self ref"}, "user:1", node_id="self-dep"
        )
        edge = await store.create_edge(TENANT, EDGE_TASK_DEP, "self-dep", "self-dep")
        assert edge.from_node_id == edge.to_node_id


# =========================================================================
# TASK QUERIES
# =========================================================================


@pytest.mark.integration
class TestTaskQueries:
    @pytest.mark.asyncio
    async def test_get_all_tasks_by_status(self, store):
        statuses = ["todo", "todo", "in_progress", "in_progress", "in_progress", "done"]
        for i, status in enumerate(statuses):
            await store.create_node(
                TENANT, TASK_TYPE, {"title": f"Q{i}", "status": status}, "user:1"
            )
        tasks = await store.get_nodes_by_type(TENANT, TASK_TYPE)
        todo = [t for t in tasks if t.payload["status"] == "todo"]
        ip = [t for t in tasks if t.payload["status"] == "in_progress"]
        done = [t for t in tasks if t.payload["status"] == "done"]
        assert len(todo) == 2
        assert len(ip) == 3
        assert len(done) == 1

    @pytest.mark.asyncio
    async def test_get_tasks_by_assignee(self, store):
        for i in range(4):
            t = await store.create_node(
                TENANT, TASK_TYPE, {"title": f"Alice {i}"}, "user:pm", node_id=f"a-task-{i}"
            )
            await store.create_edge(TENANT, EDGE_USER_TASK, "user:alice", t.node_id)
        for i in range(2):
            t = await store.create_node(
                TENANT, TASK_TYPE, {"title": f"Bob {i}"}, "user:pm", node_id=f"b-task-{i}"
            )
            await store.create_edge(TENANT, EDGE_USER_TASK, "user:bob", t.node_id)
        alice_edges = await store.get_edges_from(TENANT, "user:alice", edge_type_id=EDGE_USER_TASK)
        bob_edges = await store.get_edges_from(TENANT, "user:bob", edge_type_id=EDGE_USER_TASK)
        assert len(alice_edges) == 4
        assert len(bob_edges) == 2

    @pytest.mark.asyncio
    async def test_get_overdue_tasks(self, store):
        now_ms = int(time.time() * 1000)
        for i in range(3):
            await store.create_node(
                TENANT,
                TASK_TYPE,
                {"title": f"Late {i}", "due_date": now_ms - 86400000, "status": "todo"},
                "user:1",
            )
        for i in range(2):
            await store.create_node(
                TENANT,
                TASK_TYPE,
                {"title": f"Future {i}", "due_date": now_ms + 86400000, "status": "todo"},
                "user:1",
            )
        tasks = await store.get_nodes_by_type(TENANT, TASK_TYPE)
        overdue = [t for t in tasks if t.payload.get("due_date", float("inf")) < now_ms]
        assert len(overdue) == 3

    @pytest.mark.asyncio
    async def test_get_tasks_by_project(self, store):
        await store.create_node(
            TENANT, PROJECT_TYPE, {"name": "Query proj"}, "user:pm", node_id="q-proj"
        )
        for i in range(6):
            t = await store.create_node(TENANT, TASK_TYPE, {"title": f"QT{i}"}, "user:1")
            await store.create_edge(TENANT, EDGE_PROJ_TASK, "q-proj", t.node_id)
        edges = await store.get_edges_from(TENANT, "q-proj", edge_type_id=EDGE_PROJ_TASK)
        assert len(edges) == 6

    @pytest.mark.asyncio
    async def test_task_count_by_status(self, store):
        for status, count in [("todo", 5), ("in_progress", 3), ("done", 7)]:
            for i in range(count):
                await store.create_node(
                    TENANT, TASK_TYPE, {"title": f"{status}-{i}", "status": status}, "user:1"
                )
        tasks = await store.get_nodes_by_type(TENANT, TASK_TYPE, limit=100)
        status_counts = {}
        for t in tasks:
            s = t.payload["status"]
            status_counts[s] = status_counts.get(s, 0) + 1
        assert status_counts == {"todo": 5, "in_progress": 3, "done": 7}

    @pytest.mark.asyncio
    async def test_tasks_sorted_by_due_date(self, store):
        now_ms = int(time.time() * 1000)
        for i in range(5):
            await store.create_node(
                TENANT,
                TASK_TYPE,
                {"title": f"Sorted {i}", "due_date": now_ms + (i * 86400000)},
                "user:1",
            )
        tasks = await store.get_nodes_by_type(TENANT, TASK_TYPE)
        due_dates = [t.payload["due_date"] for t in tasks]
        # Verify all due dates are present (they come back in created_at DESC order)
        assert len(due_dates) == 5
        assert len(set(due_dates)) == 5

    @pytest.mark.asyncio
    async def test_tasks_with_pagination(self, store):
        for i in range(25):
            await store.create_node(
                TENANT, TASK_TYPE, {"title": f"Page {i}", "status": "todo"}, "user:1"
            )
        page1 = await store.get_nodes_by_type(TENANT, TASK_TYPE, limit=10, offset=0)
        page2 = await store.get_nodes_by_type(TENANT, TASK_TYPE, limit=10, offset=10)
        page3 = await store.get_nodes_by_type(TENANT, TASK_TYPE, limit=10, offset=20)
        assert len(page1) == 10
        assert len(page2) == 10
        assert len(page3) == 5
        all_ids = set()
        for page in [page1, page2, page3]:
            for t in page:
                all_ids.add(t.node_id)
        assert len(all_ids) == 25

    @pytest.mark.asyncio
    async def test_tasks_for_user_across_projects(self, store):
        for p in range(3):
            await store.create_node(
                TENANT,
                PROJECT_TYPE,
                {"name": f"Multi P{p}"},
                "user:pm",
                node_id=f"mp-{p}",
            )
            for t in range(2):
                task = await store.create_node(
                    TENANT,
                    TASK_TYPE,
                    {"title": f"P{p}-T{t}"},
                    "user:pm",
                    node_id=f"mp-{p}-t-{t}",
                )
                await store.create_edge(TENANT, EDGE_PROJ_TASK, f"mp-{p}", task.node_id)
                await store.create_edge(TENANT, EDGE_USER_TASK, "user:alice", task.node_id)
        alice_tasks = await store.get_edges_from(TENANT, "user:alice", edge_type_id=EDGE_USER_TASK)
        assert len(alice_tasks) == 6

    @pytest.mark.asyncio
    async def test_empty_task_list(self, store):
        tasks = await store.get_nodes_by_type(TENANT, TASK_TYPE)
        assert tasks == []

    @pytest.mark.asyncio
    async def test_tasks_with_mixed_statuses(self, store):
        for status in ["todo", "in_progress", "done", "blocked", "review"]:
            await store.create_node(
                TENANT, TASK_TYPE, {"title": f"Mixed {status}", "status": status}, "user:1"
            )
        tasks = await store.get_nodes_by_type(TENANT, TASK_TYPE)
        statuses = {t.payload["status"] for t in tasks}
        assert statuses == {"todo", "in_progress", "done", "blocked", "review"}


# =========================================================================
# WORKSPACE INTEGRATION
# =========================================================================


@pytest.mark.integration
class TestWorkspaceIntegration:
    @pytest.mark.asyncio
    async def test_full_flow_user_project_tasks(self, store):
        # Create users
        alice = await store.create_node(
            TENANT,
            USER_TYPE,
            {"name": "Alice", "email": "alice@co.edu", "role": "developer"},
            "system",
            node_id="user-alice",
        )
        bob = await store.create_node(
            TENANT,
            USER_TYPE,
            {"name": "Bob", "email": "bob@co.edu", "role": "designer"},
            "system",
            node_id="user-bob",
        )

        # Create project
        proj = await store.create_node(
            TENANT,
            PROJECT_TYPE,
            {"name": "Campus App", "description": "Mobile app", "owner": "user-alice"},
            "user-alice",
            node_id="proj-campus",
        )

        # Add members
        await store.create_edge(TENANT, EDGE_USER_PROJ, alice.node_id, proj.node_id)
        await store.create_edge(TENANT, EDGE_USER_PROJ, bob.node_id, proj.node_id)

        # Create tasks
        tasks = []
        for i, title in enumerate(["Login UI", "API endpoints", "Database schema"]):
            t = await store.create_node(
                TENANT,
                TASK_TYPE,
                {"title": title, "status": "todo"},
                "user-alice",
                node_id=f"task-{i}",
            )
            await store.create_edge(TENANT, EDGE_PROJ_TASK, proj.node_id, t.node_id)
            tasks.append(t)

        # Assign tasks
        await store.create_edge(TENANT, EDGE_USER_TASK, alice.node_id, tasks[1].node_id)
        await store.create_edge(TENANT, EDGE_USER_TASK, bob.node_id, tasks[0].node_id)
        await store.create_edge(TENANT, EDGE_USER_TASK, alice.node_id, tasks[2].node_id)

        # Add dependency: API endpoints depends on Database schema
        await store.create_edge(TENANT, EDGE_TASK_DEP, tasks[1].node_id, tasks[2].node_id)

        # Verify
        proj_tasks = await store.get_edges_from(TENANT, "proj-campus", edge_type_id=EDGE_PROJ_TASK)
        assert len(proj_tasks) == 3

        proj_members = await store.get_edges_to(TENANT, "proj-campus", edge_type_id=EDGE_USER_PROJ)
        assert len(proj_members) == 2

        alice_tasks = await store.get_edges_from(TENANT, "user-alice", edge_type_id=EDGE_USER_TASK)
        assert len(alice_tasks) == 2

    @pytest.mark.asyncio
    async def test_user_creates_post_about_task(self, store):
        task = await store.create_node(
            TENANT,
            TASK_TYPE,
            {"title": "Fix bug #123", "status": "in_progress"},
            "user:alice",
            node_id="bug-123",
        )
        # Simulate a forum post referencing the task
        post_type = 3
        await store.create_node(
            TENANT,
            post_type,
            {
                "title": "Need help with bug #123",
                "body": "Anyone know how to fix this?",
                "related_task": task.node_id,
            },
            "user:alice",
        )
        posts = await store.get_nodes_by_type(TENANT, post_type)
        assert len(posts) == 1
        assert posts[0].payload["related_task"] == "bug-123"

    @pytest.mark.asyncio
    async def test_task_completion_status_update(self, store):
        task = await store.create_node(
            TENANT,
            TASK_TYPE,
            {"title": "Ship feature", "status": "in_progress"},
            "user:1",
        )
        # Complete
        updated = await store.update_node(
            TENANT,
            task.node_id,
            {"status": "done", "completed_at": int(time.time() * 1000)},
        )
        assert updated.payload["status"] == "done"
        assert "completed_at" in updated.payload

    @pytest.mark.asyncio
    async def test_bulk_task_creation_20_tasks(self, store):
        await store.create_node(
            TENANT, PROJECT_TYPE, {"name": "Sprint"}, "user:pm", node_id="sprint-proj"
        )
        for i in range(20):
            t = await store.create_node(
                TENANT,
                TASK_TYPE,
                {"title": f"Sprint task {i}", "status": "todo", "story_points": (i % 5) + 1},
                "user:pm",
            )
            await store.create_edge(TENANT, EDGE_PROJ_TASK, "sprint-proj", t.node_id)
        edges = await store.get_edges_from(TENANT, "sprint-proj", edge_type_id=EDGE_PROJ_TASK)
        assert len(edges) == 20

    @pytest.mark.asyncio
    async def test_project_dashboard_task_counts(self, store):
        await store.create_node(
            TENANT, PROJECT_TYPE, {"name": "Dashboard"}, "user:pm", node_id="dash-proj"
        )
        status_plan = {"todo": 5, "in_progress": 3, "done": 8, "blocked": 2}
        for status, count in status_plan.items():
            for i in range(count):
                t = await store.create_node(
                    TENANT,
                    TASK_TYPE,
                    {"title": f"{status}-{i}", "status": status},
                    "user:pm",
                )
                await store.create_edge(TENANT, EDGE_PROJ_TASK, "dash-proj", t.node_id)

        # Get all project tasks
        edges = await store.get_edges_from(TENANT, "dash-proj", edge_type_id=EDGE_PROJ_TASK)
        assert len(edges) == 18
        # Fetch actual tasks and count by status
        counts = {}
        for e in edges:
            t = await store.get_node(TENANT, e.to_node_id)
            s = t.payload["status"]
            counts[s] = counts.get(s, 0) + 1
        assert counts == status_plan

    @pytest.mark.asyncio
    async def test_team_workload(self, store):
        users = ["user:alice", "user:bob", "user:charlie"]
        task_distribution = {"user:alice": 5, "user:bob": 3, "user:charlie": 7}
        for user in users:
            for i in range(task_distribution[user]):
                t = await store.create_node(
                    TENANT,
                    TASK_TYPE,
                    {"title": f"{user} task {i}", "status": "in_progress"},
                    "user:pm",
                )
                await store.create_edge(TENANT, EDGE_USER_TASK, user, t.node_id)
        for user in users:
            edges = await store.get_edges_from(TENANT, user, edge_type_id=EDGE_USER_TASK)
            assert len(edges) == task_distribution[user]

    @pytest.mark.asyncio
    async def test_multi_tenant_workspace_isolation(self, multi_store):
        await multi_store.create_node(
            TENANT,
            TASK_TYPE,
            {"title": "Tenant A task"},
            "user:1",
            node_id="iso-task",
        )
        await multi_store.create_node(
            "workspace-other",
            TASK_TYPE,
            {"title": "Tenant B task"},
            "user:1",
            node_id="iso-task",
        )
        a = await multi_store.get_node(TENANT, "iso-task")
        b = await multi_store.get_node("workspace-other", "iso-task")
        assert a.payload["title"] == "Tenant A task"
        assert b.payload["title"] == "Tenant B task"

    @pytest.mark.asyncio
    async def test_large_workspace(self, store):
        # Create 20 users
        for i in range(20):
            await store.create_node(
                TENANT,
                USER_TYPE,
                {"name": f"User {i}", "email": f"user{i}@co.edu"},
                "system",
                node_id=f"lw-user-{i}",
            )
        # Create 5 projects
        for p in range(5):
            await store.create_node(
                TENANT,
                PROJECT_TYPE,
                {"name": f"Project {p}"},
                "user:pm",
                node_id=f"lw-proj-{p}",
            )
            # 10 tasks per project
            for t in range(10):
                task = await store.create_node(
                    TENANT,
                    TASK_TYPE,
                    {"title": f"P{p}-T{t}", "status": "todo"},
                    "user:pm",
                )
                await store.create_edge(TENANT, EDGE_PROJ_TASK, f"lw-proj-{p}", task.node_id)
                # Assign to a user
                await store.create_edge(
                    TENANT, EDGE_USER_TASK, f"lw-user-{(p * 10 + t) % 20}", task.node_id
                )

        # Verify counts
        all_tasks = await store.get_nodes_by_type(TENANT, TASK_TYPE, limit=100)
        assert len(all_tasks) == 50
        all_users = await store.get_nodes_by_type(TENANT, USER_TYPE, limit=100)
        assert len(all_users) == 20
        all_projects = await store.get_nodes_by_type(TENANT, PROJECT_TYPE, limit=100)
        assert len(all_projects) == 5

    @pytest.mark.asyncio
    async def test_delete_user_removes_assignment_edges(self, store):
        await store.create_node(
            TENANT, USER_TYPE, {"name": "Leaving"}, "system", node_id="leaving-user"
        )
        for i in range(3):
            await store.create_node(
                TENANT, TASK_TYPE, {"title": f"Task {i}"}, "user:pm", node_id=f"lu-t-{i}"
            )
            await store.create_edge(TENANT, EDGE_USER_TASK, "leaving-user", f"lu-t-{i}")
        await store.delete_node(TENANT, "leaving-user")
        edges = await store.get_edges_from(TENANT, "leaving-user", edge_type_id=EDGE_USER_TASK)
        assert len(edges) == 0
        # Tasks still exist
        for i in range(3):
            t = await store.get_node(TENANT, f"lu-t-{i}")
            assert t is not None

    @pytest.mark.asyncio
    async def test_user_role_based_metadata(self, store):
        roles = {
            "user-admin": "admin",
            "user-dev": "developer",
            "user-viewer": "viewer",
        }
        for uid, role in roles.items():
            await store.create_node(
                TENANT,
                USER_TYPE,
                {"name": uid, "role": role},
                "system",
                node_id=uid,
            )
        for uid, role in roles.items():
            u = await store.get_node(TENANT, uid)
            assert u.payload["role"] == role

    @pytest.mark.asyncio
    async def test_kanban_board_simulation(self, store):
        columns = ["backlog", "todo", "in_progress", "review", "done"]
        task_counts = {"backlog": 4, "todo": 3, "in_progress": 2, "review": 1, "done": 5}
        for status in columns:
            for i in range(task_counts[status]):
                await store.create_node(
                    TENANT,
                    TASK_TYPE,
                    {"title": f"Kanban {status} {i}", "status": status},
                    "user:1",
                )
        tasks = await store.get_nodes_by_type(TENANT, TASK_TYPE, limit=100)
        board = {}
        for t in tasks:
            s = t.payload["status"]
            board.setdefault(s, []).append(t)
        for status in columns:
            assert len(board[status]) == task_counts[status]

    @pytest.mark.asyncio
    async def test_sprint_planning_filter_by_due_date(self, store):
        now_ms = int(time.time() * 1000)
        sprint_start = now_ms
        sprint_end = now_ms + 14 * 86400000  # 2 weeks

        # Sprint tasks (due within 2 weeks)
        for i in range(5):
            await store.create_node(
                TENANT,
                TASK_TYPE,
                {
                    "title": f"Sprint {i}",
                    "status": "todo",
                    "due_date": sprint_start + (i + 1) * 86400000,
                },
                "user:pm",
            )
        # Backlog tasks (due after sprint)
        for i in range(3):
            await store.create_node(
                TENANT,
                TASK_TYPE,
                {
                    "title": f"Backlog {i}",
                    "status": "todo",
                    "due_date": sprint_end + (i + 1) * 86400000,
                },
                "user:pm",
            )
        tasks = await store.get_nodes_by_type(TENANT, TASK_TYPE, limit=100)
        sprint_tasks = [
            t for t in tasks if sprint_start <= t.payload.get("due_date", 0) <= sprint_end
        ]
        backlog_tasks = [t for t in tasks if t.payload.get("due_date", 0) > sprint_end]
        assert len(sprint_tasks) == 5
        assert len(backlog_tasks) == 3

    @pytest.mark.asyncio
    async def test_workspace_search_across_types(self, store):
        await store.create_node(TENANT, USER_TYPE, {"name": "Alice"}, "system", node_id="s-user")
        await store.create_node(
            TENANT, PROJECT_TYPE, {"name": "Alpha"}, "user:pm", node_id="s-proj"
        )
        await store.create_node(
            TENANT, TASK_TYPE, {"title": "Build API"}, "user:1", node_id="s-task"
        )
        users = await store.get_nodes_by_type(TENANT, USER_TYPE)
        projects = await store.get_nodes_by_type(TENANT, PROJECT_TYPE)
        tasks = await store.get_nodes_by_type(TENANT, TASK_TYPE)
        assert len(users) == 1
        assert len(projects) == 1
        assert len(tasks) == 1
        assert users[0].payload["name"] == "Alice"
        assert projects[0].payload["name"] == "Alpha"
        assert tasks[0].payload["title"] == "Build API"

    @pytest.mark.asyncio
    async def test_concurrent_task_creation(self, store):
        async def create_task(idx):
            return await store.create_node(
                TENANT,
                TASK_TYPE,
                {"title": f"Concurrent {idx}", "status": "todo"},
                "user:pm",
            )

        tasks = await asyncio.gather(*[create_task(i) for i in range(30)])
        assert len(tasks) == 30
        ids = {t.node_id for t in tasks}
        assert len(ids) == 30

    @pytest.mark.asyncio
    async def test_project_with_mixed_task_statuses(self, store):
        await store.create_node(
            TENANT, PROJECT_TYPE, {"name": "Mixed"}, "user:pm", node_id="proj-mixed"
        )
        statuses = ["todo", "in_progress", "done", "todo", "blocked"]
        for i, status in enumerate(statuses):
            t = await store.create_node(
                TENANT, TASK_TYPE, {"title": f"Mixed {i}", "status": status}, "user:pm"
            )
            await store.create_edge(TENANT, EDGE_PROJ_TASK, "proj-mixed", t.node_id)
        edges = await store.get_edges_from(TENANT, "proj-mixed", edge_type_id=EDGE_PROJ_TASK)
        assert len(edges) == 5
        # Verify statuses match
        fetched_statuses = []
        for e in edges:
            t = await store.get_node(TENANT, e.to_node_id)
            fetched_statuses.append(t.payload["status"])
        assert sorted(fetched_statuses) == sorted(statuses)

    @pytest.mark.asyncio
    async def test_update_nonexistent_task_returns_none(self, store):
        result = await store.update_node(TENANT, "ghost-task", {"status": "done"})
        assert result is None
