# entdb-console

Single-binary Go server that:

1. Embeds a React SPA via `//go:embed frontend/dist`.
2. Exposes a curated, **read-only** Console API via ConnectRPC at
   `/entdb.console.v1.Console/<Method>` — Connect, gRPC, and gRPC-Web
   share the same path.
3. Pass-through-proxies each RPC to upstream `entdb-server`, attaching
   the configured API key as `authorization: Bearer …` gRPC metadata.

The proto file at `proto/console/v1/console.proto` is the **literal
access policy** for the browser: anything not declared as an RPC there
is unreachable from the SPA. Adding new functionality requires adding
an RPC to that proto (and getting it through review) — not just toggling
a feature flag.

## RPCs (PR 1 — read-only)

`Health`, `ListTenants`, `GetTenant`, `GetSchema`, `QueryNodes`,
`GetNode`, `GetEdgesFrom`, `GetEdgesTo`, `GetConnectedNodes`,
`SearchMailbox`.

Sandbox writes (`SandboxCreateNode`, `SandboxCreateEdge`) are PR 2.

## Run

```bash
# 1. Build the frontend so //go:embed has something to embed.
cd sdk/go/entdb/cmd/entdb-console/frontend
npm install
npm run build

# 2. Build and run the binary.
cd ../..   # sdk/go/entdb
go build -o /tmp/entdb-console ./cmd/entdb-console
/tmp/entdb-console --addr=:8080 --upstream=localhost:50051 --api-key=…
```

## Flags / environment

| Flag                  | Env                    | Default            | What                                                     |
|-----------------------|------------------------|--------------------|----------------------------------------------------------|
| `--addr`              | `ENTDB_CONSOLE_ADDR`   | `:8080`            | HTTP listen address                                      |
| `--upstream`          | `ENTDB_UPSTREAM`       | `localhost:50051`  | upstream entdb-server gRPC address (`host:port`)         |
| `--api-key`           | `ENTDB_API_KEY`        | (none)             | forwarded to upstream as `authorization: Bearer …`       |
| `--shutdown-timeout`  | —                      | `15s`              | graceful shutdown timeout                                |

## Browser auth (PR 1)

The SPA reads the API key from `localStorage` (key: `entdb_api_key`)
and stamps it on every Connect request via a transport interceptor.
Set it from the in-app settings dialog or paste in via DevTools:

```js
localStorage.setItem('entdb_api_key', 'your-key-here')
```

A future PR will switch to server-side templating of the key into
`index.html` so the SPA never sees the raw key. This is intentionally
kept simple for v1.

## Dev

```bash
# Frontend dev server with API proxy:
cd sdk/go/entdb/cmd/entdb-console/frontend
npm run dev   # http://localhost:5173 → proxies /entdb.console.v1.Console to localhost:8080

# In another shell — the Go server:
go run ./sdk/go/entdb/cmd/entdb-console --addr=:8080 --upstream=localhost:50051
```

## Regenerating proto code

```bash
buf generate   # from repo root; uses buf.gen.yaml
```

This produces:

* `sdk/go/entdb/internal/console/v1/console.pb.go`
* `sdk/go/entdb/internal/console/v1/consolev1connect/console.connect.go`
* `sdk/go/entdb/cmd/entdb-console/frontend/src/gen/{console,entdb}_{pb,connect}.ts`

## Tests

```bash
go test ./cmd/entdb-console/...
```

The handler tests use a `fakeServer` that satisfies the small
`upstreamStub` interface. Adding a new RPC without extending
`fakeServer` AND `upstreamStub` is intentionally a compile error.
