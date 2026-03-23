"""
Workplace communication tests: Chat Messages, Channels, Threads.

Simulates chat with:
  - Messages (type_id=2): text, channel, thread_id
  - Channels (type_id=7): name, members
  - Edges: channel->message (edge_type=20), message->reply (edge_type=21),
           user->channel membership (edge_type=22)
"""

from __future__ import annotations

import asyncio

import pytest

from dbaas.entdb_server.apply.canonical_store import CanonicalStore

TENANT = "workspace-chat"
MSG_TYPE = 2
CHANNEL_TYPE = 7
EDGE_CHAN_MSG = 20
EDGE_MSG_REPLY = 21
EDGE_USER_CHAN = 22


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
# MESSAGE LIFECYCLE
# =========================================================================


@pytest.mark.integration
class TestMessageLifecycle:
    @pytest.mark.asyncio
    async def test_send_message(self, store):
        msg = await store.create_node(
            TENANT, MSG_TYPE, {"text": "Hello!", "channel": "general"}, "user:alice"
        )
        assert msg.node_id is not None
        assert msg.payload["text"] == "Hello!"
        assert msg.owner_actor == "user:alice"

    @pytest.mark.asyncio
    async def test_read_message(self, store):
        msg = await store.create_node(TENANT, MSG_TYPE, {"text": "Read this"}, "user:1")
        fetched = await store.get_node(TENANT, msg.node_id)
        assert fetched is not None
        assert fetched.payload["text"] == "Read this"

    @pytest.mark.asyncio
    async def test_edit_message(self, store):
        msg = await store.create_node(TENANT, MSG_TYPE, {"text": "Typo hre"}, "user:1")
        updated = await store.update_node(TENANT, msg.node_id, {"text": "Typo here"})
        assert updated.payload["text"] == "Typo here"

    @pytest.mark.asyncio
    async def test_delete_message(self, store):
        msg = await store.create_node(TENANT, MSG_TYPE, {"text": "Gone"}, "user:1")
        result = await store.delete_node(TENANT, msg.node_id)
        assert result is True
        assert await store.get_node(TENANT, msg.node_id) is None

    @pytest.mark.asyncio
    async def test_message_with_empty_text(self, store):
        msg = await store.create_node(TENANT, MSG_TYPE, {"text": ""}, "user:1")
        fetched = await store.get_node(TENANT, msg.node_id)
        assert fetched.payload["text"] == ""

    @pytest.mark.asyncio
    async def test_message_with_long_text_50kb(self, store):
        long_text = "x" * 51200
        msg = await store.create_node(TENANT, MSG_TYPE, {"text": long_text}, "user:1")
        fetched = await store.get_node(TENANT, msg.node_id)
        assert len(fetched.payload["text"]) == 51200

    @pytest.mark.asyncio
    async def test_message_with_unicode_emoji(self, store):
        msg = await store.create_node(TENANT, MSG_TYPE, {"text": "Great job! 🎉🚀💯"}, "user:1")
        fetched = await store.get_node(TENANT, msg.node_id)
        assert "🎉" in fetched.payload["text"]

    @pytest.mark.asyncio
    async def test_message_with_code_blocks(self, store):
        text = "```python\ndef hello():\n    print('hi')\n```"
        msg = await store.create_node(TENANT, MSG_TYPE, {"text": text}, "user:1")
        fetched = await store.get_node(TENANT, msg.node_id)
        assert "def hello():" in fetched.payload["text"]

    @pytest.mark.asyncio
    async def test_message_with_json_payload(self, store):
        msg = await store.create_node(
            TENANT,
            MSG_TYPE,
            {
                "text": "Status update",
                "channel": "dev",
                "thread_id": "t-100",
                "metadata": {"client": "web", "version": "2.0"},
            },
            "user:1",
        )
        fetched = await store.get_node(TENANT, msg.node_id)
        assert fetched.payload["metadata"]["client"] == "web"

    @pytest.mark.asyncio
    async def test_message_ordering_by_timestamp(self, store):
        for i in range(10):
            await store.create_node(
                TENANT, MSG_TYPE, {"text": f"Msg {i}"}, "user:1", created_at=3000 + i
            )
        msgs = await store.get_nodes_by_type(TENANT, MSG_TYPE)
        timestamps = [m.created_at for m in msgs]
        assert timestamps == sorted(timestamps, reverse=True)

    @pytest.mark.asyncio
    async def test_100_messages_in_sequence(self, store):
        for i in range(100):
            await store.create_node(TENANT, MSG_TYPE, {"text": f"Seq {i}", "seq": i}, "user:1")
        msgs = await store.get_nodes_by_type(TENANT, MSG_TYPE, limit=100)
        assert len(msgs) == 100

    @pytest.mark.asyncio
    async def test_message_by_different_users(self, store):
        actors = ["user:alice", "user:bob", "user:charlie"]
        for actor in actors:
            await store.create_node(TENANT, MSG_TYPE, {"text": f"From {actor}"}, actor)
        msgs = await store.get_nodes_by_type(TENANT, MSG_TYPE)
        msg_actors = {m.owner_actor for m in msgs}
        assert msg_actors == set(actors)

    @pytest.mark.asyncio
    async def test_message_with_metadata(self, store):
        msg = await store.create_node(
            TENANT,
            MSG_TYPE,
            {"text": "Rich", "mentions": ["user:bob"], "edited": False},
            "user:alice",
        )
        fetched = await store.get_node(TENANT, msg.node_id)
        assert fetched.payload["mentions"] == ["user:bob"]
        assert fetched.payload["edited"] is False

    @pytest.mark.asyncio
    async def test_message_with_attachments_field(self, store):
        msg = await store.create_node(
            TENANT,
            MSG_TYPE,
            {
                "text": "See attached",
                "attachments": [
                    {"name": "report.pdf", "size": 1024, "type": "application/pdf"},
                    {"name": "photo.jpg", "size": 2048, "type": "image/jpeg"},
                ],
            },
            "user:1",
        )
        fetched = await store.get_node(TENANT, msg.node_id)
        assert len(fetched.payload["attachments"]) == 2
        assert fetched.payload["attachments"][0]["name"] == "report.pdf"

    @pytest.mark.asyncio
    async def test_full_crud_cycle(self, store):
        # Create
        msg = await store.create_node(
            TENANT, MSG_TYPE, {"text": "v1", "channel": "general"}, "user:1"
        )
        mid = msg.node_id
        # Read
        fetched = await store.get_node(TENANT, mid)
        assert fetched.payload["text"] == "v1"
        # Update
        await store.update_node(TENANT, mid, {"text": "v2", "edited": True})
        fetched2 = await store.get_node(TENANT, mid)
        assert fetched2.payload["text"] == "v2"
        assert fetched2.payload["edited"] is True
        # Delete
        assert await store.delete_node(TENANT, mid) is True
        assert await store.get_node(TENANT, mid) is None


# =========================================================================
# CHANNEL OPERATIONS
# =========================================================================


@pytest.mark.integration
class TestChannelOperations:
    @pytest.mark.asyncio
    async def test_create_channel(self, store):
        ch = await store.create_node(
            TENANT,
            CHANNEL_TYPE,
            {"name": "general", "description": "Main channel"},
            "user:admin",
            node_id="ch-general",
        )
        assert ch.payload["name"] == "general"

    @pytest.mark.asyncio
    async def test_add_member_edge(self, store):
        await store.create_node(
            TENANT, CHANNEL_TYPE, {"name": "dev"}, "user:admin", node_id="ch-dev"
        )
        edge = await store.create_edge(TENANT, EDGE_USER_CHAN, "user:alice", "ch-dev")
        assert edge.from_node_id == "user:alice"
        assert edge.to_node_id == "ch-dev"

    @pytest.mark.asyncio
    async def test_remove_member(self, store):
        await store.create_node(
            TENANT, CHANNEL_TYPE, {"name": "temp"}, "user:admin", node_id="ch-temp"
        )
        await store.create_edge(TENANT, EDGE_USER_CHAN, "user:bob", "ch-temp")
        result = await store.delete_edge(TENANT, EDGE_USER_CHAN, "user:bob", "ch-temp")
        assert result is True
        edges = await store.get_edges_to(TENANT, "ch-temp", edge_type_id=EDGE_USER_CHAN)
        assert len(edges) == 0

    @pytest.mark.asyncio
    async def test_list_channels_for_user(self, store):
        for i in range(3):
            await store.create_node(
                TENANT, CHANNEL_TYPE, {"name": f"ch-{i}"}, "user:admin", node_id=f"ch-{i}"
            )
            await store.create_edge(TENANT, EDGE_USER_CHAN, "user:alice", f"ch-{i}")
        edges = await store.get_edges_from(TENANT, "user:alice", edge_type_id=EDGE_USER_CHAN)
        assert len(edges) == 3

    @pytest.mark.asyncio
    async def test_list_members_of_channel(self, store):
        await store.create_node(
            TENANT, CHANNEL_TYPE, {"name": "team"}, "user:admin", node_id="ch-team"
        )
        for uid in range(5):
            await store.create_edge(TENANT, EDGE_USER_CHAN, f"user:{uid}", "ch-team")
        edges = await store.get_edges_to(TENANT, "ch-team", edge_type_id=EDGE_USER_CHAN)
        assert len(edges) == 5

    @pytest.mark.asyncio
    async def test_message_in_channel(self, store):
        await store.create_node(
            TENANT, CHANNEL_TYPE, {"name": "dev"}, "user:admin", node_id="ch-msg"
        )
        msg = await store.create_node(
            TENANT, MSG_TYPE, {"text": "Hello channel", "channel": "dev"}, "user:1"
        )
        edge = await store.create_edge(TENANT, EDGE_CHAN_MSG, "ch-msg", msg.node_id)
        assert edge.edge_type_id == EDGE_CHAN_MSG

    @pytest.mark.asyncio
    async def test_multiple_channels(self, store):
        names = ["general", "random", "dev", "design", "marketing"]
        for name in names:
            await store.create_node(
                TENANT, CHANNEL_TYPE, {"name": name}, "user:admin", node_id=f"ch-{name}"
            )
        channels = await store.get_nodes_by_type(TENANT, CHANNEL_TYPE)
        assert len(channels) == 5
        ch_names = {c.payload["name"] for c in channels}
        assert ch_names == set(names)

    @pytest.mark.asyncio
    async def test_channel_with_no_members(self, store):
        await store.create_node(
            TENANT, CHANNEL_TYPE, {"name": "empty"}, "user:admin", node_id="ch-empty"
        )
        edges = await store.get_edges_to(TENANT, "ch-empty", edge_type_id=EDGE_USER_CHAN)
        assert len(edges) == 0

    @pytest.mark.asyncio
    async def test_channel_with_100_members(self, store):
        await store.create_node(
            TENANT, CHANNEL_TYPE, {"name": "big"}, "user:admin", node_id="ch-big"
        )
        for i in range(100):
            await store.create_edge(TENANT, EDGE_USER_CHAN, f"user:{i}", "ch-big")
        edges = await store.get_edges_to(TENANT, "ch-big", edge_type_id=EDGE_USER_CHAN)
        assert len(edges) == 100

    @pytest.mark.asyncio
    async def test_rename_channel(self, store):
        await store.create_node(
            TENANT, CHANNEL_TYPE, {"name": "old-name"}, "user:admin", node_id="ch-rename"
        )
        updated = await store.update_node(TENANT, "ch-rename", {"name": "new-name"})
        assert updated.payload["name"] == "new-name"

    @pytest.mark.asyncio
    async def test_delete_channel_cascades_edges(self, store):
        await store.create_node(
            TENANT, CHANNEL_TYPE, {"name": "doomed"}, "user:admin", node_id="ch-doom"
        )
        for i in range(3):
            await store.create_edge(TENANT, EDGE_USER_CHAN, f"user:{i}", "ch-doom")
        msg = await store.create_node(TENANT, MSG_TYPE, {"text": "bye"}, "user:1")
        await store.create_edge(TENANT, EDGE_CHAN_MSG, "ch-doom", msg.node_id)
        await store.delete_node(TENANT, "ch-doom")
        member_edges = await store.get_edges_to(TENANT, "ch-doom", edge_type_id=EDGE_USER_CHAN)
        msg_edges = await store.get_edges_from(TENANT, "ch-doom", edge_type_id=EDGE_CHAN_MSG)
        assert len(member_edges) == 0
        assert len(msg_edges) == 0

    @pytest.mark.asyncio
    async def test_private_channel_metadata(self, store):
        ch = await store.create_node(
            TENANT,
            CHANNEL_TYPE,
            {"name": "secret", "is_private": True, "invite_only": True},
            "user:admin",
        )
        fetched = await store.get_node(TENANT, ch.node_id)
        assert fetched.payload["is_private"] is True

    @pytest.mark.asyncio
    async def test_channel_message_count(self, store):
        await store.create_node(
            TENANT, CHANNEL_TYPE, {"name": "active"}, "user:admin", node_id="ch-active"
        )
        for i in range(15):
            m = await store.create_node(TENANT, MSG_TYPE, {"text": f"Msg {i}"}, "user:1")
            await store.create_edge(TENANT, EDGE_CHAN_MSG, "ch-active", m.node_id)
        edges = await store.get_edges_from(TENANT, "ch-active", edge_type_id=EDGE_CHAN_MSG)
        assert len(edges) == 15

    @pytest.mark.asyncio
    async def test_cross_channel_isolation(self, store):
        for ch_name in ["alpha", "beta"]:
            await store.create_node(
                TENANT, CHANNEL_TYPE, {"name": ch_name}, "user:admin", node_id=f"ch-{ch_name}"
            )
        for i in range(5):
            m = await store.create_node(TENANT, MSG_TYPE, {"text": f"A{i}"}, "user:1")
            await store.create_edge(TENANT, EDGE_CHAN_MSG, "ch-alpha", m.node_id)
        for i in range(3):
            m = await store.create_node(TENANT, MSG_TYPE, {"text": f"B{i}"}, "user:1")
            await store.create_edge(TENANT, EDGE_CHAN_MSG, "ch-beta", m.node_id)
        alpha_msgs = await store.get_edges_from(TENANT, "ch-alpha", edge_type_id=EDGE_CHAN_MSG)
        beta_msgs = await store.get_edges_from(TENANT, "ch-beta", edge_type_id=EDGE_CHAN_MSG)
        assert len(alpha_msgs) == 5
        assert len(beta_msgs) == 3

    @pytest.mark.asyncio
    async def test_direct_message_channel(self, store):
        ch = await store.create_node(
            TENANT,
            CHANNEL_TYPE,
            {"name": "dm-alice-bob", "is_dm": True},
            "user:alice",
            node_id="ch-dm",
        )
        await store.create_edge(TENANT, EDGE_USER_CHAN, "user:alice", "ch-dm")
        await store.create_edge(TENANT, EDGE_USER_CHAN, "user:bob", "ch-dm")
        members = await store.get_edges_to(TENANT, "ch-dm", edge_type_id=EDGE_USER_CHAN)
        assert len(members) == 2
        assert ch.payload["is_dm"] is True


# =========================================================================
# THREADED CONVERSATION
# =========================================================================


@pytest.mark.integration
class TestThreadedConversation:
    @pytest.mark.asyncio
    async def test_message_starts_thread(self, store):
        parent = await store.create_node(
            TENANT,
            MSG_TYPE,
            {"text": "Thread starter", "thread_id": None},
            "user:1",
            node_id="thread-root",
        )
        assert parent.payload["thread_id"] is None

    @pytest.mark.asyncio
    async def test_reply_to_thread(self, store):
        parent = await store.create_node(
            TENANT, MSG_TYPE, {"text": "Parent"}, "user:1", node_id="th-parent"
        )
        reply = await store.create_node(
            TENANT,
            MSG_TYPE,
            {"text": "Reply", "thread_id": "th-parent"},
            "user:2",
        )
        edge = await store.create_edge(TENANT, EDGE_MSG_REPLY, parent.node_id, reply.node_id)
        assert edge.from_node_id == "th-parent"

    @pytest.mark.asyncio
    async def test_get_thread_replies(self, store):
        await store.create_node(TENANT, MSG_TYPE, {"text": "Start"}, "user:1", node_id="th-start")
        for i in range(4):
            r = await store.create_node(TENANT, MSG_TYPE, {"text": f"Reply {i}"}, f"user:{i + 2}")
            await store.create_edge(TENANT, EDGE_MSG_REPLY, "th-start", r.node_id)
        edges = await store.get_edges_from(TENANT, "th-start", edge_type_id=EDGE_MSG_REPLY)
        assert len(edges) == 4

    @pytest.mark.asyncio
    async def test_nested_replies(self, store):
        m1 = await store.create_node(TENANT, MSG_TYPE, {"text": "L0"}, "user:1", node_id="nr-0")
        m2 = await store.create_node(TENANT, MSG_TYPE, {"text": "L1"}, "user:2", node_id="nr-1")
        m3 = await store.create_node(TENANT, MSG_TYPE, {"text": "L2"}, "user:3", node_id="nr-2")
        await store.create_edge(TENANT, EDGE_MSG_REPLY, m1.node_id, m2.node_id)
        await store.create_edge(TENANT, EDGE_MSG_REPLY, m2.node_id, m3.node_id)

        l1_replies = await store.get_edges_from(TENANT, "nr-0", edge_type_id=EDGE_MSG_REPLY)
        l2_replies = await store.get_edges_from(TENANT, "nr-1", edge_type_id=EDGE_MSG_REPLY)
        assert len(l1_replies) == 1
        assert len(l2_replies) == 1
        assert l2_replies[0].to_node_id == "nr-2"

    @pytest.mark.asyncio
    async def test_thread_across_multiple_users(self, store):
        await store.create_node(TENANT, MSG_TYPE, {"text": "Discussion"}, "user:1", node_id="disc")
        users = ["user:alice", "user:bob", "user:charlie", "user:diana"]
        for _i, user in enumerate(users):
            r = await store.create_node(TENANT, MSG_TYPE, {"text": f"Input from {user}"}, user)
            await store.create_edge(TENANT, EDGE_MSG_REPLY, "disc", r.node_id)
        edges = await store.get_edges_from(TENANT, "disc", edge_type_id=EDGE_MSG_REPLY)
        assert len(edges) == 4

    @pytest.mark.asyncio
    async def test_edit_reply(self, store):
        await store.create_node(TENANT, MSG_TYPE, {"text": "Parent"}, "user:1", node_id="er-parent")
        reply = await store.create_node(TENANT, MSG_TYPE, {"text": "Wrnog"}, "user:2")
        await store.create_edge(TENANT, EDGE_MSG_REPLY, "er-parent", reply.node_id)
        updated = await store.update_node(TENANT, reply.node_id, {"text": "Wrong -> Fixed"})
        assert updated.payload["text"] == "Wrong -> Fixed"

    @pytest.mark.asyncio
    async def test_delete_reply(self, store):
        await store.create_node(TENANT, MSG_TYPE, {"text": "Parent"}, "user:1", node_id="dr-parent")
        reply = await store.create_node(
            TENANT, MSG_TYPE, {"text": "Delete me"}, "user:2", node_id="dr-reply"
        )
        await store.create_edge(TENANT, EDGE_MSG_REPLY, "dr-parent", reply.node_id)
        await store.delete_node(TENANT, "dr-reply")
        edges = await store.get_edges_from(TENANT, "dr-parent", edge_type_id=EDGE_MSG_REPLY)
        assert len(edges) == 0

    @pytest.mark.asyncio
    async def test_thread_ordering(self, store):
        await store.create_node(TENANT, MSG_TYPE, {"text": "Root"}, "user:1", node_id="to-root")
        for i in range(5):
            r = await store.create_node(
                TENANT, MSG_TYPE, {"text": f"R{i}"}, "user:1", created_at=4000 + i
            )
            await store.create_edge(
                TENANT, EDGE_MSG_REPLY, "to-root", r.node_id, created_at=4000 + i
            )
        edges = await store.get_edges_from(TENANT, "to-root", edge_type_id=EDGE_MSG_REPLY)
        assert len(edges) == 5

    @pytest.mark.asyncio
    async def test_empty_thread(self, store):
        await store.create_node(
            TENANT, MSG_TYPE, {"text": "No replies"}, "user:1", node_id="empty-th"
        )
        edges = await store.get_edges_from(TENANT, "empty-th", edge_type_id=EDGE_MSG_REPLY)
        assert len(edges) == 0

    @pytest.mark.asyncio
    async def test_long_thread_50_messages(self, store):
        await store.create_node(
            TENANT, MSG_TYPE, {"text": "Long thread"}, "user:1", node_id="lt-root"
        )
        for i in range(50):
            r = await store.create_node(
                TENANT, MSG_TYPE, {"text": f"Reply {i}"}, f"user:{(i % 5) + 1}"
            )
            await store.create_edge(TENANT, EDGE_MSG_REPLY, "lt-root", r.node_id)
        edges = await store.get_edges_from(TENANT, "lt-root", edge_type_id=EDGE_MSG_REPLY)
        assert len(edges) == 50

    @pytest.mark.asyncio
    async def test_thread_in_channel_context(self, store):
        await store.create_node(
            TENANT, CHANNEL_TYPE, {"name": "dev"}, "user:admin", node_id="ch-thread"
        )
        parent = await store.create_node(
            TENANT, MSG_TYPE, {"text": "In channel"}, "user:1", node_id="ch-th-msg"
        )
        await store.create_edge(TENANT, EDGE_CHAN_MSG, "ch-thread", parent.node_id)
        reply = await store.create_node(TENANT, MSG_TYPE, {"text": "Thread reply"}, "user:2")
        await store.create_edge(TENANT, EDGE_MSG_REPLY, parent.node_id, reply.node_id)

        chan_msgs = await store.get_edges_from(TENANT, "ch-thread", edge_type_id=EDGE_CHAN_MSG)
        thread_replies = await store.get_edges_from(
            TENANT, "ch-th-msg", edge_type_id=EDGE_MSG_REPLY
        )
        assert len(chan_msgs) == 1
        assert len(thread_replies) == 1

    @pytest.mark.asyncio
    async def test_thread_reply_to_reply(self, store):
        ids = []
        for i in range(4):
            m = await store.create_node(
                TENANT, MSG_TYPE, {"text": f"Chain {i}"}, "user:1", node_id=f"chain-{i}"
            )
            ids.append(m.node_id)
        for i in range(len(ids) - 1):
            await store.create_edge(TENANT, EDGE_MSG_REPLY, ids[i], ids[i + 1])
        # Verify the full chain
        for i in range(len(ids) - 1):
            edges = await store.get_edges_from(TENANT, ids[i], edge_type_id=EDGE_MSG_REPLY)
            assert len(edges) == 1
            assert edges[0].to_node_id == ids[i + 1]

    @pytest.mark.asyncio
    async def test_thread_metadata_in_payload(self, store):
        parent = await store.create_node(
            TENANT,
            MSG_TYPE,
            {"text": "Start", "thread_id": None, "reply_count": 0},
            "user:1",
            node_id="tm-parent",
        )
        reply = await store.create_node(
            TENANT,
            MSG_TYPE,
            {"text": "Reply", "thread_id": "tm-parent"},
            "user:2",
        )
        await store.create_edge(TENANT, EDGE_MSG_REPLY, parent.node_id, reply.node_id)
        await store.update_node(TENANT, "tm-parent", {"reply_count": 1})
        fetched = await store.get_node(TENANT, "tm-parent")
        assert fetched.payload["reply_count"] == 1

    @pytest.mark.asyncio
    async def test_concurrent_thread_replies(self, store):
        await store.create_node(
            TENANT, MSG_TYPE, {"text": "Concurrent root"}, "user:1", node_id="cc-root"
        )

        async def add_reply(idx):
            r = await store.create_node(
                TENANT, MSG_TYPE, {"text": f"Concurrent {idx}"}, f"user:{idx}"
            )
            await store.create_edge(TENANT, EDGE_MSG_REPLY, "cc-root", r.node_id)
            return r

        tasks = [add_reply(i) for i in range(10)]
        await asyncio.gather(*tasks)
        edges = await store.get_edges_from(TENANT, "cc-root", edge_type_id=EDGE_MSG_REPLY)
        assert len(edges) == 10

    @pytest.mark.asyncio
    async def test_delete_thread_root_keeps_replies(self, store):
        await store.create_node(TENANT, MSG_TYPE, {"text": "Root"}, "user:1", node_id="del-root")
        reply = await store.create_node(
            TENANT, MSG_TYPE, {"text": "Orphan reply"}, "user:2", node_id="orphan-r"
        )
        await store.create_edge(TENANT, EDGE_MSG_REPLY, "del-root", reply.node_id)
        await store.delete_node(TENANT, "del-root")
        # Reply node still exists, edge is gone
        r = await store.get_node(TENANT, "orphan-r")
        assert r is not None
        edges = await store.get_edges_from(TENANT, "del-root", edge_type_id=EDGE_MSG_REPLY)
        assert len(edges) == 0


# =========================================================================
# CHAT HISTORY
# =========================================================================


@pytest.mark.integration
class TestChatHistory:
    @pytest.mark.asyncio
    async def test_load_last_n_messages(self, store):
        for i in range(20):
            await store.create_node(
                TENANT, MSG_TYPE, {"text": f"History {i}"}, "user:1", created_at=5000 + i
            )
        last5 = await store.get_nodes_by_type(TENANT, MSG_TYPE, limit=5)
        assert len(last5) == 5
        # Should be newest first
        assert last5[0].created_at > last5[4].created_at

    @pytest.mark.asyncio
    async def test_scroll_up_offset_pagination(self, store):
        for i in range(30):
            await store.create_node(
                TENANT, MSG_TYPE, {"text": f"Scroll {i}"}, "user:1", created_at=6000 + i
            )
        page1 = await store.get_nodes_by_type(TENANT, MSG_TYPE, limit=10, offset=0)
        page2 = await store.get_nodes_by_type(TENANT, MSG_TYPE, limit=10, offset=10)
        page3 = await store.get_nodes_by_type(TENANT, MSG_TYPE, limit=10, offset=20)
        all_ids = set()
        for page in [page1, page2, page3]:
            assert len(page) == 10
            for m in page:
                all_ids.add(m.node_id)
        assert len(all_ids) == 30

    @pytest.mark.asyncio
    async def test_search_by_channel_filter(self, store):
        ch_name = "ch-search"
        await store.create_node(
            TENANT, CHANNEL_TYPE, {"name": ch_name}, "user:admin", node_id=ch_name
        )
        for i in range(5):
            m = await store.create_node(
                TENANT, MSG_TYPE, {"text": f"In ch {i}", "channel": ch_name}, "user:1"
            )
            await store.create_edge(TENANT, EDGE_CHAN_MSG, ch_name, m.node_id)
        for i in range(3):
            await store.create_node(
                TENANT, MSG_TYPE, {"text": f"Other {i}", "channel": "other"}, "user:1"
            )
        ch_edges = await store.get_edges_from(TENANT, ch_name, edge_type_id=EDGE_CHAN_MSG)
        assert len(ch_edges) == 5

    @pytest.mark.asyncio
    async def test_messages_across_tenants_isolated(self, multi_store):
        await multi_store.create_node(
            TENANT, MSG_TYPE, {"text": "Tenant A msg"}, "user:1", node_id="iso-msg"
        )
        await multi_store.create_node(
            "workspace-other", MSG_TYPE, {"text": "Tenant B msg"}, "user:1", node_id="iso-msg"
        )
        a = await multi_store.get_node(TENANT, "iso-msg")
        b = await multi_store.get_node("workspace-other", "iso-msg")
        assert a.payload["text"] == "Tenant A msg"
        assert b.payload["text"] == "Tenant B msg"

    @pytest.mark.asyncio
    async def test_history_with_deleted_messages(self, store):
        ids = []
        for i in range(10):
            m = await store.create_node(TENANT, MSG_TYPE, {"text": f"Del test {i}"}, "user:1")
            ids.append(m.node_id)
        # Delete every other message
        for i in range(0, 10, 2):
            await store.delete_node(TENANT, ids[i])
        remaining = await store.get_nodes_by_type(TENANT, MSG_TYPE)
        assert len(remaining) == 5

    @pytest.mark.asyncio
    async def test_chat_history_for_specific_user(self, store):
        for i in range(5):
            await store.create_node(TENANT, MSG_TYPE, {"text": f"Alice {i}"}, "user:alice")
        for i in range(3):
            await store.create_node(TENANT, MSG_TYPE, {"text": f"Bob {i}"}, "user:bob")
        # Use visible_nodes with principal filter
        alice_msgs = await store.get_visible_nodes(TENANT, "user:alice", type_id=MSG_TYPE)
        assert len(alice_msgs) == 5

    @pytest.mark.asyncio
    async def test_large_history_paginated(self, store):
        for i in range(100):
            await store.create_node(
                TENANT, MSG_TYPE, {"text": f"Bulk {i}"}, "user:1", created_at=7000 + i
            )
        all_ids = set()
        for offset in range(0, 100, 20):
            page = await store.get_nodes_by_type(TENANT, MSG_TYPE, limit=20, offset=offset)
            assert len(page) == 20
            for m in page:
                all_ids.add(m.node_id)
        assert len(all_ids) == 100

    @pytest.mark.asyncio
    async def test_history_ordering_newest_first(self, store):
        for i in range(10):
            await store.create_node(
                TENANT, MSG_TYPE, {"text": f"Order {i}"}, "user:1", created_at=8000 + i
            )
        msgs = await store.get_nodes_by_type(TENANT, MSG_TYPE, limit=10)
        for j in range(len(msgs) - 1):
            assert msgs[j].created_at >= msgs[j + 1].created_at

    @pytest.mark.asyncio
    async def test_empty_channel_history(self, store):
        await store.create_node(
            TENANT, CHANNEL_TYPE, {"name": "ghost"}, "user:admin", node_id="ch-ghost"
        )
        edges = await store.get_edges_from(TENANT, "ch-ghost", edge_type_id=EDGE_CHAN_MSG)
        assert len(edges) == 0


# =========================================================================
# CHAT SCALE
# =========================================================================


@pytest.mark.integration
class TestChatScale:
    @pytest.mark.asyncio
    async def test_500_messages_rapid_creation(self, store):
        for i in range(500):
            await store.create_node(TENANT, MSG_TYPE, {"text": f"Rapid {i}", "seq": i}, "user:1")
        msgs = await store.get_nodes_by_type(TENANT, MSG_TYPE, limit=100, offset=0)
        assert len(msgs) == 100
        total_pages = []
        for offset in range(0, 500, 100):
            page = await store.get_nodes_by_type(TENANT, MSG_TYPE, limit=100, offset=offset)
            total_pages.extend(page)
        assert len(total_pages) == 500

    @pytest.mark.asyncio
    async def test_10_channels_50_messages_each(self, store):
        for ch_idx in range(10):
            ch_id = f"scale-ch-{ch_idx}"
            await store.create_node(
                TENANT, CHANNEL_TYPE, {"name": f"channel-{ch_idx}"}, "user:admin", node_id=ch_id
            )
            for msg_idx in range(50):
                m = await store.create_node(
                    TENANT,
                    MSG_TYPE,
                    {"text": f"Ch{ch_idx} Msg{msg_idx}", "channel": ch_id},
                    "user:1",
                )
                await store.create_edge(TENANT, EDGE_CHAN_MSG, ch_id, m.node_id)
        # Verify each channel
        for ch_idx in range(10):
            edges = await store.get_edges_from(
                TENANT, f"scale-ch-{ch_idx}", edge_type_id=EDGE_CHAN_MSG
            )
            assert len(edges) == 50

    @pytest.mark.asyncio
    async def test_100_users_one_channel(self, store):
        await store.create_node(
            TENANT, CHANNEL_TYPE, {"name": "allhands"}, "user:admin", node_id="ch-allhands"
        )
        for i in range(100):
            await store.create_edge(TENANT, EDGE_USER_CHAN, f"user:{i}", "ch-allhands")
        members = await store.get_edges_to(TENANT, "ch-allhands", edge_type_id=EDGE_USER_CHAN)
        assert len(members) == 100

    @pytest.mark.asyncio
    async def test_channel_many_messages_pagination(self, store):
        await store.create_node(
            TENANT, CHANNEL_TYPE, {"name": "firehose"}, "user:admin", node_id="ch-firehose"
        )
        for i in range(200):
            m = await store.create_node(TENANT, MSG_TYPE, {"text": f"Fire {i}"}, "user:1")
            await store.create_edge(TENANT, EDGE_CHAN_MSG, "ch-firehose", m.node_id)
        all_msgs = await store.get_nodes_by_type(TENANT, MSG_TYPE, limit=100, offset=0)
        assert len(all_msgs) == 100
        page2 = await store.get_nodes_by_type(TENANT, MSG_TYPE, limit=100, offset=100)
        assert len(page2) == 100

    @pytest.mark.asyncio
    async def test_concurrent_reads_during_writes(self, store):
        # Pre-populate
        for i in range(50):
            await store.create_node(TENANT, MSG_TYPE, {"text": f"Pre {i}"}, "user:1")

        async def writer():
            for i in range(20):
                await store.create_node(TENANT, MSG_TYPE, {"text": f"Write {i}"}, "user:writer")

        async def reader():
            results = []
            for _ in range(20):
                msgs = await store.get_nodes_by_type(TENANT, MSG_TYPE, limit=10)
                results.append(len(msgs))
            return results

        _, read_results = await asyncio.gather(writer(), reader())
        # All reads should return 10 (limit)
        assert all(r == 10 for r in read_results)

    @pytest.mark.asyncio
    async def test_fanout_simulation_via_edges(self, store):
        await store.create_node(
            TENANT, CHANNEL_TYPE, {"name": "announce"}, "user:admin", node_id="ch-announce"
        )
        # 20 members
        for i in range(20):
            await store.create_edge(TENANT, EDGE_USER_CHAN, f"user:{i}", "ch-announce")
        # Send a message
        msg = await store.create_node(TENANT, MSG_TYPE, {"text": "Important!"}, "user:admin")
        await store.create_edge(TENANT, EDGE_CHAN_MSG, "ch-announce", msg.node_id)
        # Verify all members can be found via channel
        members = await store.get_edges_to(TENANT, "ch-announce", edge_type_id=EDGE_USER_CHAN)
        assert len(members) == 20
        chan_msgs = await store.get_edges_from(TENANT, "ch-announce", edge_type_id=EDGE_CHAN_MSG)
        assert len(chan_msgs) == 1

    @pytest.mark.asyncio
    async def test_bulk_message_deletion(self, store):
        msg_ids = []
        for i in range(50):
            m = await store.create_node(TENANT, MSG_TYPE, {"text": f"Bulk del {i}"}, "user:1")
            msg_ids.append(m.node_id)
        for mid in msg_ids:
            await store.delete_node(TENANT, mid)
        remaining = await store.get_nodes_by_type(TENANT, MSG_TYPE)
        assert len(remaining) == 0


# =========================================================================
# CHAT EDGE CASES
# =========================================================================


@pytest.mark.integration
class TestChatEdgeCases:
    @pytest.mark.asyncio
    async def test_message_with_only_whitespace(self, store):
        msg = await store.create_node(TENANT, MSG_TYPE, {"text": "   \t\n  "}, "user:1")
        fetched = await store.get_node(TENANT, msg.node_id)
        assert fetched.payload["text"].strip() == ""

    @pytest.mark.asyncio
    async def test_message_with_only_emoji(self, store):
        msg = await store.create_node(TENANT, MSG_TYPE, {"text": "👍👍👍"}, "user:1")
        fetched = await store.get_node(TENANT, msg.node_id)
        assert fetched.payload["text"] == "👍👍👍"

    @pytest.mark.asyncio
    async def test_message_at_max_text_length(self, store):
        max_text = "a" * 100000
        msg = await store.create_node(TENANT, MSG_TYPE, {"text": max_text}, "user:1")
        fetched = await store.get_node(TENANT, msg.node_id)
        assert len(fetched.payload["text"]) == 100000

    @pytest.mark.asyncio
    async def test_message_with_null_channel(self, store):
        msg = await store.create_node(
            TENANT, MSG_TYPE, {"text": "No channel", "channel": None}, "user:1"
        )
        fetched = await store.get_node(TENANT, msg.node_id)
        assert fetched.payload["channel"] is None

    @pytest.mark.asyncio
    async def test_message_to_self(self, store):
        msg = await store.create_node(TENANT, MSG_TYPE, {"text": "Note to self"}, "user:me")
        edge = await store.create_edge(TENANT, EDGE_MSG_REPLY, msg.node_id, msg.node_id)
        assert edge.from_node_id == edge.to_node_id

    @pytest.mark.asyncio
    async def test_duplicate_message_different_ids(self, store):
        m1 = await store.create_node(TENANT, MSG_TYPE, {"text": "Same text"}, "user:1")
        m2 = await store.create_node(TENANT, MSG_TYPE, {"text": "Same text"}, "user:1")
        assert m1.node_id != m2.node_id

    @pytest.mark.asyncio
    async def test_edited_message_preserves_created_at(self, store):
        msg = await store.create_node(
            TENANT, MSG_TYPE, {"text": "Original"}, "user:1", created_at=9000
        )
        original_created = msg.created_at
        updated = await store.update_node(TENANT, msg.node_id, {"text": "Edited"})
        assert updated.created_at == original_created
        assert updated.updated_at > original_created

    @pytest.mark.asyncio
    async def test_delete_nonexistent_message(self, store):
        result = await store.delete_node(TENANT, "ghost-msg-id")
        assert result is False

    @pytest.mark.asyncio
    async def test_message_with_binary_like_data(self, store):
        msg = await store.create_node(
            TENANT,
            MSG_TYPE,
            {"text": "Binary ref", "data_b64": "SGVsbG8gV29ybGQ="},
            "user:1",
        )
        fetched = await store.get_node(TENANT, msg.node_id)
        assert fetched.payload["data_b64"] == "SGVsbG8gV29ybGQ="

    @pytest.mark.asyncio
    async def test_message_with_newlines(self, store):
        text = "Line 1\nLine 2\nLine 3\n"
        msg = await store.create_node(TENANT, MSG_TYPE, {"text": text}, "user:1")
        fetched = await store.get_node(TENANT, msg.node_id)
        assert fetched.payload["text"].count("\n") == 3

    @pytest.mark.asyncio
    async def test_message_with_html_tags(self, store):
        text = "<b>Bold</b> and <script>alert('xss')</script>"
        msg = await store.create_node(TENANT, MSG_TYPE, {"text": text}, "user:1")
        fetched = await store.get_node(TENANT, msg.node_id)
        assert "<script>" in fetched.payload["text"]

    @pytest.mark.asyncio
    async def test_message_with_sql_injection_in_payload(self, store):
        text = "'; DROP TABLE nodes; --"
        msg = await store.create_node(TENANT, MSG_TYPE, {"text": text}, "user:1")
        fetched = await store.get_node(TENANT, msg.node_id)
        assert fetched.payload["text"] == text
        # Verify table still works
        all_msgs = await store.get_nodes_by_type(TENANT, MSG_TYPE)
        assert len(all_msgs) >= 1

    @pytest.mark.asyncio
    async def test_message_with_very_long_channel_name(self, store):
        long_ch = "channel-" + "x" * 1000
        msg = await store.create_node(
            TENANT, MSG_TYPE, {"text": "In long channel", "channel": long_ch}, "user:1"
        )
        fetched = await store.get_node(TENANT, msg.node_id)
        assert fetched.payload["channel"] == long_ch

    @pytest.mark.asyncio
    async def test_concurrent_edits_to_same_message(self, store):
        msg = await store.create_node(TENANT, MSG_TYPE, {"text": "Race", "version": 0}, "user:1")
        mid = msg.node_id

        async def edit(v):
            await store.update_node(TENANT, mid, {"version": v})

        await asyncio.gather(*[edit(i) for i in range(1, 11)])
        fetched = await store.get_node(TENANT, mid)
        # Last write wins; version should be one of the written values
        assert fetched.payload["version"] in list(range(1, 11))

    @pytest.mark.asyncio
    async def test_message_with_zero_length_text(self, store):
        msg = await store.create_node(TENANT, MSG_TYPE, {"text": ""}, "user:1")
        fetched = await store.get_node(TENANT, msg.node_id)
        assert fetched.payload["text"] == ""
        assert len(fetched.payload["text"]) == 0

    @pytest.mark.asyncio
    async def test_update_nonexistent_message(self, store):
        result = await store.update_node(TENANT, "no-such-msg", {"text": "nope"})
        assert result is None

    @pytest.mark.asyncio
    async def test_message_with_url_in_text(self, store):
        text = "Check out https://example.com/path?q=1&b=2#frag"
        msg = await store.create_node(TENANT, MSG_TYPE, {"text": text}, "user:1")
        fetched = await store.get_node(TENANT, msg.node_id)
        assert fetched.payload["text"] == text

    @pytest.mark.asyncio
    async def test_channel_member_rejoin(self, store):
        await store.create_node(
            TENANT, CHANNEL_TYPE, {"name": "rejoin"}, "user:admin", node_id="ch-rejoin"
        )
        await store.create_edge(TENANT, EDGE_USER_CHAN, "user:alice", "ch-rejoin")
        await store.delete_edge(TENANT, EDGE_USER_CHAN, "user:alice", "ch-rejoin")
        await store.create_edge(TENANT, EDGE_USER_CHAN, "user:alice", "ch-rejoin")
        members = await store.get_edges_to(TENANT, "ch-rejoin", edge_type_id=EDGE_USER_CHAN)
        assert len(members) == 1

    @pytest.mark.asyncio
    async def test_message_preserves_all_payload_fields_on_edit(self, store):
        msg = await store.create_node(
            TENANT,
            MSG_TYPE,
            {"text": "Original", "channel": "dev", "priority": "high"},
            "user:1",
        )
        updated = await store.update_node(TENANT, msg.node_id, {"text": "Edited"})
        assert updated.payload["channel"] == "dev"
        assert updated.payload["priority"] == "high"
        assert updated.payload["text"] == "Edited"

    @pytest.mark.asyncio
    async def test_multiple_edge_types_on_same_nodes(self, store):
        await store.create_node(TENANT, MSG_TYPE, {"text": "Source"}, "user:1", node_id="src-msg")
        await store.create_node(TENANT, MSG_TYPE, {"text": "Target"}, "user:2", node_id="tgt-msg")
        await store.create_edge(TENANT, EDGE_MSG_REPLY, "src-msg", "tgt-msg")
        await store.create_edge(TENANT, EDGE_CHAN_MSG, "src-msg", "tgt-msg")
        all_edges = await store.get_edges_from(TENANT, "src-msg")
        assert len(all_edges) == 2
        reply_edges = await store.get_edges_from(TENANT, "src-msg", edge_type_id=EDGE_MSG_REPLY)
        chan_edges = await store.get_edges_from(TENANT, "src-msg", edge_type_id=EDGE_CHAN_MSG)
        assert len(reply_edges) == 1
        assert len(chan_edges) == 1
