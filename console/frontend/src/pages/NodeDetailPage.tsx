import { useQuery } from '@tanstack/react-query'
import { useParams, Link } from 'react-router-dom'
import { ChevronLeft, ArrowRight, ArrowLeft, Network, Copy, Check } from 'lucide-react'
import { useState } from 'react'
import { api } from '../api'

export default function NodeDetailPage() {
  const { nodeId } = useParams<{ nodeId: string }>()
  const [copied, setCopied] = useState(false)

  const { data: node, isLoading } = useQuery({
    queryKey: ['node', nodeId],
    queryFn: () => api.getNode(nodeId!),
    enabled: !!nodeId,
  })

  const { data: schema } = useQuery({
    queryKey: ['schema'],
    queryFn: () => api.getSchema(),
  })

  const { data: outEdges } = useQuery({
    queryKey: ['edges', 'out', nodeId],
    queryFn: () => api.getOutgoingEdges(nodeId!),
    enabled: !!nodeId,
  })

  const { data: inEdges } = useQuery({
    queryKey: ['edges', 'in', nodeId],
    queryFn: () => api.getIncomingEdges(nodeId!),
    enabled: !!nodeId,
  })

  const schemaType = node && schema?.node_types.find(t => t.type_id === node.type_id)

  const copyToClipboard = () => {
    navigator.clipboard.writeText(nodeId || '')
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600" />
      </div>
    )
  }

  if (!node) {
    return (
      <div className="p-8">
        <p className="text-gray-600">Node not found</p>
      </div>
    )
  }

  return (
    <div className="p-8">
      {/* Header */}
      <div className="mb-6">
        <Link
          to={`/types/${node.type_id}`}
          className="text-blue-600 hover:text-blue-800 text-sm flex items-center gap-1 mb-2"
        >
          <ChevronLeft className="w-4 h-4" />
          Back to {schemaType?.name || 'Type'}
        </Link>

        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold text-gray-900">
              {schemaType?.name || 'Node'} Details
            </h1>
            <div className="flex items-center gap-2 mt-1">
              <code className="text-sm text-gray-600 bg-gray-100 px-2 py-1 rounded font-mono">
                {node.node_id}
              </code>
              <button
                onClick={copyToClipboard}
                className="text-gray-400 hover:text-gray-600"
                title="Copy ID"
              >
                {copied ? (
                  <Check className="w-4 h-4 text-green-600" />
                ) : (
                  <Copy className="w-4 h-4" />
                )}
              </button>
            </div>
          </div>

          <Link
            to={`/graph/${node.node_id}`}
            className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700"
          >
            <Network className="w-4 h-4" />
            View Graph
          </Link>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Main content */}
        <div className="lg:col-span-2 space-y-6">
          {/* Payload */}
          <div className="bg-white rounded-lg shadow">
            <div className="px-6 py-4 border-b border-gray-200">
              <h2 className="font-semibold text-gray-900">Payload</h2>
            </div>
            <div className="p-6">
              <table className="min-w-full">
                <tbody className="divide-y divide-gray-100">
                  {Object.entries(node.payload).map(([key, value]) => {
                    const field = schemaType?.fields.find(f => f.name === key)
                    return (
                      <tr key={key}>
                        <td className="py-3 pr-4 text-sm font-medium text-gray-700 w-1/3">
                          {key}
                          {field?.required && (
                            <span className="text-red-500 ml-1">*</span>
                          )}
                          {field && (
                            <span className="ml-2 text-xs text-gray-400">
                              ({field.kind})
                            </span>
                          )}
                        </td>
                        <td className="py-3 text-sm text-gray-900">
                          {typeof value === 'object' ? (
                            <pre className="bg-gray-50 p-2 rounded text-xs overflow-x-auto">
                              {JSON.stringify(value, null, 2)}
                            </pre>
                          ) : (
                            String(value)
                          )}
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
          </div>

          {/* Raw JSON */}
          <div className="bg-white rounded-lg shadow">
            <div className="px-6 py-4 border-b border-gray-200">
              <h2 className="font-semibold text-gray-900">Raw JSON</h2>
            </div>
            <div className="p-6">
              <pre className="bg-gray-900 text-gray-100 p-4 rounded-lg text-sm overflow-x-auto">
                {JSON.stringify(node, null, 2)}
              </pre>
            </div>
          </div>
        </div>

        {/* Sidebar */}
        <div className="space-y-6">
          {/* Metadata */}
          <div className="bg-white rounded-lg shadow">
            <div className="px-6 py-4 border-b border-gray-200">
              <h2 className="font-semibold text-gray-900">Metadata</h2>
            </div>
            <div className="p-6 space-y-4">
              <div>
                <p className="text-xs text-gray-500 uppercase">Type</p>
                <p className="text-sm text-gray-900">{schemaType?.name} ({node.type_id})</p>
              </div>
              <div>
                <p className="text-xs text-gray-500 uppercase">Tenant</p>
                <p className="text-sm text-gray-900">{node.tenant_id}</p>
              </div>
              <div>
                <p className="text-xs text-gray-500 uppercase">Owner</p>
                <p className="text-sm text-gray-900">{node.owner_actor}</p>
              </div>
              {node.created_at && (
                <div>
                  <p className="text-xs text-gray-500 uppercase">Created</p>
                  <p className="text-sm text-gray-900">
                    {new Date(node.created_at * 1000).toLocaleString()}
                  </p>
                </div>
              )}
            </div>
          </div>

          {/* Outgoing Edges */}
          <div className="bg-white rounded-lg shadow">
            <div className="px-6 py-4 border-b border-gray-200">
              <h2 className="font-semibold text-gray-900 flex items-center gap-2">
                <ArrowRight className="w-4 h-4" />
                Outgoing Edges ({outEdges?.length || 0})
              </h2>
            </div>
            <div className="divide-y divide-gray-100">
              {outEdges?.length === 0 && (
                <p className="p-4 text-sm text-gray-500">No outgoing edges</p>
              )}
              {outEdges?.map((edge) => {
                const edgeType = schema?.edge_types.find(e => e.edge_id === edge.edge_type_id)
                return (
                  <Link
                    key={edge.edge_type_id + edge.to_node_id}
                    to={`/nodes/${edge.to_node_id}`}
                    className="flex items-center justify-between p-4 hover:bg-gray-50"
                  >
                    <div>
                      <p className="text-sm font-medium text-gray-900">
                        {edgeType?.name || `Edge ${edge.edge_type_id}`}
                      </p>
                      <p className="text-xs text-gray-500 font-mono">
                        → {edge.to_node_id.slice(0, 12)}...
                      </p>
                    </div>
                    <ArrowRight className="w-4 h-4 text-gray-400" />
                  </Link>
                )
              })}
            </div>
          </div>

          {/* Incoming Edges */}
          <div className="bg-white rounded-lg shadow">
            <div className="px-6 py-4 border-b border-gray-200">
              <h2 className="font-semibold text-gray-900 flex items-center gap-2">
                <ArrowLeft className="w-4 h-4" />
                Incoming Edges ({inEdges?.length || 0})
              </h2>
            </div>
            <div className="divide-y divide-gray-100">
              {inEdges?.length === 0 && (
                <p className="p-4 text-sm text-gray-500">No incoming edges</p>
              )}
              {inEdges?.map((edge) => {
                const edgeType = schema?.edge_types.find(e => e.edge_id === edge.edge_type_id)
                return (
                  <Link
                    key={edge.edge_type_id + edge.from_node_id}
                    to={`/nodes/${edge.from_node_id}`}
                    className="flex items-center justify-between p-4 hover:bg-gray-50"
                  >
                    <div>
                      <p className="text-sm font-medium text-gray-900">
                        {edgeType?.name || `Edge ${edge.edge_type_id}`}
                      </p>
                      <p className="text-xs text-gray-500 font-mono">
                        ← {edge.from_node_id.slice(0, 12)}...
                      </p>
                    </div>
                    <ArrowLeft className="w-4 h-4 text-gray-400" />
                  </Link>
                )
              })}
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
