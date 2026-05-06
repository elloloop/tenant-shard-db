import { Link, useParams, useSearchParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { consoleClient } from '../api'
import { ErrorBox } from './TenantsPage'

export function NodeDetailPage() {
  const { tenantId = '', nodeId = '' } = useParams<{ tenantId: string; nodeId: string }>()
  const [searchParams] = useSearchParams()
  const typeId = Number.parseInt(searchParams.get('type') ?? '0', 10) || 0

  const ctx = { tenantId, actor: 'console' }

  const nodeQuery = useQuery({
    queryKey: ['node', tenantId, nodeId, typeId],
    queryFn: async () =>
      consoleClient.getNode({ context: ctx, typeId, nodeId }),
    enabled: !!tenantId && !!nodeId,
  })

  const fromQuery = useQuery({
    queryKey: ['edges-from', tenantId, nodeId],
    queryFn: async () =>
      consoleClient.getEdgesFrom({ context: ctx, nodeId, limit: 100 }),
    enabled: !!tenantId && !!nodeId,
  })

  const toQuery = useQuery({
    queryKey: ['edges-to', tenantId, nodeId],
    queryFn: async () =>
      consoleClient.getEdgesTo({ context: ctx, nodeId, limit: 100 }),
    enabled: !!tenantId && !!nodeId,
  })

  return (
    <div className="p-6 max-w-5xl">
      <header className="mb-6">
        <h1 className="text-2xl font-semibold font-mono break-all">{nodeId}</h1>
        <p className="text-sm text-muted-foreground mt-1">
          tenant <span className="font-mono">{tenantId}</span>
          {typeId > 0 && (
            <>
              {' · '}type <span className="font-mono">{typeId}</span>
            </>
          )}
        </p>
        <Link
          to={`/tenants/${encodeURIComponent(tenantId)}/nodes/${encodeURIComponent(nodeId)}/graph?type=${typeId}`}
          className="inline-block mt-2 text-sm text-primary hover:underline"
        >
          → graph view
        </Link>
      </header>

      {nodeQuery.isLoading && <p className="text-muted-foreground">Loading…</p>}
      {nodeQuery.error && <ErrorBox error={nodeQuery.error} />}

      {nodeQuery.data?.found && nodeQuery.data.node && (
        <section className="mb-8">
          <h2 className="text-lg font-medium mb-2">Payload</h2>
          <pre className="rounded-md border bg-card p-4 text-sm overflow-auto code-block">
            {JSON.stringify(nodeQuery.data.node.payload?.toJson() ?? {}, null, 2)}
          </pre>
        </section>
      )}

      <section className="mb-8">
        <h2 className="text-lg font-medium mb-2">Outgoing edges</h2>
        {fromQuery.isLoading && <p className="text-muted-foreground">Loading…</p>}
        {fromQuery.error && <ErrorBox error={fromQuery.error} />}
        {fromQuery.data && (
          <EdgeList tenantId={tenantId} edges={fromQuery.data.edges} direction="from" />
        )}
      </section>

      <section className="mb-8">
        <h2 className="text-lg font-medium mb-2">Incoming edges</h2>
        {toQuery.isLoading && <p className="text-muted-foreground">Loading…</p>}
        {toQuery.error && <ErrorBox error={toQuery.error} />}
        {toQuery.data && (
          <EdgeList tenantId={tenantId} edges={toQuery.data.edges} direction="to" />
        )}
      </section>
    </div>
  )
}

interface EdgeRow {
  edgeTypeId: number
  fromNodeId: string
  toNodeId: string
}

function EdgeList({
  tenantId,
  edges,
  direction,
}: {
  tenantId: string
  edges: EdgeRow[]
  direction: 'from' | 'to'
}) {
  if (edges.length === 0) {
    return <p className="text-sm text-muted-foreground italic">none</p>
  }
  return (
    <ul className="text-sm space-y-1 font-mono">
      {edges.map((e, i) => {
        const otherId = direction === 'from' ? e.toNodeId : e.fromNodeId
        return (
          <li key={i}>
            <span className="text-muted-foreground">[type {e.edgeTypeId}]</span>{' '}
            <Link
              to={`/tenants/${encodeURIComponent(tenantId)}/nodes/${encodeURIComponent(otherId)}`}
              className="text-primary hover:underline break-all"
            >
              {otherId}
            </Link>
          </li>
        )
      })}
    </ul>
  )
}
