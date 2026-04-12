# ADR-008: Notification System

## Status: Accepted

## Context

When events occur (messages, mentions, shares, assignments), users need to be notified. With 1000 users per tenant, notification fanout must not bottleneck the write path.

## Decision

### Notifications table in the tenant SQLite file (not separate files)

```sql
-- In the same tenant SQLite file as nodes and edges
CREATE TABLE notifications (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL,
    type        TEXT NOT NULL,       -- mention, dm, share, assign, reply, at_everyone
    node_id     TEXT NOT NULL,
    snippet     TEXT,
    read        INTEGER DEFAULT 0,
    created_at  INTEGER NOT NULL
);
CREATE INDEX idx_notif_user ON notifications(user_id, read, created_at DESC);

CREATE TABLE read_cursors (
    user_id      TEXT NOT NULL,
    channel_id   TEXT NOT NULL,
    last_read_at INTEGER NOT NULL,
    PRIMARY KEY (user_id, channel_id)
);
```

### Broadcast messages: no individual notifications

Messages with `tenant_visible=true` don't need per-user notification entries. Unread state is tracked via read_cursors.

```
Alice opens #general:
  1. read_cursors WHERE user_id=alice AND channel_id=general → last_read_at
  2. nodes WHERE parent=general AND created_at > last_read_at
  3. Update cursor: last_read_at = now
```

### Only targeted events create notifications

| Event | Individual notification? | Count |
|---|---|---|
| Message in channel | No (use read_cursor) | 0 |
| @mention | Yes, for mentioned user | 1 |
| @everyone | Yes, for all members | N (batched) |
| DM | Yes, for recipient | 1 |
| Share with user | Yes | 1 |
| Assignment | Yes | 1 |
| Reply to your message | Yes | 1 |

### Fanout decoupled from applier

```
Kafka topics:
  entdb-events   — tenant data (applier, synchronous)
  entdb-fanout   — notifications (fanout workers, asynchronous)

Write path:
  1. Applier writes node to tenant SQLite (fast, ~1ms)
  2. Applier emits fanout event to entdb-fanout (if needed)
  3. Applier commits Kafka offset (done, not blocked)
  4. Fanout worker writes notifications to tenant SQLite (async)
```

### @everyone batch performance

```
1000-member @everyone:
  One transaction, one file, 1000 INSERT statements
  ~5-10ms total (batch insert to single SQLite)
  
  vs. old design (per-user files):
  1000 file opens + 1000 individual writes
  ~1-2 seconds (was the bottleneck)
```

### Inbox query (unified across tenants)

```sql
-- Explicit notifications
SELECT * FROM notifications
WHERE user_id = ? AND read = 0
ORDER BY created_at DESC LIMIT 20

-- Unread channels
SELECT c.channel_id, COUNT(*) as unread
FROM read_cursors c
JOIN nodes n ON n.parent_id = c.channel_id
WHERE c.user_id = ? AND n.created_at > c.last_read_at
GROUP BY c.channel_id
```

## Consequences

- Broadcast messages have zero notification overhead
- @everyone is batched (5-10ms, not 1-2s)
- Fanout doesn't block the applier
- Single file per tenant = atomic message + notification
- read_cursors replace per-message unread tracking for channels
