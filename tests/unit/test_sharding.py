"""
Unit tests for multi-node tenant sharding.
"""
from __future__ import annotations

import pytest

from dbaas.entdb_server.sharding import ShardingConfig


@pytest.mark.unit
class TestShardingConfig:
    def test_from_env_parses_tenants(self, monkeypatch):
        monkeypatch.setenv("NODE_ID", "node-a")
        monkeypatch.setenv("ASSIGNED_TENANTS", "college-0,college-1,college-2")
        config = ShardingConfig.from_env()
        assert config.node_id == "node-a"
        assert config.assigned_tenants == frozenset({"college-0", "college-1", "college-2"})

    def test_empty_tenants_means_single_node(self, monkeypatch):
        monkeypatch.setenv("NODE_ID", "solo")
        monkeypatch.setenv("ASSIGNED_TENANTS", "")
        config = ShardingConfig.from_env()
        assert config.assigned_tenants == frozenset()
        assert not config.is_multi_node

    def test_no_env_vars_defaults(self):
        config = ShardingConfig()
        assert config.node_id == "default"
        assert config.assigned_tenants == frozenset()
        assert config.is_mine("any-tenant")

    def test_is_mine_positive(self):
        config = ShardingConfig(
            node_id="node-a",
            assigned_tenants=frozenset({"tenant-1", "tenant-2"}),
        )
        assert config.is_mine("tenant-1")
        assert config.is_mine("tenant-2")

    def test_is_mine_negative(self):
        config = ShardingConfig(
            node_id="node-a",
            assigned_tenants=frozenset({"tenant-1"}),
        )
        assert not config.is_mine("tenant-99")

    def test_is_mine_empty_accepts_all(self):
        config = ShardingConfig(node_id="solo", assigned_tenants=frozenset())
        assert config.is_mine("anything")
        assert config.is_mine("any-tenant-id")

    def test_is_multi_node(self):
        single = ShardingConfig(assigned_tenants=frozenset())
        multi = ShardingConfig(assigned_tenants=frozenset({"t1"}))
        assert not single.is_multi_node
        assert multi.is_multi_node

    def test_tenant_registry(self, monkeypatch):
        monkeypatch.setenv("NODE_ID", "node-a")
        monkeypatch.setenv("ASSIGNED_TENANTS", "college-0")
        monkeypatch.setenv("TENANT_REGISTRY", '{"college-0":"node-a","college-1":"node-b"}')
        config = ShardingConfig.from_env()
        assert config.get_owner("college-0") == "node-a"
        assert config.get_owner("college-1") == "node-b"
        assert config.get_owner("unknown") is None

    def test_invalid_registry_json_ignored(self, monkeypatch):
        monkeypatch.setenv("NODE_ID", "node-a")
        monkeypatch.setenv("ASSIGNED_TENANTS", "t1")
        monkeypatch.setenv("TENANT_REGISTRY", "not-json")
        config = ShardingConfig.from_env()
        assert config.tenant_registry is None

    def test_whitespace_in_tenants(self, monkeypatch):
        monkeypatch.setenv("ASSIGNED_TENANTS", " college-0 , college-1 , ")
        config = ShardingConfig.from_env()
        assert config.assigned_tenants == frozenset({"college-0", "college-1"})

    def test_get_owner_no_registry(self):
        config = ShardingConfig(node_id="solo")
        assert config.get_owner("anything") is None
