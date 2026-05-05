import { useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { consoleClient } from '../api'
import { ErrorBox } from './TenantsPage'

export function SearchPage() {
  const { tenantId = '' } = useParams<{ tenantId: string }>()
  const [userId, setUserId] = useState('')
  const [draft, setDraft] = useState('')
  const [query, setQuery] = useState('')

  const { data, isLoading, error } = useQuery({
    queryKey: ['mailbox-search', tenantId, userId, query],
    queryFn: async () =>
      consoleClient.searchMailbox({
        context: { tenantId, actor: 'console' },
        userId,
        query,
        limit: 50,
      }),
    enabled: !!tenantId && !!userId && !!query,
  })

  return (
    <div className="p-6 max-w-5xl">
      <header className="mb-6">
        <h1 className="text-2xl font-semibold">Search mailbox</h1>
        <p className="text-sm text-muted-foreground mt-1 font-mono">{tenantId}</p>
      </header>

      <form
        className="flex flex-wrap items-end gap-3 mb-6"
        onSubmit={(e) => {
          e.preventDefault()
          setQuery(draft)
        }}
      >
        <div>
          <label className="block text-xs uppercase tracking-wide text-muted-foreground mb-1">
            User
          </label>
          <input
            value={userId}
            onChange={(e) => setUserId(e.target.value)}
            className="rounded-md border bg-background px-3 py-2 text-sm font-mono"
            placeholder="user_id"
          />
        </div>
        <div className="flex-1 min-w-[16rem]">
          <label className="block text-xs uppercase tracking-wide text-muted-foreground mb-1">
            Query
          </label>
          <input
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            className="rounded-md border bg-background px-3 py-2 text-sm w-full"
            placeholder="search terms…"
          />
        </div>
        <button
          type="submit"
          className="rounded-md bg-primary text-primary-foreground px-4 py-2 text-sm hover:opacity-90"
        >
          Search
        </button>
      </form>

      {isLoading && <p className="text-muted-foreground">Loading…</p>}
      {error && <ErrorBox error={error} />}

      {data && (
        <ul className="space-y-2">
          {data.results.map((r, i) => {
            const it = r.item
            if (!it) return null
            return (
              <li key={i} className="rounded-md border bg-card p-3">
                <div className="text-xs text-muted-foreground font-mono">
                  rank {r.rank.toFixed(3)} · type {it.sourceTypeId}
                </div>
                <Link
                  to={`/tenants/${encodeURIComponent(tenantId)}/nodes/${encodeURIComponent(it.sourceNodeId)}?type=${it.sourceTypeId}`}
                  className="font-mono text-sm text-primary hover:underline break-all"
                >
                  {it.sourceNodeId}
                </Link>
                {it.snippet && (
                  <div className="mt-1 text-sm">{it.snippet}</div>
                )}
              </li>
            )
          })}
          {data.results.length === 0 && (
            <li className="text-muted-foreground italic">No matches.</li>
          )}
        </ul>
      )}
    </div>
  )
}
