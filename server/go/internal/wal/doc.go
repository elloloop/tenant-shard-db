// Package wal will host the write-ahead-log producer/consumer
// abstractions ported from server/python/entdb_server/wal/. The WAL is
// the source of truth for all mutations (CLAUDE.md invariant #1);
// SQLite is a materialized view rebuilt by replaying it. Concrete
// backends (Kafka, Kinesis, SQS, in-memory) land here as their RPC
// sub-issues are picked up.
package wal
