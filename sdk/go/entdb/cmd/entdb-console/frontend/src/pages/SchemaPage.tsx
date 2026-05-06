import { useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { consoleClient } from '../api'
import { ErrorBox } from './TenantsPage'

export function SchemaPage() {
  const { tenantId = '' } = useParams<{ tenantId: string }>()

  const { data, isLoading, error } = useQuery({
    queryKey: ['schema', tenantId],
    queryFn: async () => consoleClient.getSchema({ tenantId }),
    enabled: !!tenantId,
  })

  return (
    <div className="p-6 max-w-5xl">
      <header className="mb-6">
        <h1 className="text-2xl font-semibold">Schema</h1>
        <p className="text-sm text-muted-foreground mt-1 font-mono">{tenantId}</p>
        {data?.fingerprint && (
          <p className="text-xs text-muted-foreground mt-1">
            Fingerprint: <code className="font-mono">{data.fingerprint}</code>
          </p>
        )}
      </header>

      {isLoading && <p className="text-muted-foreground">Loading…</p>}
      {error && <ErrorBox error={error} />}

      {data?.schema && (
        <pre className="rounded-md border bg-card p-4 text-sm overflow-auto code-block">
          {JSON.stringify(data.schema.toJson(), null, 2)}
        </pre>
      )}
    </div>
  )
}
