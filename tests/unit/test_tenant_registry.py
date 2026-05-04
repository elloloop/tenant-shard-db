"""
Unit tests for tenant registry CRUD and membership management.

Tests cover:
- GlobalStore: tenant CRUD and membership operations
- gRPC handlers: CreateTenant, GetTenant, ArchiveTenant
- gRPC handlers: AddTenantMember, RemoveTenantMember, GetTenantMembers
- gRPC handlers: GetUserTenants, ChangeMemberRole
- Permission enforcement: owner/admin add, last owner guard, role changes
"""

import sqlite3
import tempfile
from unittest.mock import AsyncMock, MagicMock

import grpc
import pytest

from dbaas.entdb_server.global_store import GlobalStore

# -- GlobalStore unit tests ------------------------------------------------


class TestGlobalStoreTenantCRUD:
    """Tests for tenant registry operations in GlobalStore."""

    @pytest.fixture
    def store(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            s = GlobalStore(tmpdir)
            yield s
            s.close()

    async def test_create_tenant(self, store):
        """Create a tenant and verify returned fields."""
        tenant = await store.create_tenant("t1", "Acme Corp")
        assert tenant["tenant_id"] == "t1"
        assert tenant["name"] == "Acme Corp"
        assert tenant["status"] == "active"
        assert isinstance(tenant["created_at"], int)

    async def test_create_duplicate_tenant(self, store):
        """Duplicate tenant_id raises IntegrityError."""
        await store.create_tenant("t1", "Acme")
        with pytest.raises(sqlite3.IntegrityError):
            await store.create_tenant("t1", "Other")

    async def test_get_tenant(self, store):
        """Fetch a tenant by ID."""
        await store.create_tenant("t1", "Acme")
        tenant = await store.get_tenant("t1")
        assert tenant is not None
        assert tenant["tenant_id"] == "t1"
        assert tenant["name"] == "Acme"

    async def test_get_tenant_not_found(self, store):
        """Fetching a non-existent tenant returns None."""
        assert await store.get_tenant("nonexistent") is None

    async def test_set_tenant_status(self, store):
        """Change tenant status to archived."""
        await store.create_tenant("t1", "Acme")
        result = await store.set_tenant_status("t1", "archived")
        assert result is True
        tenant = await store.get_tenant("t1")
        assert tenant["status"] == "archived"

    async def test_list_tenants_filtered(self, store):
        """List only active tenants."""
        await store.create_tenant("t1", "A")
        await store.create_tenant("t2", "B")
        await store.set_tenant_status("t2", "archived")
        active = await store.list_tenants(status="active")
        assert len(active) == 1
        assert active[0]["tenant_id"] == "t1"


class TestGlobalStoreTenantRegion:
    """Tests for per-tenant region pinning in the tenant registry."""

    @pytest.fixture
    def store(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            s = GlobalStore(tmpdir)
            yield s
            s.close()

    async def test_create_tenant_default_region(self, store):
        """A tenant created without specifying region defaults to us-east-1."""
        tenant = await store.create_tenant("t1", "Acme")
        assert tenant["region"] == "us-east-1"

    async def test_create_tenant_with_region(self, store):
        """create_tenant accepts an explicit region kwarg and persists it."""
        tenant = await store.create_tenant("t-eu", "Acme EU", region="eu-west-1")
        assert tenant["region"] == "eu-west-1"

    async def test_get_tenant_returns_region(self, store):
        """get_tenant surfaces the persisted region."""
        await store.create_tenant("t-ap", "Acme APAC", region="ap-southeast-1")
        fetched = await store.get_tenant("t-ap")
        assert fetched is not None
        assert fetched["region"] == "ap-southeast-1"

    async def test_list_tenants_returns_region(self, store):
        """list_tenants surfaces region for every row."""
        await store.create_tenant("t-us", "US", region="us-east-1")
        await store.create_tenant("t-eu", "EU", region="eu-west-1")
        listed = {t["tenant_id"]: t["region"] for t in await store.list_tenants()}
        assert listed == {"t-us": "us-east-1", "t-eu": "eu-west-1"}

    async def test_alter_table_migration_for_pre_region_db(self, tmp_path):
        """A tenant_registry created before the region column exists gains it on open.

        This guards against in-place upgrades: existing deployments must not
        crash when the new GlobalStore schema runs against an old DB file.
        """
        # Hand-create a pre-region tenant_registry (no region column).
        db_path = tmp_path / "global.db"
        with sqlite3.connect(db_path) as conn:
            conn.execute(
                """
                CREATE TABLE tenant_registry (
                    tenant_id   TEXT PRIMARY KEY,
                    name        TEXT NOT NULL,
                    status      TEXT NOT NULL DEFAULT 'active',
                    created_at  INTEGER NOT NULL
                )
                """
            )
            conn.execute(
                "INSERT INTO tenant_registry (tenant_id, name, status, created_at) "
                "VALUES ('legacy', 'Legacy Co', 'active', 1)"
            )
            conn.commit()

        # Opening with the new schema must add the column without losing data.
        store = GlobalStore(str(tmp_path))
        try:
            tenant = await store.get_tenant("legacy")
            assert tenant is not None
            assert tenant["name"] == "Legacy Co"
            assert tenant["region"] == "us-east-1"  # backfilled default
        finally:
            store.close()


class TestGlobalStoreMembership:
    """Tests for tenant membership operations in GlobalStore."""

    @pytest.fixture
    def store(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            s = GlobalStore(tmpdir)
            yield s
            s.close()

    async def test_add_and_get_members(self, store):
        """Add members and list them."""
        await store.add_member("t1", "u1", role="owner")
        await store.add_member("t1", "u2", role="member")
        members = await store.get_members("t1")
        assert len(members) == 2
        roles = {m["user_id"]: m["role"] for m in members}
        assert roles == {"u1": "owner", "u2": "member"}

    async def test_remove_member(self, store):
        """Remove a member from a tenant."""
        await store.add_member("t1", "u1")
        removed = await store.remove_member("t1", "u1")
        assert removed is True
        assert await store.get_members("t1") == []

    async def test_get_user_tenants(self, store):
        """List all tenants a user belongs to."""
        await store.add_member("t1", "u1", role="owner")
        await store.add_member("t2", "u1", role="member")
        tenants = await store.get_user_tenants("u1")
        assert len(tenants) == 2
        tenant_ids = {t["tenant_id"] for t in tenants}
        assert tenant_ids == {"t1", "t2"}

    async def test_change_role(self, store):
        """Change a member's role."""
        await store.add_member("t1", "u1", role="member")
        result = await store.change_role("t1", "u1", "admin")
        assert result is True
        members = await store.get_members("t1")
        assert members[0]["role"] == "admin"

    async def test_change_role_not_found(self, store):
        """Changing role for non-existent member returns False."""
        result = await store.change_role("t1", "ghost", "admin")
        assert result is False


# -- gRPC handler tests ----------------------------------------------------


def _make_servicer(global_store=None):
    """Build an EntDBServicer with mocked dependencies and a real GlobalStore."""
    from dbaas.entdb_server.api.grpc_server import EntDBServicer

    wal = MagicMock()
    canonical_store = MagicMock()
    canonical_store.initialize_tenant = AsyncMock()
    schema_registry = MagicMock()
    schema_registry.fingerprint = "test-fp"

    servicer = EntDBServicer(
        wal=wal,
        canonical_store=canonical_store,
        schema_registry=schema_registry,
        global_store=global_store,
    )
    return servicer


class _AbortError(BaseException):
    """Raised by _FakeContext.abort to break out of handler logic.

    Inherits from BaseException (not Exception) so that it passes through
    generic 'except Exception' blocks in the gRPC handler code, matching
    the behavior of grpc.aio.AbortError in a real gRPC server.
    """

    pass


class _FakeContext:
    """Minimal mock for grpc_aio.ServicerContext."""

    def __init__(self):
        self._aborted = False
        self._abort_code = None
        self._abort_message = None

    async def abort(self, code, message):
        self._aborted = True
        self._abort_code = code
        self._abort_message = message
        raise _AbortError(f"[{code}] {message}")


class TestCreateTenantHandler:
    """Tests for CreateTenant gRPC handler."""

    @pytest.fixture
    def global_store(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            s = GlobalStore(tmpdir)
            yield s
            s.close()

    async def test_create_tenant_success(self, global_store):
        """CreateTenant creates tenant, initializes SQLite, adds creator as owner."""
        from dbaas.entdb_server.api.generated import CreateTenantRequest

        servicer = _make_servicer(global_store)
        ctx = _FakeContext()
        request = CreateTenantRequest(actor="user:alice", tenant_id="t1", name="Acme Corp")

        response = await servicer.CreateTenant(request, ctx)

        assert response.success is True
        assert response.tenant.tenant_id == "t1"
        assert response.tenant.name == "Acme Corp"
        assert response.tenant.status == "active"

        # Verify initialize_tenant was called
        servicer.canonical_store.initialize_tenant.assert_awaited_once_with("t1")

        # Verify creator is owner
        members = await global_store.get_members("t1")
        assert len(members) == 1
        assert members[0]["user_id"] == "alice"
        assert members[0]["role"] == "owner"

    async def test_create_duplicate_tenant(self, global_store):
        """CreateTenant returns error for duplicate tenant_id."""
        from dbaas.entdb_server.api.generated import CreateTenantRequest

        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        req = CreateTenantRequest(actor="user:alice", tenant_id="t1", name="Acme")
        await servicer.CreateTenant(req, ctx)

        # Second create should fail gracefully
        ctx2 = _FakeContext()
        response = await servicer.CreateTenant(req, ctx2)
        assert response.success is False
        assert "already exists" in response.error


class TestGetTenantHandler:
    """Tests for GetTenant gRPC handler."""

    @pytest.fixture
    def global_store(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            s = GlobalStore(tmpdir)
            yield s
            s.close()

    async def test_get_tenant_found(self, global_store):
        """GetTenant returns tenant details."""
        from dbaas.entdb_server.api.generated import GetTenantRequest

        await global_store.create_tenant("t1", "Acme Corp")
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        response = await servicer.GetTenant(
            GetTenantRequest(actor="user:alice", tenant_id="t1"), ctx
        )

        assert response.found is True
        assert response.tenant.tenant_id == "t1"
        assert response.tenant.name == "Acme Corp"

    async def test_get_tenant_not_found(self, global_store):
        """GetTenant returns found=False for missing tenant."""
        from dbaas.entdb_server.api.generated import GetTenantRequest

        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        response = await servicer.GetTenant(
            GetTenantRequest(actor="user:alice", tenant_id="nonexistent"), ctx
        )

        assert response.found is False


class TestArchiveTenantHandler:
    """Tests for ArchiveTenant gRPC handler."""

    @pytest.fixture
    def global_store(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            s = GlobalStore(tmpdir)
            yield s
            s.close()

    async def test_archive_by_owner(self, global_store):
        """Owner can archive a tenant."""
        from dbaas.entdb_server.api.generated import ArchiveTenantRequest

        await global_store.create_tenant("t1", "Acme")
        await global_store.add_member("t1", "alice", role="owner")
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        response = await servicer.ArchiveTenant(
            ArchiveTenantRequest(actor="user:alice", tenant_id="t1"), ctx
        )

        assert response.success is True
        tenant = await global_store.get_tenant("t1")
        assert tenant["status"] == "archived"

    async def test_archive_by_member_denied(self, global_store):
        """Regular member cannot archive a tenant."""
        from dbaas.entdb_server.api.generated import ArchiveTenantRequest

        await global_store.create_tenant("t1", "Acme")
        await global_store.add_member("t1", "bob", role="member")
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        with pytest.raises(_AbortError):
            await servicer.ArchiveTenant(
                ArchiveTenantRequest(actor="user:bob", tenant_id="t1"), ctx
            )
        assert ctx._abort_code == grpc.StatusCode.PERMISSION_DENIED

    async def test_archive_by_system(self, global_store):
        """System actor can archive a tenant."""
        from dbaas.entdb_server.api.generated import ArchiveTenantRequest

        await global_store.create_tenant("t1", "Acme")
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        response = await servicer.ArchiveTenant(
            ArchiveTenantRequest(actor="system:admin", tenant_id="t1"), ctx
        )

        assert response.success is True


class TestAddTenantMemberHandler:
    """Tests for AddTenantMember gRPC handler."""

    @pytest.fixture
    def global_store(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            s = GlobalStore(tmpdir)
            yield s
            s.close()

    async def test_add_member_by_owner(self, global_store):
        """Owner can add a new member."""
        from dbaas.entdb_server.api.generated import TenantMemberRequest

        await global_store.add_member("t1", "alice", role="owner")
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        response = await servicer.AddTenantMember(
            TenantMemberRequest(actor="user:alice", tenant_id="t1", user_id="bob", role="member"),
            ctx,
        )

        assert response.success is True
        members = await global_store.get_members("t1")
        assert len(members) == 2

    async def test_add_member_by_admin(self, global_store):
        """Admin can add a new member."""
        from dbaas.entdb_server.api.generated import TenantMemberRequest

        await global_store.add_member("t1", "alice", role="admin")
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        response = await servicer.AddTenantMember(
            TenantMemberRequest(actor="user:alice", tenant_id="t1", user_id="carol"),
            ctx,
        )

        assert response.success is True

    async def test_add_member_by_regular_member_denied(self, global_store):
        """Regular member cannot add members."""
        from dbaas.entdb_server.api.generated import TenantMemberRequest

        await global_store.add_member("t1", "alice", role="member")
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        with pytest.raises(_AbortError):
            await servicer.AddTenantMember(
                TenantMemberRequest(actor="user:alice", tenant_id="t1", user_id="bob"),
                ctx,
            )
        assert ctx._abort_code == grpc.StatusCode.PERMISSION_DENIED


class TestRemoveTenantMemberHandler:
    """Tests for RemoveTenantMember gRPC handler."""

    @pytest.fixture
    def global_store(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            s = GlobalStore(tmpdir)
            yield s
            s.close()

    async def test_remove_member_success(self, global_store):
        """Remove a member from a tenant."""
        from dbaas.entdb_server.api.generated import TenantMemberRequest

        await global_store.add_member("t1", "alice", role="owner")
        await global_store.add_member("t1", "bob", role="member")
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        response = await servicer.RemoveTenantMember(
            TenantMemberRequest(actor="system:admin", tenant_id="t1", user_id="bob"),
            ctx,
        )

        assert response.success is True
        members = await global_store.get_members("t1")
        assert len(members) == 1
        assert members[0]["user_id"] == "alice"

    async def test_remove_last_owner_denied(self, global_store):
        """Cannot remove the last owner of a tenant."""
        from dbaas.entdb_server.api.generated import TenantMemberRequest

        await global_store.add_member("t1", "alice", role="owner")
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        response = await servicer.RemoveTenantMember(
            TenantMemberRequest(actor="system:admin", tenant_id="t1", user_id="alice"),
            ctx,
        )

        assert response.success is False
        assert "last owner" in response.error

    async def test_remove_owner_when_multiple_owners(self, global_store):
        """Can remove an owner when another owner exists."""
        from dbaas.entdb_server.api.generated import TenantMemberRequest

        await global_store.add_member("t1", "alice", role="owner")
        await global_store.add_member("t1", "bob", role="owner")
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        response = await servicer.RemoveTenantMember(
            TenantMemberRequest(actor="system:admin", tenant_id="t1", user_id="alice"),
            ctx,
        )

        assert response.success is True

    async def test_remove_nonexistent_member(self, global_store):
        """Removing a non-existent member returns error."""
        from dbaas.entdb_server.api.generated import TenantMemberRequest

        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        response = await servicer.RemoveTenantMember(
            TenantMemberRequest(actor="system:admin", tenant_id="t1", user_id="ghost"),
            ctx,
        )

        assert response.success is False
        assert "not found" in response.error.lower()


class TestGetTenantMembersHandler:
    """Tests for GetTenantMembers gRPC handler."""

    @pytest.fixture
    def global_store(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            s = GlobalStore(tmpdir)
            yield s
            s.close()

    async def test_get_members(self, global_store):
        """List all members of a tenant."""
        from dbaas.entdb_server.api.generated import GetTenantMembersRequest

        await global_store.add_member("t1", "alice", role="owner")
        await global_store.add_member("t1", "bob", role="member")
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        response = await servicer.GetTenantMembers(
            GetTenantMembersRequest(actor="user:alice", tenant_id="t1"), ctx
        )

        assert len(response.members) == 2
        user_ids = {m.user_id for m in response.members}
        assert user_ids == {"alice", "bob"}


class TestGetUserTenantsHandler:
    """Tests for GetUserTenants gRPC handler."""

    @pytest.fixture
    def global_store(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            s = GlobalStore(tmpdir)
            yield s
            s.close()

    async def test_get_user_tenants(self, global_store):
        """List all tenants a user belongs to."""
        from dbaas.entdb_server.api.generated import GetUserTenantsRequest

        await global_store.add_member("t1", "alice", role="owner")
        await global_store.add_member("t2", "alice", role="member")
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        response = await servicer.GetUserTenants(
            GetUserTenantsRequest(actor="user:alice", user_id="alice"), ctx
        )

        assert len(response.memberships) == 2
        tenant_ids = {m.tenant_id for m in response.memberships}
        assert tenant_ids == {"t1", "t2"}


class TestChangeMemberRoleHandler:
    """Tests for ChangeMemberRole gRPC handler."""

    @pytest.fixture
    def global_store(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            s = GlobalStore(tmpdir)
            yield s
            s.close()

    async def test_change_role_by_owner(self, global_store):
        """Owner can change a member's role."""
        from dbaas.entdb_server.api.generated import ChangeMemberRoleRequest

        await global_store.add_member("t1", "alice", role="owner")
        await global_store.add_member("t1", "bob", role="member")
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        response = await servicer.ChangeMemberRole(
            ChangeMemberRoleRequest(
                actor="user:alice", tenant_id="t1", user_id="bob", new_role="admin"
            ),
            ctx,
        )

        assert response.success is True
        members = await global_store.get_members("t1")
        bob = [m for m in members if m["user_id"] == "bob"][0]
        assert bob["role"] == "admin"

    async def test_change_role_by_member_denied(self, global_store):
        """Regular member cannot change roles."""
        from dbaas.entdb_server.api.generated import ChangeMemberRoleRequest

        await global_store.add_member("t1", "alice", role="member")
        await global_store.add_member("t1", "bob", role="member")
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        with pytest.raises(_AbortError):
            await servicer.ChangeMemberRole(
                ChangeMemberRoleRequest(
                    actor="user:alice", tenant_id="t1", user_id="bob", new_role="admin"
                ),
                ctx,
            )
        assert ctx._abort_code == grpc.StatusCode.PERMISSION_DENIED

    async def test_change_role_by_admin_denied(self, global_store):
        """Admin cannot change roles -- only owner can."""
        from dbaas.entdb_server.api.generated import ChangeMemberRoleRequest

        await global_store.add_member("t1", "alice", role="admin")
        await global_store.add_member("t1", "bob", role="member")
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        with pytest.raises(_AbortError):
            await servicer.ChangeMemberRole(
                ChangeMemberRoleRequest(
                    actor="user:alice", tenant_id="t1", user_id="bob", new_role="admin"
                ),
                ctx,
            )
        assert ctx._abort_code == grpc.StatusCode.PERMISSION_DENIED

    async def test_change_role_nonexistent_member(self, global_store):
        """Changing role for non-existent member returns error."""
        from dbaas.entdb_server.api.generated import ChangeMemberRoleRequest

        await global_store.add_member("t1", "alice", role="owner")
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        response = await servicer.ChangeMemberRole(
            ChangeMemberRoleRequest(
                actor="user:alice", tenant_id="t1", user_id="ghost", new_role="admin"
            ),
            ctx,
        )

        assert response.success is False
        assert "not found" in response.error.lower()

    async def test_change_role_by_system(self, global_store):
        """System actor can change roles."""
        from dbaas.entdb_server.api.generated import ChangeMemberRoleRequest

        await global_store.add_member("t1", "bob", role="member")
        servicer = _make_servicer(global_store)
        ctx = _FakeContext()

        response = await servicer.ChangeMemberRole(
            ChangeMemberRoleRequest(
                actor="system:admin", tenant_id="t1", user_id="bob", new_role="owner"
            ),
            ctx,
        )

        assert response.success is True


# -- SDK admin surface: create_tenant region pinning -----------------------
# These tests exercise the public SDK wrappers for create_tenant /
# get_tenant — verifying that the new ``region`` kwarg is forwarded
# to the wire CreateTenantRequest and that the resolved region from
# TenantDetail is returned to callers. Wire shape coverage is the
# point: a future proto edit that moves the field tag will break
# them.


class TestSDKCreateTenantRegion:
    """SDK-side tests for region pinning on create_tenant."""

    async def test_grpc_client_create_tenant_forwards_region(self) -> None:
        """``GrpcClient.create_tenant`` writes ``region`` onto the
        wire request and surfaces the resolved region back."""
        from sdk.entdb_sdk._generated import (
            CreateTenantResponse,
            TenantDetail,
        )
        from sdk.entdb_sdk._grpc_client import GrpcClient

        c = GrpcClient("localhost", 50051)
        # Stand up a fake stub so we can inspect the request that
        # _retry was given. _ensure_connected just unwraps _stub.
        c._stub = MagicMock()
        captured: dict[str, object] = {}

        async def fake_retry(method, request, **_kwargs):
            captured["request"] = request
            return CreateTenantResponse(
                success=True,
                tenant=TenantDetail(
                    tenant_id="acme-eu",
                    name="Acme EU",
                    status="active",
                    region="eu-west-1",
                    created_at=1_700_000_000_000,
                ),
            )

        c._retry = fake_retry  # type: ignore[method-assign]

        result = await c.create_tenant(
            actor="system:admin",
            tenant_id="acme-eu",
            name="Acme EU",
            region="eu-west-1",
        )

        # Wire request carries the region.
        assert captured["request"].region == "eu-west-1"  # type: ignore[attr-defined]
        # Tenant dict surfaces the resolved region.
        assert result["success"] is True
        assert result["tenant"]["region"] == "eu-west-1"

    async def test_grpc_client_create_tenant_no_region_sends_empty(self) -> None:
        """Omitting ``region`` sends an empty string on the wire so the
        server can default to its own served region."""
        from sdk.entdb_sdk._generated import CreateTenantResponse, TenantDetail
        from sdk.entdb_sdk._grpc_client import GrpcClient

        c = GrpcClient("localhost", 50051)
        c._stub = MagicMock()
        captured: dict[str, object] = {}

        async def fake_retry(method, request, **_kwargs):
            captured["request"] = request
            return CreateTenantResponse(
                success=True,
                tenant=TenantDetail(
                    tenant_id="acme",
                    name="Acme",
                    status="active",
                    region="us-east-1",  # server defaulted
                ),
            )

        c._retry = fake_retry  # type: ignore[method-assign]

        result = await c.create_tenant(actor="system:admin", tenant_id="acme", name="Acme")

        assert captured["request"].region == ""  # type: ignore[attr-defined]
        # Even though caller didn't set it, the server's default is
        # surfaced back in the tenant dict.
        assert result["tenant"]["region"] == "us-east-1"

    async def test_dbclient_create_tenant_passes_region_kwarg(self) -> None:
        """The public ``DbClient.create_tenant`` wrapper forwards the
        ``region`` kwarg to the underlying gRPC client."""
        from sdk.entdb_sdk.client import DbClient

        c = DbClient("localhost:50051")
        c._connected = True
        c._grpc = AsyncMock()
        c._grpc.create_tenant = AsyncMock(
            return_value={
                "success": True,
                "error": None,
                "tenant": {
                    "tenant_id": "acme-eu",
                    "name": "Acme EU",
                    "status": "active",
                    "region": "eu-west-1",
                    "created_at": 0,
                },
            }
        )

        result = await c.create_tenant("acme-eu", "Acme EU", region="eu-west-1")

        c._grpc.create_tenant.assert_awaited_once()
        kwargs = c._grpc.create_tenant.await_args.kwargs
        assert kwargs["region"] == "eu-west-1"
        assert kwargs["tenant_id"] == "acme-eu"
        assert result["tenant"]["region"] == "eu-west-1"

    async def test_grpc_client_get_tenant_returns_region(self) -> None:
        """``get_tenant`` surfaces the persisted region back to callers."""
        from sdk.entdb_sdk._generated import GetTenantResponse, TenantDetail
        from sdk.entdb_sdk._grpc_client import GrpcClient

        c = GrpcClient("localhost", 50051)
        c._stub = MagicMock()

        async def fake_retry(method, request, **_kwargs):
            return GetTenantResponse(
                found=True,
                tenant=TenantDetail(
                    tenant_id="acme-eu",
                    name="Acme EU",
                    status="active",
                    region="eu-west-1",
                ),
            )

        c._retry = fake_retry  # type: ignore[method-assign]

        result = await c.get_tenant(actor="system:admin", tenant_id="acme-eu")
        assert result is not None
        assert result["region"] == "eu-west-1"
