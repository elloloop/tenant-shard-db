import { Link, useParams, useSearchParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { consoleClient } from '../api'
import { ErrorBox } from './TenantsPage'

const PAGE_SIZE = 50

export function NodesPage() {
  const { tenantId = '' } = useParams<{ tenantId: string }>()
  const [searchParams, setSearchParams] = useSearchParams()
  const typeIdRaw = searchParams.get('type') ?? ''
  const offsetRaw = searchParams.get('offset') ?? '0'
  const typeId = Number.parseInt(typeIdRaw, 10)
  const offset = Number.parseInt(offsetRaw, 10) || 0

  const enabled = !!tenantId && Number.isFinite(typeId) && typeId > 0

  const { data, isLoading, error } = useQuery({
    queryKey: ['nodes', tenantId, typeId, offset],
    queryFn: async () =>
      consoleClient.queryNodes({
        context: { tenantId, actor: 'console' },
        typeId,
        limit: PAGE_SIZE,
        offset,
      }),
    enabled,
  })

  const setTypeId = (v: string) => {
    const next = new URLSearchParams(searchParams)
    next.set('type', v)
    next.delete('offset')
    setSearchParams(next, { replace: true })
  }
  const setOffset = (v: number) => {
    const next = new URLSearchParams(searchParams)
    next.set('offset', String(v))
    setSearchParams(next, { replace: true })
  }

  return (
    <div className="p-6 max-w-6xl">
      <header className="mb-6">
        <h1 className="text-2xl font-semibold">Nodes</h1>
        <p className="text-sm text-muted-foreground mt-1 font-mono">{tenantId}</p>
      </header>

      <div className="mb-4 flex items-center gap-3">
        <label className="text-sm font-medium">Type ID</label>
        <input
          type="number"
          min={1}
          value={typeIdRaw}
          onChange={(e) => setTypeId(e.target.value)}
          className="rounded-md border bg-background px-2 py-1 text-sm w-32"
          placeholder="e.g. 1"
        />
        {enabled && data && (
          <span className="text-xs text-muted-foreground">
            Showing {offset + 1}–{offset + data.nodes.length} of {data.totalCount}
          </span>
        )}
      </div>

      {!enabled && (
        <p className="text-muted-foreground italic">Choose a numeric type id to query.</p>
      )}

      {isLoading && enabled && <p className="text-muted-foreground">Loading…</p>}
      {error && <ErrorBox error={error} />}

      {data && enabled && (
        <>
          <table className="w-full text-sm border rounded-md overflow-hidden">
            <thead className="bg-muted text-left">
              <tr>
                <th className="px-3 py-2 font-medium">Node ID</th>
                <th className="px-3 py-2 font-medium">Owner</th>
                <th className="px-3 py-2 font-medium">Updated</th>
                <th className="px-3 py-2"></th>
              </tr>
            </thead>
            <tbody>
              {data.nodes.map((n) => (
                <tr key={n.nodeId} className="border-t hover:bg-accent/40">
                  <td className="px-3 py-2 font-mono">{n.nodeId}</td>
                  <td className="px-3 py-2 font-mono">{n.ownerActor}</td>
                  <td className="px-3 py-2 font-mono">
                    {n.updatedAt ? new Date(Number(n.updatedAt)).toISOString() : '—'}
                  </td>
                  <td className="px-3 py-2">
                    <Link
                      to={`/tenants/${encodeURIComponent(tenantId)}/nodes/${encodeURIComponent(n.nodeId)}?type=${typeId}`}
                      className="text-primary hover:underline"
                    >
                      view
                    </Link>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>

          <div className="mt-4 flex gap-2">
            <button
              onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
              disabled={offset === 0}
              className="rounded-md border px-3 py-1 text-sm disabled:opacity-50"
            >
              ← Prev
            </button>
            <button
              onClick={() => setOffset(offset + PAGE_SIZE)}
              disabled={!data.hasMore}
              className="rounded-md border px-3 py-1 text-sm disabled:opacity-50"
            >
              Next →
            </button>
          </div>
        </>
      )}
    </div>
  )
}
