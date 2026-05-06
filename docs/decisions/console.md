# Console — Frozen Decisions

Decisions about EntDB's developer-facing web console (data browser, schema viewer, sandbox writer). Newest first.

---

## 2026-05-06: Single Go binary `entdb-console` with curated Connect proto, replacing FastAPI console + playground

**Status:** frozen
**Decided:** 2026-05-06
**Tags:** console, api, security, build, architecture, llm-safety
**Supersedes:** none
**Superseded by:** —

### Decision

The EntDB developer console is a **single Go binary** named `entdb-console` that:

1. Speaks **Connect / gRPC / gRPC-Web** on one HTTP port via `connectrpc.com/connect`. The browser uses `@connectrpc/connect-web`.
2. Embeds the **React SPA** into the binary via `//go:embed`. No separate frontend container, no separate static-file server.
3. Exposes a **curated `Console` service** defined in `proto/console/v1/console.proto`. **The proto file is the literal access policy** — the browser cannot call any RPC not declared in `console.proto`, because the Connect handler has no route for it.
4. Each handler is a typed **pass-through proxy** to the upstream `EntDBService` gRPC server. Console request/response message types are imported from `entdb.proto` via proto `import`, so handlers are one-line forwards (`return s.upstream.X(ctx, req.Msg)`), not hand-written translations.
5. Uses **`buf`** to drive codegen for both Go (`protoc-gen-go` + `protoc-gen-connect-go`) and TypeScript (`@bufbuild/protoc-gen-es` + `@connectrpc/protoc-gen-connect-es`) from the same `console.proto`.
6. Enforces "proto = access policy" at **compile time** via `var _ consolev1connect.ConsoleHandler = (*consoleServer)(nil)`. Adding an RPC to the proto without implementing it on `consoleServer` is a build failure; removing one without removing the handler is a build failure.
7. Authenticates with the upstream gRPC server via **API key in Bearer header**, same pattern as the Go SDK and `entdbctl` CLI. No OAuth, no session cookies.

The existing FastAPI services `console/` (read-only browser) and `playground/` (write-only sandbox) are deleted in a phased sequence:

- **PR 1** — `entdb-console` binary with read-only RPCs (10 RPCs: `Health`, `ListTenants`, `GetTenant`, `GetSchema`, `QueryNodes`, `GetNode`, `GetEdgesFrom`, `GetEdgesTo`, `GetConnectedNodes`, `SearchMailbox`). Frontend pages port the existing console UI. **Additive only — nothing deleted.** (PR #163, in flight at decision time.)
- **PR 2** — Add sandbox write RPCs (`SandboxCreateNode`, `SandboxCreateEdge`) to the curated proto, gated to the configured sandbox tenant. Replaces playground functionally. **Additive only.**
- **PR 3** — Delete `console/`, `playground/`, both Docker stages, both compose services, both `requirements.txt` files, all `CONSOLE_*` and `PLAYGROUND_*` env vars. **Pure deletion.**

### Context

EntDB shipped two parallel FastAPI services:

- `console/gateway/` — read-only data browser, ~1k LOC of hand-written FastAPI routes that translate JSON/REST into gRPC calls against `EntDBService` and proto responses back into JSON. Includes per-tenant schema fetch + id-keyed → name-keyed payload translation done server-side.
- `playground/` — write-only sandbox, similar shape but smaller, hardcoded `playground` tenant and a fixed schema, used for "create some data and see it in console."

Three problems with this shape:

1. **Hand-written drift.** PR #155 fixed a bug in `console_client.py` where it accessed reserved proto field names (`payload_json`, `schema_json`, `props_json`, `state_json`) that had been deleted from the proto — the gateway was broken at runtime and nobody noticed because it was a hand-translated layer with no compile-time check that it matched the proto. The same bug class would recur every time the proto changes meaningfully.
2. **Two services for one tool.** Console (read) + playground (write) split is an artifact of how they were built, not a useful product split. Both are FastAPI, both have React frontends, share zero code, and the user actually wants them as one tool.
3. **LLM safety.** With LLM-assisted dev, hand-written REST endpoints become a hallucination surface — an LLM editing the frontend can invent a URL or field name that compiles and runs locally but doesn't actually exist. Codegen'd Connect-Web TS client physically cannot have methods we don't expose; codegen is the type-system enforcement we want.

The user explicitly identified the dev console as **for developers debugging local environment and prod during difficult times**, with usage already declining as LLMs route more queries through the SDK and CLI directly. So the bar isn't "build a polished customer UI" — it's "have a small, debug-only web tool that doesn't drift from the proto and can't accidentally expose dangerous operations."

### Alternatives considered

- **Keep FastAPI, just merge console + playground.** Rejected — doesn't address the proto-drift bug class, and the user explicitly said they want FastAPI removed.
- **Re-export `EntDBService` directly through Connect (no curated proto).** Rejected — gives the browser ~30+ RPCs including `DeleteUser`, `FreezeUser`, `RevokeAllUserAccess`, `TransferUserContent`, GDPR exports, admin tenant management. Even if the UI doesn't show buttons, the methods are callable from any DevTools console. To filter, would need a Connect interceptor with a hand-maintained allow-list — same drift problem we're trying to escape.
- **gRPC-Web via grpcwebproxy/envoy sidecar + unchanged Python server.** Considered — would have removed FastAPI without touching the server. Rejected because (a) the user wants a single binary with embedded frontend ("not a separate container"), and (b) the curated-proto security boundary is harder to enforce when the proxy is generic pass-through.
- **ConnectRPC server-side in the EntDB server itself (replace `grpc.aio` with `connecpy`).** Rejected — Python ConnectRPC support (`connecpy`) is less battle-tested than the Go implementation, and migrating every interceptor / auth flow / health registration in the existing `grpc.aio` server is a much bigger change than the user is asking for. The Connect protocol is only valuable on the *console-facing* boundary; the entdb-server's gRPC interface stays cluster-internal.
- **Just delete the console entirely (no replacement).** Considered — `entdbctl` (Go CLI, PR #162) covers most read use cases; SDK in a Python REPL covers writes. Rejected because graph visualization + a tabbed SPA workflow is genuinely useful for prod debugging, and at "single Go binary" cost the maintenance is small enough to justify keeping a visual tool.
- **chi router instead of stdlib `net/http`.** Rejected — Go 1.22+'s `http.ServeMux` does path patterns natively and the SDK is dep-light; not worth a router dep.
- **Hand-rolled `protoc` invocations instead of `buf`.** Rejected — `buf` is industry standard, drives Go + TS from one config, gives free breaking-change linting, and is already used widely. Trade-off: one build-time CLI dep, worth it.

### Consequences

**What this locks in:**

- The console's API surface is exactly `proto/console/v1/console.proto`. Any new browser-callable capability requires a proto edit (and code review of that edit). New `EntDBService` RPCs do NOT auto-appear in the console.
- The frontend's TS Connect client is generated, never hand-written. Frontend code that references undefined RPCs or fields is a TypeScript compile error.
- Console and playground frontends merge into a single React SPA. No more parallel UIs.
- Single binary, single Docker stage, single compose service. Operationally simpler than two FastAPI services + two frontend builds.
- Auth model is API-key Bearer only. No OAuth flows, no session management. If we ever need user-scoped sessions, that's a new decision.
- Sandbox writes are gated to a single configured sandbox tenant. The console binary cannot perform writes against a real tenant by design.

**What this makes easy:**

- Adding a read RPC = add to `console.proto`, regenerate, write a 4-line forward, port a React page.
- Removing a capability = delete from `console.proto` and re-generate. Compile fails until handler + frontend are also cleaned up.
- Auditing what the browser can do = read one proto file.

**What this makes hard (intentionally):**

- Quickly exposing a new capability to the browser without a proto-level review. That's the security boundary working as designed.
- Writing arbitrary cross-RPC client logic in the frontend (no transactions, no batching beyond what individual RPCs support). Acceptable for a debug tool.
- Running the console without the upstream EntDB server reachable. Smoke endpoints (`/`, SPA fallback) work, but RPC handlers return Connect `unavailable` until upstream is live. Acceptable.

**Tradeoffs accepted:**

- ~50 lines of pass-through handler code across 10 RPCs (one-line each plus error mapping). Could be templated/generated; not worth the meta-tooling at this scale.
- Frontend rewrite to Connect-Web (one-time cost). Done in PR #163.
- `buf` becomes a build-time requirement for anyone regenerating proto. CI installs it; local dev needs it for proto edits only.

### References

- Conversation thread: 2026-05-06 (this session)
- Related PRs:
  - #155 — Wire-format fix that exposed the FastAPI gateway's hand-written drift bug
  - #163 — PR 1 of this decision: `entdb-console` Go binary with read-only Connect API
  - #162 — `entdbctl` Go CLI (sibling tool, same module)
- Code (after PR #163):
  - `proto/console/v1/console.proto` — the curated service, also the access policy
  - `buf.yaml`, `buf.gen.yaml` — codegen config
  - `sdk/go/entdb/cmd/entdb-console/` — Go binary + embedded frontend
  - `sdk/go/entdb/internal/console/v1/` — generated Go
  - `sdk/go/entdb/cmd/entdb-console/frontend/src/gen/` — generated TS

---

_Powered by the [`freeze-decision` skill](../../.claude/skills/freeze-decision/SKILL.md)._
