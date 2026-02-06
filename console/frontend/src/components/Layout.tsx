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
  Settings,
} from 'lucide-react'
import { api } from '../api'

interface LayoutProps {
  children: ReactNode
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
    <div className="flex h-screen">
      {/* Sidebar */}
      <aside
        className={`${
          sidebarOpen ? 'w-64' : 'w-16'
        } bg-gray-900 text-white flex flex-col transition-all duration-200`}
      >
        {/* Logo */}
        <div className="h-16 flex items-center justify-between px-4 border-b border-gray-800">
          {sidebarOpen && (
            <Link to="/" className="flex items-center gap-2 font-bold text-lg">
              <Database className="w-6 h-6 text-blue-400" />
              EntDB Console
            </Link>
          )}
          <button
            onClick={() => setSidebarOpen(!sidebarOpen)}
            className="p-2 rounded hover:bg-gray-800"
          >
            {sidebarOpen ? <X className="w-5 h-5" /> : <Menu className="w-5 h-5" />}
          </button>
        </div>

        {/* Navigation */}
        <nav className="flex-1 overflow-y-auto py-4">
          <div className="px-4 mb-4">
            <Link
              to="/search"
              className={`flex items-center gap-3 px-3 py-2 rounded-lg ${
                isActive('/search')
                  ? 'bg-blue-600'
                  : 'hover:bg-gray-800'
              }`}
            >
              <Search className="w-5 h-5" />
              {sidebarOpen && <span>Search</span>}
            </Link>
          </div>

          {sidebarOpen && (
            <div className="px-4 mb-2">
              <h3 className="text-xs font-semibold text-gray-500 uppercase tracking-wide">
                Node Types
              </h3>
            </div>
          )}

          <div className="px-2">
            {typesData?.types.map((type) => (
              <Link
                key={type.type_id}
                to={`/types/${type.type_id}`}
                className={`flex items-center gap-3 px-3 py-2 rounded-lg mb-1 ${
                  location.pathname === `/types/${type.type_id}`
                    ? 'bg-blue-600'
                    : 'hover:bg-gray-800'
                }`}
              >
                <Box className="w-4 h-4 text-gray-400" />
                {sidebarOpen && (
                  <>
                    <span className="flex-1">{type.name}</span>
                    <ChevronRight className="w-4 h-4 text-gray-500" />
                  </>
                )}
              </Link>
            ))}
          </div>
        </nav>

        {/* Footer */}
        <div className="border-t border-gray-800 p-4">
          <Link
            to="/settings"
            className="flex items-center gap-3 px-3 py-2 rounded-lg hover:bg-gray-800"
          >
            <Settings className="w-5 h-5" />
            {sidebarOpen && <span>Settings</span>}
          </Link>
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 overflow-auto">
        {children}
      </main>
    </div>
  )
}
