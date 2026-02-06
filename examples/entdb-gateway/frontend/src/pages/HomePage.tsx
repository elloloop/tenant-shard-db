import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { Box, Database, GitBranch, Hash } from 'lucide-react'
import { api } from '../api'

export default function HomePage() {
  const { data: schema, isLoading } = useQuery({
    queryKey: ['schema'],
    queryFn: () => api.getSchema(),
  })

  const { data: typesData } = useQuery({
    queryKey: ['types'],
    queryFn: () => api.getTypes(),
  })

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600" />
      </div>
    )
  }

  return (
    <div className="p-8">
      {/* Header */}
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-gray-900 flex items-center gap-3">
          <Database className="w-8 h-8 text-blue-600" />
          EntDB Browser
        </h1>
        <p className="mt-2 text-gray-600">
          Browse and explore your EntDB data
        </p>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mb-8">
        <div className="bg-white rounded-lg shadow p-6">
          <div className="flex items-center gap-4">
            <div className="p-3 bg-blue-100 rounded-lg">
              <Box className="w-6 h-6 text-blue-600" />
            </div>
            <div>
              <p className="text-sm text-gray-600">Node Types</p>
              <p className="text-2xl font-bold">{schema?.node_types.length || 0}</p>
            </div>
          </div>
        </div>

        <div className="bg-white rounded-lg shadow p-6">
          <div className="flex items-center gap-4">
            <div className="p-3 bg-green-100 rounded-lg">
              <GitBranch className="w-6 h-6 text-green-600" />
            </div>
            <div>
              <p className="text-sm text-gray-600">Edge Types</p>
              <p className="text-2xl font-bold">{schema?.edge_types.length || 0}</p>
            </div>
          </div>
        </div>

        <div className="bg-white rounded-lg shadow p-6">
          <div className="flex items-center gap-4">
            <div className="p-3 bg-purple-100 rounded-lg">
              <Hash className="w-6 h-6 text-purple-600" />
            </div>
            <div>
              <p className="text-sm text-gray-600">Schema Fingerprint</p>
              <p className="text-sm font-mono text-gray-700 truncate">
                {schema?.fingerprint.slice(0, 16)}...
              </p>
            </div>
          </div>
        </div>
      </div>

      {/* Node Types */}
      <div className="bg-white rounded-lg shadow">
        <div className="px-6 py-4 border-b border-gray-200">
          <h2 className="text-lg font-semibold">Node Types</h2>
        </div>
        <div className="divide-y divide-gray-200">
          {typesData?.types.map((type) => (
            <Link
              key={type.type_id}
              to={`/types/${type.type_id}`}
              className="flex items-center justify-between px-6 py-4 hover:bg-gray-50"
            >
              <div className="flex items-center gap-4">
                <div className="p-2 bg-gray-100 rounded-lg">
                  <Box className="w-5 h-5 text-gray-600" />
                </div>
                <div>
                  <p className="font-medium text-gray-900">{type.name}</p>
                  <p className="text-sm text-gray-500">
                    {type.field_count} fields
                  </p>
                </div>
              </div>
              <span className="text-sm text-gray-400">
                Type ID: {type.type_id}
              </span>
            </Link>
          ))}
        </div>
      </div>

      {/* Edge Types */}
      {schema && schema.edge_types.length > 0 && (
        <div className="bg-white rounded-lg shadow mt-6">
          <div className="px-6 py-4 border-b border-gray-200">
            <h2 className="text-lg font-semibold">Edge Types</h2>
          </div>
          <div className="divide-y divide-gray-200">
            {schema.edge_types.map((edge) => (
              <div
                key={edge.edge_id}
                className="flex items-center justify-between px-6 py-4"
              >
                <div className="flex items-center gap-4">
                  <div className="p-2 bg-gray-100 rounded-lg">
                    <GitBranch className="w-5 h-5 text-gray-600" />
                  </div>
                  <div>
                    <p className="font-medium text-gray-900">{edge.name}</p>
                    <p className="text-sm text-gray-500">
                      Type {edge.from_type} → Type {edge.to_type}
                    </p>
                  </div>
                </div>
                <span className="text-sm text-gray-400">
                  Edge ID: {edge.edge_id}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
