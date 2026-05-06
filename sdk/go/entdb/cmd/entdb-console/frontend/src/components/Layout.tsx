import { ReactNode, useState } from 'react'
import { Link, useLocation, useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Database, FlaskConical, Search as SearchIcon, Menu, X, Settings } from 'lucide-react'
import { consoleClient } from '../api'
import { cn } from '../lib/utils'
import { sandboxEnabled } from '../env'
import { SettingsDialog } from './SettingsDialog'

interface LayoutProps {
  children: ReactNode
}

export default function Layout({ children }: LayoutProps) {
  const [sidebarOpen, setSidebarOpen] = useState(true)
  const [settingsOpen, setSettingsOpen] = useState(false)
  const location = useLocation()
  const params = useParams()
  const activeTenant = (params.tenantId as string | undefined) ?? ''

  const { data: tenantsData } = useQuery({
    queryKey: ['tenants'],
    queryFn: async () => consoleClient.listTenants({}),
  })

  const isActive = (path: string) => location.pathname === path

  return (
    <div className="flex h-screen bg-background text-foreground">
      <aside
        className={cn(
          'bg-card border-r flex flex-col transition-all duration-200',
          sidebarOpen ? 'w-64' : 'w-16',
        )}
      >
        <div className="h-14 flex items-center justify-between px-4 border-b">
          {sidebarOpen && (
            <Link to="/" className="flex items-center gap-2 font-semibold">
              <Database className="w-5 h-5 text-primary" />
              EntDB Console
            </Link>
          )}
          <button
            onClick={() => setSidebarOpen(!sidebarOpen)}
            className="h-8 w-8 inline-flex items-center justify-center rounded-md hover:bg-accent"
            aria-label={sidebarOpen ? 'Collapse sidebar' : 'Expand sidebar'}
          >
            {sidebarOpen ? <X className="w-4 h-4" /> : <Menu className="w-4 h-4" />}
          </button>
        </div>

        <nav className="flex-1 overflow-y-auto py-4">
          <div className="px-3 mb-4">
            <Link
              to="/tenants"
              className={cn(
                'flex items-center gap-3 px-3 py-2 rounded-md text-sm font-medium',
                isActive('/tenants') || isActive('/')
                  ? 'bg-primary text-primary-foreground'
                  : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground',
              )}
            >
              <Database className="w-4 h-4" />
              {sidebarOpen && <span>Tenants</span>}
            </Link>
          </div>

          {sidebarOpen && tenantsData?.tenants && tenantsData.tenants.length > 0 && (
            <div className="px-2 space-y-1">
              <div className="px-2 mb-1">
                <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">
                  Tenants
                </h3>
              </div>
              {tenantsData.tenants.map((t) => (
                <Link
                  key={t.tenantId}
                  to={`/tenants/${encodeURIComponent(t.tenantId)}`}
                  className={cn(
                    'flex items-center gap-3 px-3 py-2 rounded-md text-sm transition-colors',
                    activeTenant === t.tenantId
                      ? 'bg-primary text-primary-foreground'
                      : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground',
                  )}
                >
                  <span className="flex-1 truncate">{t.tenantId}</span>
                </Link>
              ))}
            </div>
          )}

          {sidebarOpen && activeTenant && (
            <div className="px-3 mt-6">
              <Link
                to={`/tenants/${encodeURIComponent(activeTenant)}/search`}
                className="flex items-center gap-3 px-3 py-2 rounded-md text-sm text-muted-foreground hover:bg-accent hover:text-accent-foreground"
              >
                <SearchIcon className="w-4 h-4" />
                Search mailbox
              </Link>
            </div>
          )}

          {/* Sandbox tab is rendered ONLY when the Go server stamped a
              non-empty __ENTDB_SANDBOX_TENANT__ into index.html. The
              decision is taken once at first render — no flicker —
              because the value lands on `window` before React mounts. */}
          {sandboxEnabled() && (
            <div className="px-3 mt-6">
              <Link
                to="/sandbox"
                className={cn(
                  'flex items-center gap-3 px-3 py-2 rounded-md text-sm',
                  isActive('/sandbox')
                    ? 'bg-primary text-primary-foreground'
                    : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground',
                )}
              >
                <FlaskConical className="w-4 h-4" />
                {sidebarOpen && <span>Sandbox</span>}
              </Link>
            </div>
          )}
        </nav>

        <div className="border-t p-3">
          <button
            onClick={() => setSettingsOpen(true)}
            className="flex items-center gap-2 w-full px-3 py-2 rounded-md text-sm text-muted-foreground hover:bg-accent hover:text-accent-foreground"
          >
            <Settings className="w-4 h-4" />
            {sidebarOpen && <span>Settings</span>}
          </button>
        </div>
      </aside>

      <main className="flex-1 overflow-auto bg-background">{children}</main>

      <SettingsDialog open={settingsOpen} onOpenChange={setSettingsOpen} />
    </div>
  )
}
