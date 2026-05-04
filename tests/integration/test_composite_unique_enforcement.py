"""Integration tests for composite (multi-field) unique enforcement.

These tests run the real applier + canonical_store stack on a
temporary SQLite database to prove that
``(entdb.node).composite_unique`` is enforced atomically. The
race-condition test in particular validates the original motivation:
two concurrent ``create_node`` calls with the same composite-unique
tuple — exactly one wins, one returns a typed
``CompositeUniqueConstraintError``. A Find-then-Create flow alone
cannot guarantee that.
"""

from __future__ import annotations

import asyncio

import pytest

from dbaas.entdb_server.apply.applier import (
    Applier,
    CompositeUniqueConstraintError,
    MailboxFanoutConfig,
    TransactionEvent,
    UniqueConstraintError,
)
from dbaas.entdb_server.apply.canonical_store import CanonicalStore
from dbaas.entdb_server.schema.registry import (
    get_registry,
    reset_registry,
)
from dbaas.entdb_server.schema.types import (
    CompositeUniqueDef,
    NodeTypeDef,
    field,
)
from dbaas.entdb_server.wal.memory import InMemoryWalStream

TENANT = "tenant-cu-tests"
ALICE = "user:alice"

TYPE_OAUTH = 301
F_PROVIDER = 1
F_PROVIDER_USER_ID = 2
F_USER_ID = 3


@pytest.fixture(autouse=True)
def registered_schema():
    """Register an ``OAuthIdentity`` type with a (provider,
    provider_user_id) composite unique constraint for every test in
    this module. The autouse + reset_registry pattern matches
    ``tests/unit/test_unique_keys.py``.
    """
    reset_registry()
    reg = get_registry()
    nt = NodeTypeDef(
        type_id=TYPE_OAUTH,
        name="OAuthIdentity",
        fields=(
            field(F_PROVIDER, "provider", "str"),
            field(F_PROVIDER_USER_ID, "provider_user_id", "str"),
            field(F_USER_ID, "user_id", "str"),
        ),
        composite_unique=(
            CompositeUniqueDef(
                name="provider_user_id",
                field_ids=(F_PROVIDER, F_PROVIDER_USER_ID),
            ),
        ),
    )
    reg.register_node_type(nt)
    yield reg
    reset_registry()


@pytest.fixture
def store(tmp_path):
    s = CanonicalStore(data_dir=str(tmp_path))
    with s._get_connection(TENANT, create=True) as conn:
        s._create_schema(conn)
    return s


async def _make_applier(store: CanonicalStore) -> Applier:
    wal = InMemoryWalStream(num_partitions=1)
    await wal.connect()
    return Applier(
        wal=wal,
        canonical_store=store,
        topic="t",
        fanout_config=MailboxFanoutConfig(enabled=False),
    )


def _create_event(
    idempotency_key: str,
    *,
    node_id: str,
    payload: dict,
) -> TransactionEvent:
    return TransactionEvent.from_dict(
        {
            "tenant_id": TENANT,
            "actor": ALICE,
            "idempotency_key": idempotency_key,
            "ops": [
                {
                    "op": "create_node",
                    "type_id": TYPE_OAUTH,
                    "id": node_id,
                    "data": payload,
                }
            ],
        }
    )


class TestCompositeUniqueEnforcement:
    @pytest.mark.asyncio
    async def test_first_create_succeeds(self, store):
        applier = await _make_applier(store)
        ev = _create_event(
            "ev-1",
            node_id="ident-1",
            payload={
                str(F_PROVIDER): "google",
                str(F_PROVIDER_USER_ID): "uid-123",
                str(F_USER_ID): "alice",
            },
        )
        result = await applier.apply_event(ev)
        assert result.success, result.error

    @pytest.mark.asyncio
    async def test_duplicate_tuple_rejected(self, store):
        """Second create with the same (provider, provider_user_id)
        tuple but a different ``user_id`` must be rejected.
        """
        applier = await _make_applier(store)
        await applier.apply_event(
            _create_event(
                "ev-a",
                node_id="ident-a",
                payload={
                    str(F_PROVIDER): "google",
                    str(F_PROVIDER_USER_ID): "uid-123",
                    str(F_USER_ID): "alice",
                },
            )
        )
        result = await applier.apply_event(
            _create_event(
                "ev-b",
                node_id="ident-b",
                payload={
                    # Same composite tuple, different non-unique field.
                    str(F_PROVIDER): "google",
                    str(F_PROVIDER_USER_ID): "uid-123",
                    str(F_USER_ID): "bob",
                },
            )
        )
        assert not result.success
        # Server message comes from CompositeUniqueConstraintError.
        assert "composite unique" in (result.error or "").lower()

    @pytest.mark.asyncio
    async def test_partial_overlap_allowed(self, store):
        """Same provider but different provider_user_id must succeed —
        composite uniqueness is on the *tuple*, not on either field.
        """
        applier = await _make_applier(store)
        r1 = await applier.apply_event(
            _create_event(
                "ev-a",
                node_id="ident-a",
                payload={
                    str(F_PROVIDER): "google",
                    str(F_PROVIDER_USER_ID): "uid-1",
                    str(F_USER_ID): "alice",
                },
            )
        )
        r2 = await applier.apply_event(
            _create_event(
                "ev-b",
                node_id="ident-b",
                payload={
                    str(F_PROVIDER): "google",
                    str(F_PROVIDER_USER_ID): "uid-2",  # different
                    str(F_USER_ID): "bob",
                },
            )
        )
        assert r1.success
        assert r2.success

    @pytest.mark.asyncio
    async def test_typed_error_carries_constraint_metadata(self, store):
        """Apply-time enforcement raises ``CompositeUniqueConstraintError``
        with the constraint name and colliding tuple. The error must
        also be an instance of ``UniqueConstraintError`` so legacy
        ``except`` blocks keep working.
        """
        applier = await _make_applier(store)
        await applier.apply_event(
            _create_event(
                "ev-a",
                node_id="ident-a",
                payload={
                    str(F_PROVIDER): "github",
                    str(F_PROVIDER_USER_ID): "gh-1",
                    str(F_USER_ID): "alice",
                },
            )
        )
        # Drive ``_sync_apply_event_body`` directly so the typed
        # exception escapes uncaught (``apply_event`` swallows it
        # into ``ApplyResult.error``). This mirrors how the WAL
        # consumer's apply path will see the error.
        ev = _create_event(
            "ev-b",
            node_id="ident-b",
            payload={
                str(F_PROVIDER): "github",
                str(F_PROVIDER_USER_ID): "gh-1",
                str(F_USER_ID): "bob",
            },
        )
        with pytest.raises(CompositeUniqueConstraintError) as exc_info:
            applier._sync_apply_event_body(ev)
        err = exc_info.value
        assert isinstance(err, UniqueConstraintError)
        assert err.tenant_id == TENANT
        assert err.type_id == TYPE_OAUTH
        assert err.constraint_name == "provider_user_id"
        assert err.field_ids == (F_PROVIDER, F_PROVIDER_USER_ID)
        assert err.values == ("github", "gh-1")

    @pytest.mark.asyncio
    async def test_concurrent_creates_exactly_one_wins(self, store):
        """Race condition: kick off two concurrent applies with the
        same composite-unique tuple. Exactly one must succeed; the
        other must fail with the typed error. This is the original
        motivation for composite_unique — Find-then-Create alone
        cannot guarantee this.
        """
        applier = await _make_applier(store)

        async def attempt(idempotency_key: str, node_id: str):
            ev = _create_event(
                idempotency_key,
                node_id=node_id,
                payload={
                    str(F_PROVIDER): "okta",
                    str(F_PROVIDER_USER_ID): "okta-42",
                    str(F_USER_ID): node_id,
                },
            )
            return await applier.apply_event(ev)

        results = await asyncio.gather(
            attempt("ev-r1", "ident-r1"),
            attempt("ev-r2", "ident-r2"),
        )
        wins = [r for r in results if r.success]
        losses = [r for r in results if not r.success]
        assert len(wins) == 1, f"expected 1 winner, got {len(wins)}: {results}"
        assert len(losses) == 1
        assert "composite unique" in (losses[0].error or "").lower()

        # Loser must not have left a half-written row.
        winner_id = "ident-r1" if wins[0].created_nodes == ["ident-r1"] else "ident-r2"
        loser_id = "ident-r2" if winner_id == "ident-r1" else "ident-r1"
        winner_node = await store.get_node(TENANT, winner_id)
        loser_node = await store.get_node(TENANT, loser_id)
        assert winner_node is not None
        assert loser_node is None

    @pytest.mark.asyncio
    async def test_update_to_colliding_tuple_rejected(self, store):
        """``update_node`` that would create a duplicate tuple is
        rejected by the same unique expression index.
        """
        applier = await _make_applier(store)
        await applier.apply_event(
            _create_event(
                "ev-a",
                node_id="ident-a",
                payload={
                    str(F_PROVIDER): "google",
                    str(F_PROVIDER_USER_ID): "uid-1",
                    str(F_USER_ID): "alice",
                },
            )
        )
        await applier.apply_event(
            _create_event(
                "ev-b",
                node_id="ident-b",
                payload={
                    str(F_PROVIDER): "google",
                    str(F_PROVIDER_USER_ID): "uid-2",
                    str(F_USER_ID): "bob",
                },
            )
        )
        update = TransactionEvent.from_dict(
            {
                "tenant_id": TENANT,
                "actor": ALICE,
                "idempotency_key": "ev-upd",
                "ops": [
                    {
                        "op": "update_node",
                        "type_id": TYPE_OAUTH,
                        "id": "ident-b",
                        "patch": {
                            # Mutate ident-b to collide with ident-a's
                            # composite tuple.
                            str(F_PROVIDER_USER_ID): "uid-1",
                        },
                    }
                ],
            }
        )
        result = await applier.apply_event(update)
        assert not result.success
        assert "composite unique" in (result.error or "").lower()
