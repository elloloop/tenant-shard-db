import { Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { consoleClient } from '../api'

export function TenantsPage() {
  const { data, isLoading, error } = useQuery({
    queryKey: ['tenants'],
    queryFn: async () => consoleClient.listTenants({}),
  })

  return (
    <div className="p-6 max-w-4xl">
      <header className="mb-6">
        <h1 className="text-2xl font-semibold">Tenants</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Tenants visible to the configured API key.
        </p>
      </header>

      {isLoading && <p className="text-muted-foreground">Loading…</p>}
      {error && <ErrorBox error={error} />}

      {data && (
        <ul className="space-y-2">
          {data.tenants.map((t) => (
            <li key={t.tenantId}>
              <Link
                to={`/tenants/${encodeURIComponent(t.tenantId)}`}
                className="block rounded-md border bg-card px-4 py-3 hover:bg-accent transition-colors"
              >
                <span className="font-mono">{t.tenantId}</span>
              </Link>
            </li>
          ))}
          {data.tenants.length === 0 && (
            <li className="text-muted-foreground italic">No tenants visible.</li>
          )}
        </ul>
      )}
    </div>
  )
}

export function ErrorBox({ error }: { error: unknown }) {
  const msg = error instanceof Error ? error.message : String(error)
  return (
    <div className="rounded-md border border-destructive/50 bg-destructive/10 p-3 text-sm text-destructive">
      {msg}
    </div>
  )
}
