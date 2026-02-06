import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { Search, Eye, Network } from 'lucide-react'
import { api } from '../api'

export default function SearchPage() {
  const [query, setQuery] = useState('')
  const [searchTerm, setSearchTerm] = useState('')

  const { data: results, isLoading, isFetching } = useQuery({
    queryKey: ['search', searchTerm],
    queryFn: () => api.search(searchTerm),
    enabled: searchTerm.length > 0,
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (query.trim()) {
      setSearchTerm(query.trim())
    }
  }

  return (
    <div className="p-8">
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-gray-900">Search</h1>
        <p className="text-gray-600">
          Full-text search across your mailbox items
        </p>
      </div>

      {/* Search form */}
      <form onSubmit={handleSubmit} className="mb-8">
        <div className="flex gap-4">
          <div className="flex-1 relative">
            <Search className="absolute left-4 top-1/2 -translate-y-1/2 w-5 h-5 text-gray-400" />
            <input
              type="text"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Search mailbox items..."
              className="w-full pl-12 pr-4 py-3 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none"
            />
          </div>
          <button
            type="submit"
            disabled={!query.trim() || isFetching}
            className="px-6 py-3 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
          >
            {isFetching ? (
              <div className="animate-spin rounded-full h-5 w-5 border-b-2 border-white" />
            ) : (
              <Search className="w-5 h-5" />
            )}
            Search
          </button>
        </div>
      </form>

      {/* Results */}
      {isLoading && (
        <div className="flex items-center justify-center py-12">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600" />
        </div>
      )}

      {results && (
        <div>
          <p className="text-sm text-gray-600 mb-4">
            {results.results.length} results for "{results.query}"
          </p>

          {results.results.length === 0 ? (
            <div className="bg-white rounded-lg shadow p-8 text-center">
              <Search className="w-12 h-12 text-gray-300 mx-auto mb-4" />
              <p className="text-gray-500">No results found</p>
              <p className="text-sm text-gray-400 mt-1">
                Try different keywords or check your spelling
              </p>
            </div>
          ) : (
            <div className="bg-white rounded-lg shadow divide-y divide-gray-200">
              {results.results.map((result: any, index) => (
                <div
                  key={result.node_id || index}
                  className="p-4 hover:bg-gray-50"
                >
                  <div className="flex items-start justify-between">
                    <div className="flex-1">
                      <p className="font-medium text-gray-900">
                        {result.preview_text || result.title || 'Untitled'}
                      </p>
                      {result.snippet && (
                        <p className="text-sm text-gray-600 mt-1">
                          {result.snippet}
                        </p>
                      )}
                      <div className="flex items-center gap-4 mt-2">
                        <span className="text-xs text-gray-500">
                          Type: {result.node_type_id}
                        </span>
                        {result.node_id && (
                          <span className="text-xs text-gray-500 font-mono">
                            ID: {result.node_id.slice(0, 12)}...
                          </span>
                        )}
                        {result.is_read !== undefined && (
                          <span className={`text-xs px-2 py-0.5 rounded ${
                            result.is_read
                              ? 'bg-gray-100 text-gray-600'
                              : 'bg-blue-100 text-blue-600'
                          }`}>
                            {result.is_read ? 'Read' : 'Unread'}
                          </span>
                        )}
                      </div>
                    </div>
                    {result.node_id && (
                      <div className="flex gap-2 ml-4">
                        <Link
                          to={`/nodes/${result.node_id}`}
                          className="p-2 text-blue-600 hover:bg-blue-50 rounded"
                          title="View details"
                        >
                          <Eye className="w-4 h-4" />
                        </Link>
                        <Link
                          to={`/graph/${result.node_id}`}
                          className="p-2 text-green-600 hover:bg-green-50 rounded"
                          title="View graph"
                        >
                          <Network className="w-4 h-4" />
                        </Link>
                      </div>
                    )}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {!results && !searchTerm && (
        <div className="bg-white rounded-lg shadow p-8 text-center">
          <Search className="w-12 h-12 text-gray-300 mx-auto mb-4" />
          <p className="text-gray-500">Enter a search query to find items</p>
          <p className="text-sm text-gray-400 mt-1">
            Search uses SQLite FTS5 for full-text matching
          </p>
        </div>
      )}
    </div>
  )
}
