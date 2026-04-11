"""ACL v2 benchmarks: access check, inheritance, groups, connected nodes."""

import time
import uuid

import pytest

from dbaas.entdb_server.apply.canonical_store import CanonicalStore

TENANT = "bench_acl"
ALICE = "user:alice"
BOB = "user:bob"


@pytest.fixture
def store(tmp_path):
    s = CanonicalStore(data_dir=tmp_path)
    with s._get_connection(TENANT, create=True) as conn:
        s._create_schema(conn)
    return s


def _node(store, owner=ALICE, type_id=1, acl=None):
    nid = str(uuid.uuid4())
    now = int(time.time() * 1000)
    return store._sync_create_node(
        TENANT, type_id, {"title": "test"}, owner, nid, acl or [], now
    ).node_id


def _edge(store, from_id, to_id, edge_type_id=1, propagates_acl=False):
    now = int(time.time() * 1000)
    with store._get_connection(TENANT) as conn:
        conn.execute(
            "INSERT OR REPLACE INTO edges "
            "(tenant_id, edge_type_id, from_node_id, to_node_id, props_json, propagates_acl, created_at) "
            "VALUES (?, ?, ?, ?, ?, ?, ?)",
            (TENANT, edge_type_id, from_id, to_id, "{}", 1 if propagates_acl else 0, now),
        )
        if propagates_acl:
            conn.execute(
                "INSERT OR IGNORE INTO acl_inherit (node_id, inherit_from) VALUES (?, ?)",
                (to_id, from_id),
            )


def _share(store, node_id, actor_id, permission="read"):
    now = int(time.time() * 1000)
    store._sync_share_node(TENANT, node_id, actor_id, "user", permission, ALICE, now, None)


def _group_add(store, group_id, member):
    now = int(time.time() * 1000)
    store._sync_add_group_member(TENANT, group_id, member, "member", now)


@pytest.mark.benchmark(group="acl")
def test_can_access_point_check(store, benchmark):
    """Point check: direct share."""
    nid = _node(store)
    _share(store, nid, BOB)
    benchmark(store._sync_can_access, TENANT, nid, [BOB])


@pytest.mark.benchmark(group="acl")
def test_can_access_inherited_3_levels(store, benchmark):
    """Inherited access: 3-level chain."""
    root = _node(store, type_id=1)
    mid = _node(store, type_id=2)
    leaf = _node(store, type_id=3)
    _edge(store, root, mid, propagates_acl=True)
    _edge(store, mid, leaf, propagates_acl=True)
    _share(store, root, BOB)
    benchmark(store._sync_can_access, TENANT, leaf, [BOB])


@pytest.mark.benchmark(group="acl")
def test_can_access_with_groups(store, benchmark):
    """Access via group membership."""
    nid = _node(store)
    _group_add(store, "group:eng", BOB)
    _share(store, nid, "group:eng")
    groups = store._sync_resolve_actor_groups(TENANT, BOB)
    benchmark(store._sync_can_access, TENANT, nid, groups)


@pytest.mark.benchmark(group="acl")
def test_resolve_actor_groups(store, benchmark):
    """Resolve nested group memberships."""
    _group_add(store, "group:eng", BOB)
    _group_add(store, "group:design", BOB)
    _group_add(store, "group:all-staff", "group:eng")
    benchmark(store._sync_resolve_actor_groups, TENANT, BOB)


@pytest.mark.benchmark(group="acl")
def test_connected_nodes_10(store, benchmark):
    """Fetch 10 connected nodes with ACL."""
    parent = _node(store)
    for _ in range(10):
        _edge(store, parent, _node(store, type_id=2), edge_type_id=30, propagates_acl=True)
    _share(store, parent, BOB)
    benchmark(store._sync_get_connected_nodes, TENANT, parent, 30, [BOB], 100, 0)


@pytest.mark.benchmark(group="acl")
def test_connected_nodes_100(store, benchmark):
    """Fetch 100 connected nodes with ACL."""
    parent = _node(store)
    for _ in range(100):
        _edge(store, parent, _node(store, type_id=2), edge_type_id=31, propagates_acl=True)
    _share(store, parent, BOB)
    benchmark(store._sync_get_connected_nodes, TENANT, parent, 31, [BOB], 100, 0)


@pytest.mark.benchmark(group="acl")
def test_shared_with_me_50(store, benchmark):
    """List 50 shared nodes."""
    for _ in range(50):
        _share(store, _node(store), BOB)
    benchmark(store._sync_list_shared_with_me, TENANT, [BOB], 100, 0)
