"""
Workplace communication tests: Posts, Comments, Votes.

Simulates a college Reddit-like forum with:
  - Posts (type_id=3): title, body, subreddit, score
  - Comments (type_id=4): text, score, parent_post
  - Votes/Reactions (type_id=6): voter, target, vote_type
  - Edges: post->comment (edge_type=10), user->post upvote (edge_type=11),
           comment->comment reply (edge_type=12)
"""

from __future__ import annotations

import asyncio
import time

import pytest

from dbaas.entdb_server.apply.canonical_store import CanonicalStore

TENANT = "campus"
POST_TYPE = 3
COMMENT_TYPE = 4
VOTE_TYPE = 6
EDGE_POST_COMMENT = 10
EDGE_USER_UPVOTE = 11
EDGE_COMMENT_REPLY = 12


@pytest.fixture
def store(tmp_path):
    s = CanonicalStore(data_dir=str(tmp_path), wal_mode=True)
    asyncio.get_event_loop().run_until_complete(s.initialize_tenant(TENANT))
    return s


# =========================================================================
# POST LIFECYCLE
# =========================================================================


@pytest.mark.integration
class TestPostLifecycle:
    @pytest.mark.asyncio
    async def test_create_post(self, store):
        post = await store.create_node(
            TENANT,
            POST_TYPE,
            {"title": "First post!", "body": "Hello world", "subreddit": "general", "score": 0},
            "user:1",
        )
        assert post.node_id is not None
        assert post.type_id == POST_TYPE
        assert post.payload["title"] == "First post!"

    @pytest.mark.asyncio
    async def test_read_post(self, store):
        post = await store.create_node(
            TENANT, POST_TYPE, {"title": "Read me", "body": "Content"}, "user:1"
        )
        fetched = await store.get_node(TENANT, post.node_id)
        assert fetched is not None
        assert fetched.payload["title"] == "Read me"
        assert fetched.payload["body"] == "Content"

    @pytest.mark.asyncio
    async def test_update_post_body(self, store):
        post = await store.create_node(
            TENANT, POST_TYPE, {"title": "Edit me", "body": "Original"}, "user:1"
        )
        updated = await store.update_node(TENANT, post.node_id, {"body": "Edited body"})
        assert updated.payload["body"] == "Edited body"
        assert updated.payload["title"] == "Edit me"

    @pytest.mark.asyncio
    async def test_update_post_score(self, store):
        post = await store.create_node(
            TENANT, POST_TYPE, {"title": "Vote me", "score": 0}, "user:1"
        )
        updated = await store.update_node(TENANT, post.node_id, {"score": 42})
        assert updated.payload["score"] == 42

    @pytest.mark.asyncio
    async def test_delete_post(self, store):
        post = await store.create_node(TENANT, POST_TYPE, {"title": "Bye"}, "user:1")
        result = await store.delete_node(TENANT, post.node_id)
        assert result is True
        assert await store.get_node(TENANT, post.node_id) is None

    @pytest.mark.asyncio
    async def test_create_post_with_all_fields(self, store):
        post = await store.create_node(
            TENANT,
            POST_TYPE,
            {
                "title": "Complete",
                "body": "Full post",
                "subreddit": "cs101",
                "score": 5,
                "flair": "discussion",
                "is_pinned": False,
            },
            "user:42",
        )
        assert post.payload["flair"] == "discussion"
        assert post.payload["is_pinned"] is False
        assert post.owner_actor == "user:42"

    @pytest.mark.asyncio
    async def test_post_with_empty_body(self, store):
        post = await store.create_node(
            TENANT, POST_TYPE, {"title": "Title only", "body": ""}, "user:1"
        )
        fetched = await store.get_node(TENANT, post.node_id)
        assert fetched.payload["body"] == ""

    @pytest.mark.asyncio
    async def test_post_with_very_long_body(self, store):
        long_body = "x" * 10240
        post = await store.create_node(
            TENANT, POST_TYPE, {"title": "Long", "body": long_body}, "user:1"
        )
        fetched = await store.get_node(TENANT, post.node_id)
        assert len(fetched.payload["body"]) == 10240

    @pytest.mark.asyncio
    async def test_post_with_special_chars_in_title(self, store):
        title = "He said \"hello\" & she said <goodbye> 'ok'"
        post = await store.create_node(TENANT, POST_TYPE, {"title": title}, "user:1")
        fetched = await store.get_node(TENANT, post.node_id)
        assert fetched.payload["title"] == title

    @pytest.mark.asyncio
    async def test_multiple_posts_same_subreddit(self, store):
        for i in range(5):
            await store.create_node(
                TENANT, POST_TYPE, {"title": f"Post {i}", "subreddit": "cs101"}, "user:1"
            )
        posts = await store.get_nodes_by_type(TENANT, POST_TYPE)
        cs101_posts = [p for p in posts if p.payload.get("subreddit") == "cs101"]
        assert len(cs101_posts) == 5

    @pytest.mark.asyncio
    async def test_post_ordering_by_created_at(self, store):
        for i in range(5):
            await store.create_node(
                TENANT, POST_TYPE, {"title": f"Post {i}"}, "user:1", created_at=1000 + i
            )
        posts = await store.get_nodes_by_type(TENANT, POST_TYPE)
        # Default ordering is created_at DESC
        timestamps = [p.created_at for p in posts]
        assert timestamps == sorted(timestamps, reverse=True)

    @pytest.mark.asyncio
    async def test_post_pagination(self, store):
        for i in range(15):
            await store.create_node(TENANT, POST_TYPE, {"title": f"Post {i}"}, "user:1")
        page1 = await store.get_nodes_by_type(TENANT, POST_TYPE, limit=5, offset=0)
        page2 = await store.get_nodes_by_type(TENANT, POST_TYPE, limit=5, offset=5)
        page3 = await store.get_nodes_by_type(TENANT, POST_TYPE, limit=5, offset=10)
        assert len(page1) == 5
        assert len(page2) == 5
        assert len(page3) == 5
        all_ids = (
            {p.node_id for p in page1} | {p.node_id for p in page2} | {p.node_id for p in page3}
        )
        assert len(all_ids) == 15

    @pytest.mark.asyncio
    async def test_count_posts_by_subreddit_via_type_query(self, store):
        for i in range(3):
            await store.create_node(
                TENANT, POST_TYPE, {"title": f"A{i}", "subreddit": "math"}, "user:1"
            )
        for i in range(7):
            await store.create_node(
                TENANT, POST_TYPE, {"title": f"B{i}", "subreddit": "physics"}, "user:1"
            )
        all_posts = await store.get_nodes_by_type(TENANT, POST_TYPE)
        math = [p for p in all_posts if p.payload.get("subreddit") == "math"]
        physics = [p for p in all_posts if p.payload.get("subreddit") == "physics"]
        assert len(math) == 3
        assert len(physics) == 7

    @pytest.mark.asyncio
    async def test_post_with_unicode_title(self, store):
        post = await store.create_node(TENANT, POST_TYPE, {"title": "Cafe des Amis"}, "user:1")
        fetched = await store.get_node(TENANT, post.node_id)
        assert fetched.payload["title"] == "Cafe des Amis"

    @pytest.mark.asyncio
    async def test_full_crud_cycle(self, store):
        # Create
        post = await store.create_node(
            TENANT, POST_TYPE, {"title": "Lifecycle", "body": "v1", "score": 0}, "user:1"
        )
        pid = post.node_id
        # Read
        fetched = await store.get_node(TENANT, pid)
        assert fetched.payload["body"] == "v1"
        # Update
        await store.update_node(TENANT, pid, {"body": "v2", "score": 10})
        fetched2 = await store.get_node(TENANT, pid)
        assert fetched2.payload["body"] == "v2"
        assert fetched2.payload["score"] == 10
        # Delete
        assert await store.delete_node(TENANT, pid) is True
        assert await store.get_node(TENANT, pid) is None


# =========================================================================
# COMMENT THREAD
# =========================================================================


@pytest.mark.integration
class TestCommentThread:
    @pytest.mark.asyncio
    async def test_create_comment_on_post(self, store):
        post = await store.create_node(
            TENANT, POST_TYPE, {"title": "Parent"}, "user:1", node_id="post-1"
        )
        comment = await store.create_node(
            TENANT, COMMENT_TYPE, {"text": "Nice post!", "score": 0}, "user:2"
        )
        edge = await store.create_edge(TENANT, EDGE_POST_COMMENT, post.node_id, comment.node_id)
        assert edge.from_node_id == "post-1"
        assert edge.to_node_id == comment.node_id

    @pytest.mark.asyncio
    async def test_create_nested_reply(self, store):
        post = await store.create_node(
            TENANT, POST_TYPE, {"title": "Thread"}, "user:1", node_id="post-t"
        )
        c1 = await store.create_node(
            TENANT, COMMENT_TYPE, {"text": "Top-level"}, "user:2", node_id="c1"
        )
        c2 = await store.create_node(
            TENANT, COMMENT_TYPE, {"text": "Reply to c1"}, "user:3", node_id="c2"
        )
        await store.create_edge(TENANT, EDGE_POST_COMMENT, post.node_id, c1.node_id)
        await store.create_edge(TENANT, EDGE_COMMENT_REPLY, c1.node_id, c2.node_id)

        replies = await store.get_edges_from(TENANT, c1.node_id, edge_type_id=EDGE_COMMENT_REPLY)
        assert len(replies) == 1
        assert replies[0].to_node_id == "c2"

    @pytest.mark.asyncio
    async def test_get_all_comments_for_post(self, store):
        post = await store.create_node(
            TENANT, POST_TYPE, {"title": "Popular"}, "user:1", node_id="pop"
        )
        for i in range(5):
            c = await store.create_node(
                TENANT, COMMENT_TYPE, {"text": f"Comment {i}"}, f"user:{i + 2}"
            )
            await store.create_edge(TENANT, EDGE_POST_COMMENT, post.node_id, c.node_id)

        edges = await store.get_edges_from(TENANT, "pop", edge_type_id=EDGE_POST_COMMENT)
        assert len(edges) == 5

    @pytest.mark.asyncio
    async def test_delete_comment_removes_reply_edges(self, store):
        c1 = await store.create_node(
            TENANT, COMMENT_TYPE, {"text": "Parent"}, "user:1", node_id="dc1"
        )
        c2 = await store.create_node(
            TENANT, COMMENT_TYPE, {"text": "Reply"}, "user:2", node_id="dc2"
        )
        await store.create_edge(TENANT, EDGE_COMMENT_REPLY, c1.node_id, c2.node_id)
        await store.delete_node(TENANT, c1.node_id)
        edges = await store.get_edges_from(TENANT, "dc1", edge_type_id=EDGE_COMMENT_REPLY)
        assert len(edges) == 0

    @pytest.mark.asyncio
    async def test_update_comment_text(self, store):
        c = await store.create_node(TENANT, COMMENT_TYPE, {"text": "Original"}, "user:1")
        updated = await store.update_node(TENANT, c.node_id, {"text": "Edited"})
        assert updated.payload["text"] == "Edited"

    @pytest.mark.asyncio
    async def test_multiple_comments_same_post(self, store):
        post = await store.create_node(
            TENANT, POST_TYPE, {"title": "Multi"}, "user:1", node_id="multi-p"
        )
        comment_ids = []
        for i in range(10):
            c = await store.create_node(TENANT, COMMENT_TYPE, {"text": f"C{i}"}, "user:1")
            await store.create_edge(TENANT, EDGE_POST_COMMENT, post.node_id, c.node_id)
            comment_ids.append(c.node_id)
        edges = await store.get_edges_from(TENANT, "multi-p", edge_type_id=EDGE_POST_COMMENT)
        assert len(edges) == 10
        edge_targets = {e.to_node_id for e in edges}
        assert edge_targets == set(comment_ids)

    @pytest.mark.asyncio
    async def test_comment_by_different_users(self, store):
        post = await store.create_node(
            TENANT, POST_TYPE, {"title": "Open"}, "user:1", node_id="open-p"
        )
        for uid in range(1, 6):
            c = await store.create_node(
                TENANT, COMMENT_TYPE, {"text": f"From user {uid}"}, f"user:{uid}"
            )
            await store.create_edge(TENANT, EDGE_POST_COMMENT, post.node_id, c.node_id)
        edges = await store.get_edges_from(TENANT, "open-p", edge_type_id=EDGE_POST_COMMENT)
        assert len(edges) == 5

    @pytest.mark.asyncio
    async def test_deeply_nested_thread_5_levels(self, store):
        prev_id = None
        for level in range(5):
            c = await store.create_node(
                TENANT,
                COMMENT_TYPE,
                {"text": f"Level {level}", "depth": level},
                "user:1",
                node_id=f"nest-{level}",
            )
            if prev_id is not None:
                await store.create_edge(TENANT, EDGE_COMMENT_REPLY, prev_id, c.node_id)
            prev_id = c.node_id

        # Verify chain
        for level in range(4):
            edges = await store.get_edges_from(
                TENANT, f"nest-{level}", edge_type_id=EDGE_COMMENT_REPLY
            )
            assert len(edges) == 1
            assert edges[0].to_node_id == f"nest-{level + 1}"

        # Last node has no outgoing replies
        edges = await store.get_edges_from(TENANT, "nest-4", edge_type_id=EDGE_COMMENT_REPLY)
        assert len(edges) == 0

    @pytest.mark.asyncio
    async def test_comment_ordering(self, store):
        for i in range(5):
            await store.create_node(
                TENANT, COMMENT_TYPE, {"text": f"C{i}"}, "user:1", created_at=2000 + i
            )
        comments = await store.get_nodes_by_type(TENANT, COMMENT_TYPE)
        timestamps = [c.created_at for c in comments]
        assert timestamps == sorted(timestamps, reverse=True)

    @pytest.mark.asyncio
    async def test_delete_parent_doesnt_delete_child_nodes(self, store):
        c1 = await store.create_node(
            TENANT, COMMENT_TYPE, {"text": "Parent"}, "user:1", node_id="p-del"
        )
        c2 = await store.create_node(
            TENANT, COMMENT_TYPE, {"text": "Child"}, "user:2", node_id="c-keep"
        )
        await store.create_edge(TENANT, EDGE_COMMENT_REPLY, c1.node_id, c2.node_id)
        await store.delete_node(TENANT, c1.node_id)
        # Edge gone, but child node still exists
        child = await store.get_node(TENANT, "c-keep")
        assert child is not None
        assert child.payload["text"] == "Child"

    @pytest.mark.asyncio
    async def test_comment_on_nonexistent_post_edge_still_works(self, store):
        c = await store.create_node(TENANT, COMMENT_TYPE, {"text": "Orphan"}, "user:1")
        edge = await store.create_edge(TENANT, EDGE_POST_COMMENT, "no-such-post", c.node_id)
        assert edge.from_node_id == "no-such-post"
        edges = await store.get_edges_from(TENANT, "no-such-post", edge_type_id=EDGE_POST_COMMENT)
        assert len(edges) == 1

    @pytest.mark.asyncio
    async def test_delete_all_comments_leaves_post(self, store):
        post = await store.create_node(
            TENANT, POST_TYPE, {"title": "Survive"}, "user:1", node_id="survive-p"
        )
        cids = []
        for i in range(3):
            c = await store.create_node(
                TENANT, COMMENT_TYPE, {"text": f"Del {i}"}, "user:1", node_id=f"del-c-{i}"
            )
            await store.create_edge(TENANT, EDGE_POST_COMMENT, post.node_id, c.node_id)
            cids.append(c.node_id)
        for cid in cids:
            await store.delete_node(TENANT, cid)
        remaining = await store.get_node(TENANT, "survive-p")
        assert remaining is not None
        edges = await store.get_edges_from(TENANT, "survive-p", edge_type_id=EDGE_POST_COMMENT)
        assert len(edges) == 0

    @pytest.mark.asyncio
    async def test_get_comments_via_edges_to(self, store):
        c = await store.create_node(
            TENANT, COMMENT_TYPE, {"text": "Target"}, "user:1", node_id="target-c"
        )
        for i in range(3):
            await store.create_edge(TENANT, EDGE_COMMENT_REPLY, f"parent-{i}", c.node_id)
        incoming = await store.get_edges_to(TENANT, "target-c", edge_type_id=EDGE_COMMENT_REPLY)
        assert len(incoming) == 3

    @pytest.mark.asyncio
    async def test_comment_with_score_update(self, store):
        c = await store.create_node(TENANT, COMMENT_TYPE, {"text": "Scored", "score": 0}, "user:1")
        for val in [1, 5, 10, 25]:
            await store.update_node(TENANT, c.node_id, {"score": val})
        fetched = await store.get_node(TENANT, c.node_id)
        assert fetched.payload["score"] == 25


# =========================================================================
# VOTING SYSTEM
# =========================================================================


@pytest.mark.integration
class TestVotingSystem:
    @pytest.mark.asyncio
    async def test_upvote_post(self, store):
        post = await store.create_node(
            TENANT, POST_TYPE, {"title": "Upvotable", "score": 0}, "user:1", node_id="uv-post"
        )
        edge = await store.create_edge(
            TENANT, EDGE_USER_UPVOTE, "user:2", post.node_id, props={"vote_type": "up"}
        )
        assert edge.props["vote_type"] == "up"

    @pytest.mark.asyncio
    async def test_downvote_different_edge_type(self, store):
        post = await store.create_node(
            TENANT, POST_TYPE, {"title": "Downvotable"}, "user:1", node_id="dv-post"
        )
        # Use edge_type=11 with vote_type prop to distinguish
        edge = await store.create_edge(
            TENANT, EDGE_USER_UPVOTE, "user:2", post.node_id, props={"vote_type": "down"}
        )
        assert edge.props["vote_type"] == "down"

    @pytest.mark.asyncio
    async def test_remove_vote(self, store):
        post = await store.create_node(
            TENANT, POST_TYPE, {"title": "Unvote"}, "user:1", node_id="rm-vote"
        )
        await store.create_edge(TENANT, EDGE_USER_UPVOTE, "user:2", post.node_id)
        result = await store.delete_edge(TENANT, EDGE_USER_UPVOTE, "user:2", "rm-vote")
        assert result is True
        edges = await store.get_edges_to(TENANT, "rm-vote", edge_type_id=EDGE_USER_UPVOTE)
        assert len(edges) == 0

    @pytest.mark.asyncio
    async def test_count_votes_via_edges_to(self, store):
        post = await store.create_node(
            TENANT, POST_TYPE, {"title": "Popular"}, "user:1", node_id="cnt-post"
        )
        for i in range(7):
            await store.create_edge(TENANT, EDGE_USER_UPVOTE, f"user:{i + 10}", post.node_id)
        edges = await store.get_edges_to(TENANT, "cnt-post", edge_type_id=EDGE_USER_UPVOTE)
        assert len(edges) == 7

    @pytest.mark.asyncio
    async def test_change_vote(self, store):
        post = await store.create_node(
            TENANT, POST_TYPE, {"title": "Change"}, "user:1", node_id="chg-post"
        )
        await store.create_edge(
            TENANT, EDGE_USER_UPVOTE, "user:2", post.node_id, props={"vote_type": "up"}
        )
        await store.delete_edge(TENANT, EDGE_USER_UPVOTE, "user:2", "chg-post")
        await store.create_edge(
            TENANT, EDGE_USER_UPVOTE, "user:2", post.node_id, props={"vote_type": "down"}
        )
        edges = await store.get_edges_to(TENANT, "chg-post", edge_type_id=EDGE_USER_UPVOTE)
        assert len(edges) == 1
        assert edges[0].props["vote_type"] == "down"

    @pytest.mark.asyncio
    async def test_user_votes_on_multiple_posts(self, store):
        for i in range(5):
            await store.create_node(
                TENANT, POST_TYPE, {"title": f"P{i}"}, "user:1", node_id=f"vp-{i}"
            )
            await store.create_edge(TENANT, EDGE_USER_UPVOTE, "user:voter", f"vp-{i}")
        edges = await store.get_edges_from(TENANT, "user:voter", edge_type_id=EDGE_USER_UPVOTE)
        assert len(edges) == 5

    @pytest.mark.asyncio
    async def test_multiple_users_vote_same_post(self, store):
        await store.create_node(
            TENANT, POST_TYPE, {"title": "Mass vote"}, "user:1", node_id="mass-v"
        )
        for i in range(20):
            await store.create_edge(TENANT, EDGE_USER_UPVOTE, f"user:{i}", "mass-v")
        edges = await store.get_edges_to(TENANT, "mass-v", edge_type_id=EDGE_USER_UPVOTE)
        assert len(edges) == 20

    @pytest.mark.asyncio
    async def test_vote_on_comment(self, store):
        c = await store.create_node(
            TENANT, COMMENT_TYPE, {"text": "Voteable comment"}, "user:1", node_id="vc"
        )
        await store.create_edge(
            TENANT, EDGE_USER_UPVOTE, "user:2", c.node_id, props={"vote_type": "up"}
        )
        edges = await store.get_edges_to(TENANT, "vc", edge_type_id=EDGE_USER_UPVOTE)
        assert len(edges) == 1

    @pytest.mark.asyncio
    async def test_double_vote_replaces_edge(self, store):
        await store.create_node(TENANT, POST_TYPE, {"title": "Double"}, "user:1", node_id="dbl")
        await store.create_edge(
            TENANT, EDGE_USER_UPVOTE, "user:2", "dbl", props={"vote_type": "up"}
        )
        # INSERT OR REPLACE semantics means same PK replaces
        await store.create_edge(
            TENANT, EDGE_USER_UPVOTE, "user:2", "dbl", props={"vote_type": "down"}
        )
        edges = await store.get_edges_to(TENANT, "dbl", edge_type_id=EDGE_USER_UPVOTE)
        assert len(edges) == 1
        assert edges[0].props["vote_type"] == "down"

    @pytest.mark.asyncio
    async def test_vote_doesnt_affect_post_payload(self, store):
        await store.create_node(
            TENANT, POST_TYPE, {"title": "Unchanged", "score": 0}, "user:1", node_id="no-fx"
        )
        await store.create_edge(TENANT, EDGE_USER_UPVOTE, "user:2", "no-fx")
        fetched = await store.get_node(TENANT, "no-fx")
        assert fetched.payload["score"] == 0

    @pytest.mark.asyncio
    async def test_bulk_voting_10_votes(self, store):
        await store.create_node(TENANT, POST_TYPE, {"title": "Bulk"}, "user:1", node_id="bulk-v")
        for i in range(10):
            await store.create_edge(
                TENANT, EDGE_USER_UPVOTE, f"voter:{i}", "bulk-v", props={"vote_type": "up"}
            )
        edges = await store.get_edges_to(TENANT, "bulk-v", edge_type_id=EDGE_USER_UPVOTE)
        assert len(edges) == 10
        assert all(e.props["vote_type"] == "up" for e in edges)

    @pytest.mark.asyncio
    async def test_remove_nonexistent_vote(self, store):
        result = await store.delete_edge(TENANT, EDGE_USER_UPVOTE, "user:ghost", "no-post")
        assert result is False

    @pytest.mark.asyncio
    async def test_vote_edge_has_timestamp(self, store):
        await store.create_node(TENANT, POST_TYPE, {"title": "Timed"}, "user:1", node_id="timed-v")
        before = int(time.time() * 1000)
        edge = await store.create_edge(TENANT, EDGE_USER_UPVOTE, "user:2", "timed-v")
        after = int(time.time() * 1000)
        assert before <= edge.created_at <= after

    @pytest.mark.asyncio
    async def test_vote_with_custom_props(self, store):
        await store.create_node(
            TENANT, POST_TYPE, {"title": "Rich vote"}, "user:1", node_id="rich-v"
        )
        edge = await store.create_edge(
            TENANT,
            EDGE_USER_UPVOTE,
            "user:2",
            "rich-v",
            props={"vote_type": "up", "reaction": "heart", "source": "mobile"},
        )
        assert edge.props["reaction"] == "heart"
        assert edge.props["source"] == "mobile"

    @pytest.mark.asyncio
    async def test_votes_isolated_by_edge_type(self, store):
        await store.create_node(
            TENANT, POST_TYPE, {"title": "Multi-edge"}, "user:1", node_id="me-post"
        )
        await store.create_edge(TENANT, EDGE_USER_UPVOTE, "user:2", "me-post")
        await store.create_edge(TENANT, EDGE_COMMENT_REPLY, "user:2", "me-post")
        upvotes = await store.get_edges_to(TENANT, "me-post", edge_type_id=EDGE_USER_UPVOTE)
        replies = await store.get_edges_to(TENANT, "me-post", edge_type_id=EDGE_COMMENT_REPLY)
        assert len(upvotes) == 1
        assert len(replies) == 1


# =========================================================================
# FEED GENERATION
# =========================================================================


@pytest.mark.integration
class TestFeedGeneration:
    @pytest.mark.asyncio
    async def test_get_posts_with_pagination(self, store):
        for i in range(30):
            await store.create_node(
                TENANT, POST_TYPE, {"title": f"Feed {i}", "subreddit": "all"}, "user:1"
            )
        page1 = await store.get_nodes_by_type(TENANT, POST_TYPE, limit=10, offset=0)
        page2 = await store.get_nodes_by_type(TENANT, POST_TYPE, limit=10, offset=10)
        assert len(page1) == 10
        assert len(page2) == 10
        ids1 = {p.node_id for p in page1}
        ids2 = {p.node_id for p in page2}
        assert ids1.isdisjoint(ids2)

    @pytest.mark.asyncio
    async def test_posts_ordered_by_newest(self, store):
        for i in range(10):
            await store.create_node(
                TENANT, POST_TYPE, {"title": f"Ordered {i}"}, "user:1", created_at=5000 + i
            )
        posts = await store.get_nodes_by_type(TENANT, POST_TYPE, limit=10)
        assert posts[0].created_at > posts[-1].created_at

    @pytest.mark.asyncio
    async def test_posts_for_specific_subreddit(self, store):
        for i in range(5):
            await store.create_node(
                TENANT, POST_TYPE, {"title": f"Sci {i}", "subreddit": "science"}, "user:1"
            )
        for i in range(3):
            await store.create_node(
                TENANT, POST_TYPE, {"title": f"Art {i}", "subreddit": "art"}, "user:1"
            )
        all_posts = await store.get_nodes_by_type(TENANT, POST_TYPE)
        science = [p for p in all_posts if p.payload.get("subreddit") == "science"]
        assert len(science) == 5

    @pytest.mark.asyncio
    async def test_mixed_content_types_in_feed(self, store):
        await store.create_node(TENANT, POST_TYPE, {"title": "Post"}, "user:1")
        await store.create_node(TENANT, COMMENT_TYPE, {"text": "Comment"}, "user:1")
        await store.create_node(TENANT, VOTE_TYPE, {"vote": "up"}, "user:1")
        posts = await store.get_nodes_by_type(TENANT, POST_TYPE)
        comments = await store.get_nodes_by_type(TENANT, COMMENT_TYPE)
        assert len(posts) == 1
        assert len(comments) == 1

    @pytest.mark.asyncio
    async def test_pagination_with_offset(self, store):
        for i in range(25):
            await store.create_node(
                TENANT, POST_TYPE, {"title": f"Off {i}"}, "user:1", created_at=10000 + i
            )
        page = await store.get_nodes_by_type(TENANT, POST_TYPE, limit=5, offset=20)
        assert len(page) == 5

    @pytest.mark.asyncio
    async def test_empty_feed(self, store):
        posts = await store.get_nodes_by_type(TENANT, POST_TYPE)
        assert posts == []

    @pytest.mark.asyncio
    async def test_feed_with_exactly_limit_items(self, store):
        for i in range(10):
            await store.create_node(TENANT, POST_TYPE, {"title": f"Exact {i}"}, "user:1")
        posts = await store.get_nodes_by_type(TENANT, POST_TYPE, limit=10)
        assert len(posts) == 10

    @pytest.mark.asyncio
    async def test_large_feed_100_posts(self, store):
        for i in range(100):
            await store.create_node(
                TENANT, POST_TYPE, {"title": f"Large {i}", "subreddit": "bulk"}, "user:1"
            )
        all_posts = await store.get_nodes_by_type(TENANT, POST_TYPE, limit=100)
        assert len(all_posts) == 100

    @pytest.mark.asyncio
    async def test_feed_beyond_total(self, store):
        for i in range(5):
            await store.create_node(TENANT, POST_TYPE, {"title": f"Short {i}"}, "user:1")
        page = await store.get_nodes_by_type(TENANT, POST_TYPE, limit=10, offset=5)
        assert len(page) == 0

    @pytest.mark.asyncio
    async def test_feed_partial_last_page(self, store):
        for i in range(13):
            await store.create_node(TENANT, POST_TYPE, {"title": f"Partial {i}"}, "user:1")
        page = await store.get_nodes_by_type(TENANT, POST_TYPE, limit=10, offset=10)
        assert len(page) == 3


# =========================================================================
# CONTENT MODERATION
# =========================================================================


@pytest.mark.integration
class TestContentModeration:
    @pytest.mark.asyncio
    async def test_delete_post_removes_all_vote_edges(self, store):
        post = await store.create_node(
            TENANT, POST_TYPE, {"title": "Moderated"}, "user:1", node_id="mod-p"
        )
        for i in range(5):
            await store.create_edge(TENANT, EDGE_USER_UPVOTE, f"user:{i}", post.node_id)
        await store.delete_node(TENANT, "mod-p")
        edges = await store.get_edges_to(TENANT, "mod-p", edge_type_id=EDGE_USER_UPVOTE)
        assert len(edges) == 0

    @pytest.mark.asyncio
    async def test_delete_post_removes_all_comment_edges(self, store):
        post = await store.create_node(
            TENANT, POST_TYPE, {"title": "With comments"}, "user:1", node_id="com-del"
        )
        for i in range(3):
            c = await store.create_node(
                TENANT, COMMENT_TYPE, {"text": f"C{i}"}, "user:2", node_id=f"com-c-{i}"
            )
            await store.create_edge(TENANT, EDGE_POST_COMMENT, post.node_id, c.node_id)
        await store.delete_node(TENANT, "com-del")
        edges = await store.get_edges_from(TENANT, "com-del", edge_type_id=EDGE_POST_COMMENT)
        assert len(edges) == 0

    @pytest.mark.asyncio
    async def test_update_post_to_mark_removed(self, store):
        post = await store.create_node(
            TENANT, POST_TYPE, {"title": "Bad post", "removed": False}, "user:1"
        )
        updated = await store.update_node(TENANT, post.node_id, {"removed": True})
        assert updated.payload["removed"] is True
        assert updated.payload["title"] == "Bad post"

    @pytest.mark.asyncio
    async def test_hidden_post_still_exists(self, store):
        post = await store.create_node(
            TENANT,
            POST_TYPE,
            {"title": "Hidden", "visible": False, "removed_by": "mod:1"},
            "user:1",
        )
        fetched = await store.get_node(TENANT, post.node_id)
        assert fetched is not None
        assert fetched.payload["visible"] is False

    @pytest.mark.asyncio
    async def test_bulk_delete_multiple_posts(self, store):
        pids = []
        for i in range(5):
            p = await store.create_node(TENANT, POST_TYPE, {"title": f"Bulk del {i}"}, "user:1")
            pids.append(p.node_id)
        for pid in pids:
            await store.delete_node(TENANT, pid)
        for pid in pids:
            assert await store.get_node(TENANT, pid) is None

    @pytest.mark.asyncio
    async def test_moderator_creates_post(self, store):
        post = await store.create_node(
            TENANT,
            POST_TYPE,
            {"title": "Official", "is_announcement": True},
            "mod:admin",
        )
        assert post.owner_actor == "mod:admin"
        assert post.payload["is_announcement"] is True

    @pytest.mark.asyncio
    async def test_post_by_different_actor(self, store):
        p1 = await store.create_node(TENANT, POST_TYPE, {"title": "User post"}, "user:banned")
        p2 = await store.create_node(TENANT, POST_TYPE, {"title": "Mod post"}, "mod:1")
        assert p1.owner_actor == "user:banned"
        assert p2.owner_actor == "mod:1"

    @pytest.mark.asyncio
    async def test_cross_subreddit_isolation(self, store):
        for i in range(4):
            await store.create_node(
                TENANT,
                POST_TYPE,
                {"title": f"CS {i}", "subreddit": "cs"},
                "user:1",
            )
        for i in range(6):
            await store.create_node(
                TENANT,
                POST_TYPE,
                {"title": f"EE {i}", "subreddit": "ee"},
                "user:1",
            )
        all_posts = await store.get_nodes_by_type(TENANT, POST_TYPE)
        cs = [p for p in all_posts if p.payload["subreddit"] == "cs"]
        ee = [p for p in all_posts if p.payload["subreddit"] == "ee"]
        assert len(cs) == 4
        assert len(ee) == 6

    @pytest.mark.asyncio
    async def test_delete_post_then_recreate_same_id(self, store):
        await store.create_node(TENANT, POST_TYPE, {"title": "V1"}, "user:1", node_id="recycle")
        await store.delete_node(TENANT, "recycle")
        p2 = await store.create_node(
            TENANT, POST_TYPE, {"title": "V2"}, "user:1", node_id="recycle"
        )
        assert p2.payload["title"] == "V2"

    @pytest.mark.asyncio
    async def test_moderation_preserves_other_posts(self, store):
        await store.create_node(TENANT, POST_TYPE, {"title": "Good"}, "user:1", node_id="good-p")
        await store.create_node(TENANT, POST_TYPE, {"title": "Bad"}, "user:1", node_id="bad-p")
        await store.delete_node(TENANT, "bad-p")
        good = await store.get_node(TENANT, "good-p")
        assert good is not None
        assert good.payload["title"] == "Good"


# =========================================================================
# EDGE CASES
# =========================================================================


@pytest.mark.integration
class TestPostEdgeCases:
    @pytest.mark.asyncio
    async def test_post_with_null_like_values(self, store):
        post = await store.create_node(
            TENANT, POST_TYPE, {"title": "Null-ish", "body": None, "score": 0}, "user:1"
        )
        fetched = await store.get_node(TENANT, post.node_id)
        assert fetched.payload["body"] is None

    @pytest.mark.asyncio
    async def test_post_with_numeric_string_title(self, store):
        post = await store.create_node(TENANT, POST_TYPE, {"title": "12345"}, "user:1")
        fetched = await store.get_node(TENANT, post.node_id)
        assert fetched.payload["title"] == "12345"
        assert isinstance(fetched.payload["title"], str)

    @pytest.mark.asyncio
    async def test_post_with_boolean_fields(self, store):
        post = await store.create_node(
            TENANT,
            POST_TYPE,
            {"title": "Bools", "is_nsfw": True, "is_spoiler": False},
            "user:1",
        )
        fetched = await store.get_node(TENANT, post.node_id)
        assert fetched.payload["is_nsfw"] is True
        assert fetched.payload["is_spoiler"] is False

    @pytest.mark.asyncio
    async def test_post_with_list_in_payload(self, store):
        post = await store.create_node(
            TENANT,
            POST_TYPE,
            {"title": "Tags", "tags": ["python", "rust", "go"]},
            "user:1",
        )
        fetched = await store.get_node(TENANT, post.node_id)
        assert fetched.payload["tags"] == ["python", "rust", "go"]

    @pytest.mark.asyncio
    async def test_post_with_deeply_nested_json(self, store):
        nested = {"l1": {"l2": {"l3": {"l4": {"l5": "deep"}}}}}
        post = await store.create_node(
            TENANT, POST_TYPE, {"title": "Deep", "metadata": nested}, "user:1"
        )
        fetched = await store.get_node(TENANT, post.node_id)
        assert fetched.payload["metadata"]["l1"]["l2"]["l3"]["l4"]["l5"] == "deep"

    @pytest.mark.asyncio
    async def test_concurrent_creation_50_posts(self, store):
        async def create_post(idx):
            return await store.create_node(
                TENANT, POST_TYPE, {"title": f"Concurrent {idx}"}, "user:1"
            )

        tasks = [create_post(i) for i in range(50)]
        results = await asyncio.gather(*tasks)
        assert len(results) == 50
        ids = {r.node_id for r in results}
        assert len(ids) == 50

    @pytest.mark.asyncio
    async def test_update_payload_to_empty_object(self, store):
        post = await store.create_node(TENANT, POST_TYPE, {"title": "Will be empty"}, "user:1")
        # Merge with empty dict = no change
        updated = await store.update_node(TENANT, post.node_id, {})
        assert updated.payload["title"] == "Will be empty"

    @pytest.mark.asyncio
    async def test_update_then_delete(self, store):
        post = await store.create_node(TENANT, POST_TYPE, {"title": "Short lived"}, "user:1")
        await store.update_node(TENANT, post.node_id, {"title": "Updated"})
        await store.delete_node(TENANT, post.node_id)
        assert await store.get_node(TENANT, post.node_id) is None

    @pytest.mark.asyncio
    async def test_create_read_consistency(self, store):
        post = await store.create_node(
            TENANT,
            POST_TYPE,
            {"title": "Consistent", "body": "Data"},
            "user:1",
        )
        fetched = await store.get_node(TENANT, post.node_id)
        assert fetched.node_id == post.node_id
        assert fetched.type_id == post.type_id
        assert fetched.payload == post.payload
        assert fetched.owner_actor == post.owner_actor
        assert fetched.created_at == post.created_at

    @pytest.mark.asyncio
    async def test_post_with_timestamp_in_the_past(self, store):
        post = await store.create_node(
            TENANT, POST_TYPE, {"title": "Ancient"}, "user:1", created_at=1000
        )
        assert post.created_at == 1000
        fetched = await store.get_node(TENANT, post.node_id)
        assert fetched.created_at == 1000

    @pytest.mark.asyncio
    async def test_post_with_timestamp_zero(self, store):
        post = await store.create_node(
            TENANT, POST_TYPE, {"title": "Epoch"}, "user:1", created_at=0
        )
        # created_at=0 is falsy, so the store uses current time
        assert post.created_at > 0

    @pytest.mark.asyncio
    async def test_post_with_max_int_timestamp(self, store):
        big_ts = 2**53
        post = await store.create_node(
            TENANT, POST_TYPE, {"title": "Far future"}, "user:1", created_at=big_ts
        )
        assert post.created_at == big_ts

    @pytest.mark.asyncio
    async def test_payload_with_special_json_chars(self, store):
        payload = {
            "title": 'Quotes "and" more',
            "body": "Back\\slash\nnewline\ttab",
            "raw": '{"nested": "json string"}',
        }
        post = await store.create_node(TENANT, POST_TYPE, payload, "user:1")
        fetched = await store.get_node(TENANT, post.node_id)
        assert fetched.payload["title"] == 'Quotes "and" more'
        assert "\n" in fetched.payload["body"]
        assert "\t" in fetched.payload["body"]

    @pytest.mark.asyncio
    async def test_post_with_very_long_node_id(self, store):
        long_id = "p-" + "a" * 500
        await store.create_node(TENANT, POST_TYPE, {"title": "Long ID"}, "user:1", node_id=long_id)
        fetched = await store.get_node(TENANT, long_id)
        assert fetched is not None
        assert fetched.node_id == long_id

    @pytest.mark.asyncio
    async def test_post_with_empty_string_fields(self, store):
        post = await store.create_node(
            TENANT,
            POST_TYPE,
            {"title": "", "body": "", "subreddit": ""},
            "user:1",
        )
        fetched = await store.get_node(TENANT, post.node_id)
        assert fetched.payload["title"] == ""
        assert fetched.payload["body"] == ""

    @pytest.mark.asyncio
    async def test_update_nonexistent_post_returns_none(self, store):
        result = await store.update_node(TENANT, "ghost-post-id", {"title": "nope"})
        assert result is None
