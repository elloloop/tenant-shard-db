import { Link, useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { consoleClient } from '../api'
import { ErrorBox } from './TenantsPage'

// PR 1 ships a JSON / table view of the connected-nodes endpoint. A
// real graph viz (cytoscape / vis-network / d3) is intentionally out
// of scope — this page exists so the GetConnectedNodes RPC is reachable
// from the SPA and can be test-driven before we layer presentation on
// top of it.
export function GraphPage() {
  const { tenantId = '', nodeId = '' } = useParams<{ tenantId: string; nodeId: string }>()

  const { data, isLoading, error } = useQuery({
    queryKey: ['graph', tenantId, nodeId],
    queryFn: async () =>
      consoleClient.getConnectedNodes({
        context: { tenantId, actor: 'console' },
        nodeId,
        limit: 100,
      }),
    enabled: !!tenantId && !!nodeId,
  })

  return (
    <div className="p-6 max-w-5xl">
      <header className="mb-6">
        <h1 className="text-2xl font-semibold">Connected nodes</h1>
        <p className="text-sm text-muted-foreground mt-1 font-mono break-all">
          {nodeId}
        </p>
        <Link
          to={`/tenants/${encodeURIComponent(tenantId)}/nodes/${encodeURIComponent(nodeId)}`}
          className="inline-block mt-2 text-sm text-primary hover:underline"
        >
          ← back to node detail
        </Link>
      </header>

      {isLoading && <p className="text-muted-foreground">Loading…</p>}
      {error && <ErrorBox error={error} />}

      {data && (
        <table className="w-full text-sm border rounded-md overflow-hidden">
          <thead className="bg-muted text-left">
            <tr>
              <th className="px-3 py-2 font-medium">Node ID</th>
              <th className="px-3 py-2 font-medium">Type</th>
              <th className="px-3 py-2 font-medium">Owner</th>
            </tr>
          </thead>
          <tbody>
            {data.nodes.map((n) => (
              <tr key={n.nodeId} className="border-t hover:bg-accent/40">
                <td className="px-3 py-2 font-mono">
                  <Link
                    to={`/tenants/${encodeURIComponent(tenantId)}/nodes/${encodeURIComponent(n.nodeId)}`}
                    className="text-primary hover:underline break-all"
                  >
                    {n.nodeId}
                  </Link>
                </td>
                <td className="px-3 py-2 font-mono">{n.typeId}</td>
                <td className="px-3 py-2 font-mono">{n.ownerActor}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}
