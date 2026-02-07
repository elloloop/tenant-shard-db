import { ReactNode, useState } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import {
  Database,
  Box,
  Search,
  Menu,
  X,
  ChevronRight,
  ExternalLink,
  Sun,
  Moon,
  Monitor,
} from 'lucide-react'
import { api } from '../api'
import { Button } from '@/components/ui/button'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import { useTheme } from '@/components/theme-provider'
import { cn } from '@/lib/utils'

interface LayoutProps {
  children: ReactNode
}

function ThemeToggle() {
  const { theme, setTheme } = useTheme()

  const cycleTheme = () => {
    if (theme === 'light') setTheme('dark')
    else if (theme === 'dark') setTheme('system')
    else setTheme('light')
  }

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button variant="ghost" size="icon" onClick={cycleTheme} className="h-9 w-9">
          {theme === 'light' && <Sun className="h-4 w-4" />}
          {theme === 'dark' && <Moon className="h-4 w-4" />}
          {theme === 'system' && <Monitor className="h-4 w-4" />}
        </Button>
      </TooltipTrigger>
      <TooltipContent>
        {theme === 'light' && 'Light mode'}
        {theme === 'dark' && 'Dark mode'}
        {theme === 'system' && 'System mode'}
      </TooltipContent>
    </Tooltip>
  )
}

export default function Layout({ children }: LayoutProps) {
  const [sidebarOpen, setSidebarOpen] = useState(true)
  const location = useLocation()

  const { data: typesData } = useQuery({
    queryKey: ['types'],
    queryFn: () => api.getTypes(),
  })

  const isActive = (path: string) => location.pathname === path

  return (
    <TooltipProvider>
      <div className="flex h-screen bg-background">
        {/* Sidebar */}
        <aside
          className={cn(
            "bg-card border-r flex flex-col transition-all duration-200",
            sidebarOpen ? 'w-64' : 'w-16'
          )}
        >
          {/* Logo */}
          <div className="h-14 flex items-center justify-between px-4 border-b">
            {sidebarOpen && (
              <Link to="/" className="flex items-center gap-2 font-semibold">
                <Database className="w-5 h-5 text-primary" />
                EntDB Console
              </Link>
            )}
            <Button
              variant="ghost"
              size="icon"
              onClick={() => setSidebarOpen(!sidebarOpen)}
              className="h-8 w-8"
            >
              {sidebarOpen ? <X className="w-4 h-4" /> : <Menu className="w-4 h-4" />}
            </Button>
          </div>

          {/* Navigation */}
          <nav className="flex-1 overflow-y-auto py-4">
            <div className="px-3 mb-4">
              <Link
                to="/search"
                className={cn(
                  "flex items-center gap-3 px-3 py-2 rounded-md text-sm font-medium transition-colors",
                  isActive('/search')
                    ? 'bg-primary text-primary-foreground'
                    : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground'
                )}
              >
                <Search className="w-4 h-4" />
                {sidebarOpen && <span>Search</span>}
              </Link>
            </div>

            {sidebarOpen && (
              <div className="px-4 mb-2">
                <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">
                  Node Types
                </h3>
              </div>
            )}

            <div className="px-2 space-y-1">
              {typesData?.types.map((type) => (
                <Link
                  key={type.type_id}
                  to={`/types/${type.type_id}`}
                  className={cn(
                    "flex items-center gap-3 px-3 py-2 rounded-md text-sm transition-colors",
                    location.pathname === `/types/${type.type_id}`
                      ? 'bg-primary text-primary-foreground'
                      : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground'
                  )}
                >
                  <Box className="w-4 h-4" />
                  {sidebarOpen && (
                    <>
                      <span className="flex-1 truncate">{type.name}</span>
                      <ChevronRight className="w-4 h-4 opacity-50" />
                    </>
                  )}
                </Link>
              ))}
            </div>
          </nav>

          {/* Footer */}
          <div className="border-t p-3 space-y-2">
            <div className="flex items-center gap-2">
              <ThemeToggle />
              {sidebarOpen && (
                <Button variant="ghost" size="sm" asChild className="flex-1 justify-start gap-2 text-muted-foreground">
                  <a href="http://localhost:8081" target="_blank" rel="noopener noreferrer">
                    <ExternalLink className="w-4 h-4" />
                    Playground
                  </a>
                </Button>
              )}
            </div>
          </div>
        </aside>

        {/* Main content */}
        <main className="flex-1 overflow-auto bg-background">
          {children}
        </main>
      </div>
    </TooltipProvider>
  )
}
