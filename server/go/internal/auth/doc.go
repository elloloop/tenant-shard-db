// Package auth will host the gRPC auth interceptor that produces a
// trusted Actor on the request context, ported from
// server/python/entdb_server/auth/auth_interceptor.py. CLAUDE.md
// invariant: request.actor / request.context.actor on the wire is
// UNTRUSTED — verified identity comes from the interceptor.
// tests/python/integration/test_privilege_escalation.py is the
// behavioural contract.
package auth
