import { useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { api, MailboxItemData } from '../api'

export default function MailboxPage() {
  const { userId } = useParams<{ userId: string }>()
  const decodedUserId = userId ? decodeURIComponent(userId) : ''

  const { data, isLoading, error } = useQuery({
    queryKey: ['mailbox', decodedUserId],
    queryFn: () => api.getUserMailbox(decodedUserId),
    enabled: !!decodedUserId,
  })

  if (!decodedUserId) {
    return <div className="p-6 text-muted-foreground">No user selected.</div>
  }

  return (
    <div className="p-6">
      <div className="mb-6">
        <h1 className="text-2xl font-bold">Mailbox: {decodedUserId}</h1>
        <p className="text-muted-foreground text-sm mt-1">
          {data?.items?.length ?? 0} item{(data?.items?.length ?? 0) !== 1 ? 's' : ''}
        </p>
      </div>

      {isLoading && <p className="text-muted-foreground">Loading...</p>}
      {error && <p className="text-destructive">Error loading mailbox: {String(error)}</p>}

      {data?.items && data.items.length > 0 && (
        <div className="rounded-md border">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b bg-muted/50">
                <th className="text-left px-4 py-3 font-medium">Item ID</th>
                <th className="text-left px-4 py-3 font-medium">Source Type</th>
                <th className="text-left px-4 py-3 font-medium">Snippet</th>
                <th className="text-left px-4 py-3 font-medium">Timestamp</th>
                <th className="text-left px-4 py-3 font-medium">State</th>
              </tr>
            </thead>
            <tbody>
              {data.items.map((item: MailboxItemData) => (
                <tr key={item.item_id} className="border-b last:border-0 hover:bg-muted/30">
                  <td className="px-4 py-3 font-mono text-xs">{item.item_id.slice(0, 8)}...</td>
                  <td className="px-4 py-3">{item.source_type_id}</td>
                  <td className="px-4 py-3 max-w-md truncate">{item.snippet || '-'}</td>
                  <td className="px-4 py-3 text-muted-foreground">
                    {item.ts ? new Date(item.ts).toLocaleString() : '-'}
                  </td>
                  <td className="px-4 py-3">
                    {item.state?.read ? (
                      <span className="text-muted-foreground">read</span>
                    ) : (
                      <span className="font-medium text-primary">unread</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {data?.items && data.items.length === 0 && (
        <p className="text-muted-foreground">No mailbox items.</p>
      )}
    </div>
  )
}
