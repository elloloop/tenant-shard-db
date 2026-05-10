// Package apply will host the WAL consumer ("Applier") that reads
// TransactionEvents off the WAL and writes them to per-tenant SQLite,
// ported from server/python/entdb_server/apply/. The Python applier
// pins the determinism contract enforced by
// tests/python/integration/test_wal_replay_determinism.py — the Go
// applier must satisfy the same.
package apply
