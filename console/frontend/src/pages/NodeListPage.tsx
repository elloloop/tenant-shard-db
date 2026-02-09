import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useParams, Link } from 'react-router-dom'
import { ChevronLeft, ChevronRight, Eye, Network } from 'lucide-react'
import { api } from '../api'

export default function NodeListPage() {
  const { typeId } = useParams<{ typeId: string }>()
  const [offset, setOffset] = useState(0)
  const limit = 20

  const { data: schema } = useQuery({
    queryKey: ['schema'],
    queryFn: () => api.getSchema(),
  })

  const { data: nodesData, isLoading } = useQuery({
    queryKey: ['nodes', typeId, offset, limit],
    queryFn: () => api.listNodes({
      type_id: Number(typeId),
      offset,
      limit,
    }),
    enabled: !!typeId,
  })

  const schemaType = schema?.node_types.find(t => t.type_id === Number(typeId))

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
      <div className="mb-6">
        <Link
          to="/types"
          className="text-blue-600 hover:text-blue-800 text-sm flex items-center gap-1 mb-2"
        >
          <ChevronLeft className="w-4 h-4" />
          Back to Types
        </Link>
        <h1 className="text-2xl font-bold text-gray-900">
          {schemaType?.name || `Type ${typeId}`}
        </h1>
        <p className="text-gray-600">
          Browse nodes of this type
        </p>
      </div>

      {/* Schema info */}
      {schemaType && (
        <div className="bg-white rounded-lg shadow mb-6 p-4">
          <h3 className="font-medium text-gray-900 mb-3">Schema Fields</h3>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
            {schemaType.fields.map((field) => (
              <div
                key={field.field_id}
                className="flex items-center justify-between px-3 py-2 bg-gray-50 rounded"
              >
                <span className="text-sm font-medium text-gray-700">
                  {field.name}
                  {field.required && <span className="text-red-500">*</span>}
                </span>
                <span className="text-xs text-gray-500">{field.kind}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Nodes table */}
      <div className="bg-white rounded-lg shadow overflow-hidden">
        <table className="min-w-full divide-y divide-gray-200">
          <thead className="bg-gray-50">
            <tr>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                ID
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Owner
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Payload Preview
              </th>
              <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">
                Actions
              </th>
            </tr>
          </thead>
          <tbody className="bg-white divide-y divide-gray-200">
            {nodesData?.nodes.map((node) => (
              <tr key={node.node_id} className="hover:bg-gray-50">
                <td className="px-6 py-4 whitespace-nowrap">
                  <span className="font-mono text-sm text-gray-900">
                    {node.node_id.slice(0, 12)}...
                  </span>
                </td>
                <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                  {node.owner_actor}
                </td>
                <td className="px-6 py-4 text-sm text-gray-500">
                  <div className="max-w-md truncate">
                    {JSON.stringify(node.payload).slice(0, 80)}
                    {JSON.stringify(node.payload).length > 80 && '...'}
                  </div>
                </td>
                <td className="px-6 py-4 whitespace-nowrap text-right text-sm">
                  <div className="flex justify-end gap-2">
                    <Link
                      to={`/nodes/${node.node_id}`}
                      className="text-blue-600 hover:text-blue-800 p-1"
                      title="View details"
                    >
                      <Eye className="w-4 h-4" />
                    </Link>
                    <Link
                      to={`/graph/${node.node_id}`}
                      className="text-green-600 hover:text-green-800 p-1"
                      title="View graph"
                    >
                      <Network className="w-4 h-4" />
                    </Link>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>

        {/* Pagination */}
        <div className="bg-gray-50 px-6 py-3 flex items-center justify-between border-t border-gray-200">
          <div className="text-sm text-gray-500">
            Showing {offset + 1} - {offset + (nodesData?.nodes.length || 0)}
          </div>
          <div className="flex gap-2">
            <button
              onClick={() => setOffset(Math.max(0, offset - limit))}
              disabled={offset === 0}
              className="px-3 py-1 border rounded text-sm disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-100"
            >
              <ChevronLeft className="w-4 h-4" />
            </button>
            <button
              onClick={() => setOffset(offset + limit)}
              disabled={!nodesData?.has_more}
              className="px-3 py-1 border rounded text-sm disabled:opacity-50 disabled:cursor-not-allowed hover:bg-gray-100"
            >
              <ChevronRight className="w-4 h-4" />
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
