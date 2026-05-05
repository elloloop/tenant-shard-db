import { Link, useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { consoleClient } from '../api'
import { ErrorBox } from './TenantsPage'

export function TenantDetailPage() {
  const { tenantId = '' } = useParams<{ tenantId: string }>()

  const { data, isLoading, error } = useQuery({
    queryKey: ['tenant', tenantId],
    queryFn: async () => consoleClient.getTenant({ tenantId, actor: 'console' }),
    enabled: !!tenantId,
  })

  return (
    <div className="p-6 max-w-4xl">
      <header className="mb-6">
        <h1 className="text-2xl font-semibold font-mono">{tenantId}</h1>
        <p className="text-sm text-muted-foreground mt-1">Tenant detail</p>
      </header>

      {isLoading && <p className="text-muted-foreground">Loading…</p>}
      {error && <ErrorBox error={error} />}

      {data?.found && data.tenant && (
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 mb-6">
          <Field label="Name" value={data.tenant.name} />
          <Field label="Status" value={data.tenant.status} />
          <Field label="Region" value={data.tenant.region || '(default)'} />
          <Field
            label="Created"
            value={data.tenant.createdAt ? new Date(Number(data.tenant.createdAt)).toISOString() : '—'}
          />
        </div>
      )}

      {data && !data.found && (
        <p className="text-muted-foreground">Tenant not found (or no permission).</p>
      )}

      <nav className="flex gap-2 mt-6">
        <Link
          to={`/tenants/${encodeURIComponent(tenantId)}/schema`}
          className="rounded-md border px-4 py-2 text-sm hover:bg-accent"
        >
          Schema
        </Link>
        <Link
          to={`/tenants/${encodeURIComponent(tenantId)}/nodes`}
          className="rounded-md border px-4 py-2 text-sm hover:bg-accent"
        >
          Nodes
        </Link>
        <Link
          to={`/tenants/${encodeURIComponent(tenantId)}/search`}
          className="rounded-md border px-4 py-2 text-sm hover:bg-accent"
        >
          Search mailbox
        </Link>
      </nav>
    </div>
  )
}

function Field({ label, value }: { label: string; value: string | undefined }) {
  return (
    <div>
      <div className="text-xs uppercase tracking-wide text-muted-foreground">{label}</div>
      <div className="font-mono text-sm mt-1">{value ?? '—'}</div>
    </div>
  )
}
