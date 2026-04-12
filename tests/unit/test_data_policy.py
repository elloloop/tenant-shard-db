"""
Unit tests for data policy enforcement at runtime.

Tests cover:
- DataPolicy enum values and membership
- Default to PERSONAL when data_policy is unset
- get_data_policy returns correct policy from registry
- get_pii_fields returns PII-marked fields
- get_subject_field returns the correct field
- type_metadata table populated via sync_type_metadata
- Warnings logged for unclassified types
- Warnings logged for missing legal_basis on regulated policies
- NodeTypeDef serialization round-trip with data_policy fields
- FieldDef PII flag serialization round-trip
"""

from __future__ import annotations

import json
import logging
from pathlib import Path

import pytest

from dbaas.entdb_server.apply.canonical_store import CanonicalStore
from dbaas.entdb_server.data_policy import REQUIRES_LEGAL_BASIS, DataPolicy
from dbaas.entdb_server.schema.registry import SchemaRegistry
from dbaas.entdb_server.schema.types import NodeTypeDef, field

# ---------------------------------------------------------------------------
# DataPolicy enum
# ---------------------------------------------------------------------------


class TestDataPolicyEnum:
    """DataPolicy enum values and semantics."""

    def test_all_values_present(self):
        """All six expected policies exist."""
        expected = {"personal", "business", "financial", "audit", "ephemeral", "healthcare"}
        actual = {p.value for p in DataPolicy}
        assert actual == expected

    def test_personal_value(self):
        assert DataPolicy.PERSONAL.value == "personal"

    def test_business_value(self):
        assert DataPolicy.BUSINESS.value == "business"

    def test_financial_value(self):
        assert DataPolicy.FINANCIAL.value == "financial"

    def test_audit_value(self):
        assert DataPolicy.AUDIT.value == "audit"

    def test_ephemeral_value(self):
        assert DataPolicy.EPHEMERAL.value == "ephemeral"

    def test_healthcare_value(self):
        assert DataPolicy.HEALTHCARE.value == "healthcare"

    def test_is_str_enum(self):
        """DataPolicy values can be used directly as strings."""
        assert DataPolicy.PERSONAL == "personal"
        assert isinstance(DataPolicy.PERSONAL, str)

    def test_requires_legal_basis_set(self):
        """FINANCIAL, AUDIT, HEALTHCARE require legal_basis."""
        assert frozenset({"financial", "audit", "healthcare"}) == REQUIRES_LEGAL_BASIS


# ---------------------------------------------------------------------------
# Registry: get_data_policy
# ---------------------------------------------------------------------------


class TestGetDataPolicy:
    """SchemaRegistry.get_data_policy behaviour."""

    def test_returns_explicit_policy(self):
        """Explicit data_policy on NodeTypeDef is returned."""
        registry = SchemaRegistry()
        Patient = NodeTypeDef(
            type_id=1,
            name="Patient",
            data_policy=DataPolicy.HEALTHCARE,
            legal_basis="consent",
        )
        registry.register_node_type(Patient)

        assert registry.get_data_policy(1) == DataPolicy.HEALTHCARE

    def test_defaults_to_personal_when_unset(self, caplog):
        """Missing data_policy defaults to PERSONAL with a warning."""
        registry = SchemaRegistry()
        User = NodeTypeDef(type_id=1, name="User")
        registry.register_node_type(User)

        with caplog.at_level(logging.WARNING, logger="dbaas.entdb_server.schema.registry"):
            policy = registry.get_data_policy(1)

        assert policy == DataPolicy.PERSONAL
        assert any("defaulting to PERSONAL" in m for m in caplog.messages)

    def test_raises_for_unknown_type_id(self):
        """KeyError for unregistered type_id."""
        registry = SchemaRegistry()
        with pytest.raises(KeyError, match="Unknown type_id 999"):
            registry.get_data_policy(999)


# ---------------------------------------------------------------------------
# Registry: get_pii_fields
# ---------------------------------------------------------------------------


class TestGetPiiFields:
    """SchemaRegistry.get_pii_fields behaviour."""

    def test_returns_pii_field_names(self):
        """Fields marked pii=True are returned."""
        registry = SchemaRegistry()
        User = NodeTypeDef(
            type_id=1,
            name="User",
            fields=(
                field(1, "email", "str", pii=True),
                field(2, "display_name", "str"),
                field(3, "ssn", "str", pii=True),
            ),
            data_policy=DataPolicy.PERSONAL,
        )
        registry.register_node_type(User)

        pii = registry.get_pii_fields(1)
        assert sorted(pii) == ["email", "ssn"]

    def test_empty_when_no_pii(self):
        """Returns empty list when no fields are PII."""
        registry = SchemaRegistry()
        Config = NodeTypeDef(
            type_id=2,
            name="Config",
            fields=(field(1, "key", "str"), field(2, "value", "str")),
        )
        registry.register_node_type(Config)

        assert registry.get_pii_fields(2) == []

    def test_excludes_deprecated_pii_fields(self):
        """Deprecated PII fields are excluded."""
        registry = SchemaRegistry()
        User = NodeTypeDef(
            type_id=1,
            name="User",
            fields=(
                field(1, "email", "str", pii=True),
                field(2, "old_ssn", "str", pii=True, deprecated=True),
            ),
        )
        registry.register_node_type(User)

        assert registry.get_pii_fields(1) == ["email"]

    def test_raises_for_unknown_type_id(self):
        registry = SchemaRegistry()
        with pytest.raises(KeyError):
            registry.get_pii_fields(999)


# ---------------------------------------------------------------------------
# Registry: get_subject_field
# ---------------------------------------------------------------------------


class TestGetSubjectField:
    """SchemaRegistry.get_subject_field behaviour."""

    def test_returns_subject_field(self):
        """subject_field is returned when set."""
        registry = SchemaRegistry()
        Order = NodeTypeDef(
            type_id=10,
            name="Order",
            fields=(field(1, "user_id", "str"),),
            subject_field="user_id",
            data_policy=DataPolicy.FINANCIAL,
            legal_basis="contract",
        )
        registry.register_node_type(Order)

        assert registry.get_subject_field(10) == "user_id"

    def test_returns_none_when_unset(self):
        """None returned when subject_field is not set."""
        registry = SchemaRegistry()
        Log = NodeTypeDef(type_id=20, name="Log")
        registry.register_node_type(Log)

        assert registry.get_subject_field(20) is None

    def test_raises_for_unknown_type_id(self):
        registry = SchemaRegistry()
        with pytest.raises(KeyError):
            registry.get_subject_field(999)


# ---------------------------------------------------------------------------
# NodeTypeDef serialization round-trip
# ---------------------------------------------------------------------------


class TestNodeTypeDefSerialization:
    """data_policy, subject_field, legal_basis survive to_dict/from_dict."""

    def test_round_trip_with_all_fields(self):
        original = NodeTypeDef(
            type_id=5,
            name="Patient",
            fields=(
                field(1, "name", "str", pii=True),
                field(2, "dob", "str", pii=True),
            ),
            data_policy=DataPolicy.HEALTHCARE,
            subject_field="name",
            legal_basis="consent",
        )
        d = original.to_dict()
        restored = NodeTypeDef.from_dict(d)

        assert restored.data_policy == DataPolicy.HEALTHCARE
        assert restored.subject_field == "name"
        assert restored.legal_basis == "consent"
        assert restored.fields[0].pii is True
        assert restored.fields[1].pii is True

    def test_round_trip_without_optional_fields(self):
        """Omitted data_policy / subject_field stay None after round-trip."""
        original = NodeTypeDef(type_id=6, name="Metric")
        d = original.to_dict()
        restored = NodeTypeDef.from_dict(d)

        assert restored.data_policy is None
        assert restored.subject_field is None
        assert restored.legal_basis is None


# ---------------------------------------------------------------------------
# type_metadata table (canonical store integration)
# ---------------------------------------------------------------------------


class TestTypeMetadataTable:
    """sync_type_metadata populates the SQLite type_metadata table."""

    @pytest.fixture()
    def store(self, tmp_path):
        """Create a CanonicalStore with an initialized tenant."""
        store = CanonicalStore(str(tmp_path), wal_mode=False)
        # Synchronously initialize the tenant database
        with store._get_connection("t1", create=True) as conn:
            store._create_schema(conn)
        return store

    def _query_metadata(self, store, type_id):
        """Helper to query type_metadata table."""
        import sqlite3

        db_path = Path(store.data_dir) / "tenant_t1.db"
        conn = sqlite3.connect(str(db_path))
        conn.row_factory = sqlite3.Row
        row = conn.execute(
            "SELECT * FROM type_metadata WHERE type_id = ?", (type_id,)
        ).fetchone()
        conn.close()
        return row

    def test_table_populated_on_sync(self, store):
        registry = SchemaRegistry()
        User = NodeTypeDef(
            type_id=1,
            name="User",
            fields=(field(1, "email", "str", pii=True),),
            data_policy=DataPolicy.PERSONAL,
            subject_field="email",
        )
        registry.register_node_type(User)

        store.sync_type_metadata("t1", registry)

        row = self._query_metadata(store, 1)
        assert row is not None
        assert row["data_policy"] == "personal"
        assert json.loads(row["pii_fields"]) == ["email"]
        assert row["subject_field"] == "email"

    def test_table_updates_on_re_sync(self, store):
        """Re-syncing overwrites stale rows."""
        registry = SchemaRegistry()
        User = NodeTypeDef(
            type_id=1,
            name="User",
            data_policy=DataPolicy.BUSINESS,
        )
        registry.register_node_type(User)
        store.sync_type_metadata("t1", registry)

        # Build a new registry with updated policy
        registry2 = SchemaRegistry()
        User2 = NodeTypeDef(
            type_id=1,
            name="User",
            data_policy=DataPolicy.PERSONAL,
        )
        registry2.register_node_type(User2)
        store.sync_type_metadata("t1", registry2)

        row = self._query_metadata(store, 1)
        assert row["data_policy"] == "personal"

    def test_default_policy_stored_when_unset(self, store, caplog):
        """When data_policy is None the table stores 'personal'."""
        registry = SchemaRegistry()
        Anon = NodeTypeDef(type_id=99, name="Anon")
        registry.register_node_type(Anon)

        with caplog.at_level(logging.WARNING):
            store.sync_type_metadata("t1", registry)

        row = self._query_metadata(store, 99)
        assert row["data_policy"] == "personal"


# ---------------------------------------------------------------------------
# Warning for unclassified types
# ---------------------------------------------------------------------------


class TestWarningForUnclassifiedTypes:
    """A warning is logged when a type has no data_policy."""

    def test_warning_logged_on_get_data_policy(self, caplog):
        registry = SchemaRegistry()
        Widget = NodeTypeDef(type_id=50, name="Widget")
        registry.register_node_type(Widget)

        with caplog.at_level(logging.WARNING, logger="dbaas.entdb_server.schema.registry"):
            policy = registry.get_data_policy(50)

        assert policy == DataPolicy.PERSONAL
        assert any("no data_policy set" in m for m in caplog.messages)
        assert any("Widget" in m for m in caplog.messages)
