// Sandbox — create-node and create-edge forms backed by the
// SandboxCreateNode / SandboxCreateEdge RPCs.
//
// This page replaces the FastAPI playground functionally. It is
// intentionally minimal:
//
//   - tenant_id is read-only (the server enforces it anyway).
//   - payload / props are plain JSON textareas; the user types
//     id-keyed objects (`{"1": "value"}`) per the wire-format
//     invariant. We do NOT translate name-keyed → id-keyed in the
//     browser; it's a debug tool, the user reads the schema page.
//   - No update / delete UI. Sandbox writes are CREATE-only in v1.
//
// If you find yourself adding tabs / autocomplete / Monaco here,
// stop — read docs/decisions/console.md, the explicit goal is "small
// debug tool that doesn't drift from the proto."

import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Struct } from '@bufbuild/protobuf'
import { Link } from 'react-router-dom'
import { consoleClient } from '../api'
import { sandboxDefaultActor, sandboxTenant } from '../env'
import { ErrorBox } from './TenantsPage'

type CreateResult =
  | { kind: 'idle' }
  | { kind: 'pending' }
  | { kind: 'ok'; lines: string[] }
  | { kind: 'err'; code?: string; message: string }

// parseJsonObject turns a JSON-text input into a plain `Record<string,
// unknown>` suitable for `Struct.fromJson`. Empty strings parse to
// `{}` so leaving the props textarea blank means "no props." Any
// non-object (`null`, array, scalar) is rejected — the server stores
// payloads as a Struct, not a free-form JSON value.
function parseJsonObject(s: string): Record<string, unknown> {
  if (s.trim() === '') return {}
  const parsed = JSON.parse(s)
  if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error('payload must be a JSON object, e.g. {"1": "value"}')
  }
  return parsed as Record<string, unknown>
}

function describeError(err: unknown): { code?: string; message: string } {
  // connect.Error has a `.code` property; fall back to a plain message
  // for non-Connect errors (e.g. bad JSON the user typed).
  const anyErr = err as { code?: string; message?: string; rawMessage?: string }
  return {
    code: anyErr?.code,
    message: anyErr?.rawMessage ?? anyErr?.message ?? String(err),
  }
}

export function SandboxPage() {
  const tenant = sandboxTenant()
  const defaultActor = sandboxDefaultActor()

  // We don't redirect when the sandbox is disabled; the App-level
  // routing already hides this page from the sidebar. Reaching it via
  // a typed URL still gets a clear message instead of a runtime
  // exception when the RPC returns Unimplemented.
  if (!tenant) {
    return (
      <div className="p-6 max-w-4xl">
        <h1 className="text-2xl font-semibold">Sandbox</h1>
        <p className="text-sm text-muted-foreground mt-2">
          Sandbox writes are disabled. Start the console binary with
          <code className="font-mono mx-1">--sandbox-tenant=&lt;name&gt;</code>
          to enable.
        </p>
      </div>
    )
  }

  return (
    <div className="p-6 max-w-5xl space-y-8">
      <header>
        <h1 className="text-2xl font-semibold">Sandbox</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Create nodes and edges in the sandbox tenant{' '}
          <code className="font-mono">{tenant}</code>. This is a debug tool —
          payloads are id-keyed (look up field ids on the{' '}
          <Link
            to={`/tenants/${encodeURIComponent(tenant)}/schema`}
            className="text-primary underline"
          >
            schema page
          </Link>
          ).
        </p>
      </header>

      <CreateNodeCard tenant={tenant} defaultActor={defaultActor} />
      <CreateEdgeCard tenant={tenant} defaultActor={defaultActor} />
    </div>
  )
}

// extractTypeHints reads the schema Struct (which is a free-form
// Struct on the wire, not a typed message — see entdb.proto's
// GetSchemaResponse) and pulls a list of `{name, id}` for the given
// inner key (`node_types` or `edge_types`). Schema serialisation may
// vary across server versions, so we accept either an array of
// `{name, type_id}` / `{name, edge_id}` shapes or a map of
// name→numeric-id; everything else returns an empty list.
type SchemaTypeHint = { name: string; id: number }

function extractSchemaHints(
  schema: Struct | undefined,
  arrayKey: string,
  idField: string,
): SchemaTypeHint[] {
  if (!schema) return []
  const json = schema.toJson() as Record<string, unknown>
  const raw = json[arrayKey]
  if (Array.isArray(raw)) {
    return raw
      .map((item) => {
        if (item && typeof item === 'object') {
          const o = item as Record<string, unknown>
          const name = typeof o.name === 'string' ? o.name : ''
          const idVal = o[idField]
          const id = typeof idVal === 'number' ? idVal : Number(idVal)
          if (name && Number.isFinite(id)) return { name, id }
        }
        return null
      })
      .filter((x): x is SchemaTypeHint => x !== null)
  }
  if (raw && typeof raw === 'object') {
    return Object.entries(raw as Record<string, unknown>)
      .map(([name, idVal]) => {
        const id = typeof idVal === 'number' ? idVal : Number(idVal)
        if (Number.isFinite(id)) return { name, id }
        return null
      })
      .filter((x): x is SchemaTypeHint => x !== null)
  }
  return []
}

function SchemaTypeHintLine({ tenant, kind }: { tenant: string; kind: 'node' | 'edge' }) {
  // Surface the schema's known type ids inline so the user doesn't
  // have to context-switch to the schema page just to look up "what
  // numeric id is User?" Best-effort: if the schema serialisation
  // doesn't match a shape we recognise, the line just doesn't render.
  const { data, error } = useQuery({
    queryKey: ['schema', tenant],
    queryFn: async () => consoleClient.getSchema({ tenantId: tenant }),
    enabled: !!tenant,
  })
  if (error) return null
  const hints =
    kind === 'node'
      ? extractSchemaHints(data?.schema, 'node_types', 'type_id')
      : extractSchemaHints(data?.schema, 'edge_types', 'edge_id')
  if (hints.length === 0) return null
  return (
    <p className="text-xs text-muted-foreground">
      Known {kind} types:{' '}
      {hints.map((h, i) => (
        <span key={`${h.name}:${h.id}`}>
          {i > 0 && ', '}
          <code className="font-mono">
            {h.name}={h.id}
          </code>
        </span>
      ))}
    </p>
  )
}

function CreateNodeCard({ tenant, defaultActor }: { tenant: string; defaultActor: string }) {
  const [actor, setActor] = useState(defaultActor)
  const [typeIdRaw, setTypeIdRaw] = useState('')
  const [payloadRaw, setPayloadRaw] = useState('{\n  "1": "example"\n}')
  const [idemKey, setIdemKey] = useState('')
  const [result, setResult] = useState<CreateResult>({ kind: 'idle' })

  const submit = async () => {
    setResult({ kind: 'pending' })
    let typeId: number
    let payloadObj: Record<string, unknown>
    try {
      typeId = Number.parseInt(typeIdRaw, 10)
      if (!Number.isFinite(typeId) || typeId <= 0) {
        throw new Error('type_id must be a positive integer')
      }
      payloadObj = parseJsonObject(payloadRaw)
    } catch (e) {
      setResult({ kind: 'err', message: (e as Error).message })
      return
    }
    try {
      const resp = await consoleClient.sandboxCreateNode({
        tenantId: tenant,
        actor,
        typeId,
        payload: Struct.fromJsonString(JSON.stringify(payloadObj)),
        idempotencyKey: idemKey,
      })
      setResult({
        kind: 'ok',
        lines: [
          `node_id: ${resp.nodeId}`,
          `idempotency_key: ${resp.idempotencyKey}`,
          `stream_position: ${resp.streamPosition}`,
        ],
      })
    } catch (e) {
      setResult({ kind: 'err', ...describeError(e) })
    }
  }

  return (
    <section className="rounded-md border bg-card p-4">
      <h2 className="text-lg font-semibold mb-3">Create node</h2>
      <div className="space-y-3">
        <Field label="tenant_id">
          <input
            value={tenant}
            readOnly
            className="w-full rounded-md border bg-muted px-2 py-1 text-sm font-mono"
          />
        </Field>
        <Field label="actor">
          <input
            value={actor}
            onChange={(e) => setActor(e.target.value)}
            className="w-full rounded-md border bg-background px-2 py-1 text-sm font-mono"
            placeholder="user:demo"
          />
        </Field>
        <Field label="type_id">
          <input
            type="number"
            min={1}
            value={typeIdRaw}
            onChange={(e) => setTypeIdRaw(e.target.value)}
            className="w-32 rounded-md border bg-background px-2 py-1 text-sm font-mono"
            placeholder="e.g. 1"
          />
        </Field>
        <SchemaTypeHintLine tenant={tenant} kind="node" />
        <Field label="payload (id-keyed JSON)">
          <textarea
            value={payloadRaw}
            onChange={(e) => setPayloadRaw(e.target.value)}
            rows={8}
            className="w-full rounded-md border bg-background px-2 py-1 text-sm font-mono code-block"
            spellCheck={false}
          />
        </Field>
        <Field label="idempotency_key (optional)">
          <input
            value={idemKey}
            onChange={(e) => setIdemKey(e.target.value)}
            className="w-full rounded-md border bg-background px-2 py-1 text-sm font-mono"
            placeholder="auto-generated if blank"
          />
        </Field>
        <div className="pt-2">
          <button
            onClick={submit}
            disabled={result.kind === 'pending'}
            className="rounded-md border bg-primary text-primary-foreground px-4 py-2 text-sm font-medium hover:opacity-90 disabled:opacity-50"
          >
            {result.kind === 'pending' ? 'Creating…' : 'Create'}
          </button>
        </div>
        <ResultPanel result={result} />
      </div>
    </section>
  )
}

function CreateEdgeCard({ tenant, defaultActor }: { tenant: string; defaultActor: string }) {
  const [actor, setActor] = useState(defaultActor)
  const [edgeTypeIdRaw, setEdgeTypeIdRaw] = useState('')
  const [fromNodeId, setFromNodeId] = useState('')
  const [toNodeId, setToNodeId] = useState('')
  const [propsRaw, setPropsRaw] = useState('')
  const [idemKey, setIdemKey] = useState('')
  const [result, setResult] = useState<CreateResult>({ kind: 'idle' })

  const submit = async () => {
    setResult({ kind: 'pending' })
    let edgeTypeId: number
    let propsObj: Record<string, unknown>
    try {
      edgeTypeId = Number.parseInt(edgeTypeIdRaw, 10)
      if (!Number.isFinite(edgeTypeId) || edgeTypeId <= 0) {
        throw new Error('edge_type_id must be a positive integer')
      }
      if (!fromNodeId || !toNodeId) {
        throw new Error('from_node_id and to_node_id are required')
      }
      propsObj = parseJsonObject(propsRaw)
    } catch (e) {
      setResult({ kind: 'err', message: (e as Error).message })
      return
    }
    try {
      const resp = await consoleClient.sandboxCreateEdge({
        tenantId: tenant,
        actor,
        edgeTypeId,
        fromNodeId,
        toNodeId,
        // Only attach props when the user typed something, so empty
        // textareas don't send a non-null `Struct{}` (cleaner on the
        // upstream WAL event).
        props:
          propsRaw.trim() === ''
            ? undefined
            : Struct.fromJsonString(JSON.stringify(propsObj)),
        idempotencyKey: idemKey,
      })
      setResult({
        kind: 'ok',
        lines: [
          `idempotency_key: ${resp.idempotencyKey}`,
          `stream_position: ${resp.streamPosition}`,
        ],
      })
    } catch (e) {
      setResult({ kind: 'err', ...describeError(e) })
    }
  }

  return (
    <section className="rounded-md border bg-card p-4">
      <h2 className="text-lg font-semibold mb-3">Create edge</h2>
      <div className="space-y-3">
        <Field label="tenant_id">
          <input
            value={tenant}
            readOnly
            className="w-full rounded-md border bg-muted px-2 py-1 text-sm font-mono"
          />
        </Field>
        <Field label="actor">
          <input
            value={actor}
            onChange={(e) => setActor(e.target.value)}
            className="w-full rounded-md border bg-background px-2 py-1 text-sm font-mono"
            placeholder="user:demo"
          />
        </Field>
        <Field label="edge_type_id">
          <input
            type="number"
            min={1}
            value={edgeTypeIdRaw}
            onChange={(e) => setEdgeTypeIdRaw(e.target.value)}
            className="w-32 rounded-md border bg-background px-2 py-1 text-sm font-mono"
            placeholder="e.g. 1"
          />
        </Field>
        <SchemaTypeHintLine tenant={tenant} kind="edge" />
        <Field label="from_node_id">
          <input
            value={fromNodeId}
            onChange={(e) => setFromNodeId(e.target.value)}
            className="w-full rounded-md border bg-background px-2 py-1 text-sm font-mono"
          />
        </Field>
        <Field label="to_node_id">
          <input
            value={toNodeId}
            onChange={(e) => setToNodeId(e.target.value)}
            className="w-full rounded-md border bg-background px-2 py-1 text-sm font-mono"
          />
        </Field>
        <Field label="props (id-keyed JSON, optional)">
          <textarea
            value={propsRaw}
            onChange={(e) => setPropsRaw(e.target.value)}
            rows={4}
            className="w-full rounded-md border bg-background px-2 py-1 text-sm font-mono code-block"
            placeholder='{"1": "value"}'
            spellCheck={false}
          />
        </Field>
        <Field label="idempotency_key (optional)">
          <input
            value={idemKey}
            onChange={(e) => setIdemKey(e.target.value)}
            className="w-full rounded-md border bg-background px-2 py-1 text-sm font-mono"
          />
        </Field>
        <div className="pt-2">
          <button
            onClick={submit}
            disabled={result.kind === 'pending'}
            className="rounded-md border bg-primary text-primary-foreground px-4 py-2 text-sm font-medium hover:opacity-90 disabled:opacity-50"
          >
            {result.kind === 'pending' ? 'Creating…' : 'Create'}
          </button>
        </div>
        <ResultPanel result={result} />
      </div>
    </section>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block">
      <span className="block text-xs font-medium text-muted-foreground mb-1">{label}</span>
      {children}
    </label>
  )
}

function ResultPanel({ result }: { result: CreateResult }) {
  if (result.kind === 'idle' || result.kind === 'pending') return null
  if (result.kind === 'ok') {
    return (
      <div className="rounded-md border border-green-700/40 bg-green-900/10 p-3 text-sm">
        <div className="font-medium mb-1">Created</div>
        <pre className="font-mono text-xs whitespace-pre-wrap">{result.lines.join('\n')}</pre>
      </div>
    )
  }
  return (
    <ErrorBox
      error={
        result.code
          ? new Error(`[${result.code}] ${result.message}`)
          : new Error(result.message)
      }
    />
  )
}
