import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { Box, ChevronRight } from 'lucide-react'
import { api } from '../api'

export default function TypeListPage() {
  const { data: typesData, isLoading } = useQuery({
    queryKey: ['types'],
    queryFn: () => api.getTypes(),
  })

  const { data: schema } = useQuery({
    queryKey: ['schema'],
    queryFn: () => api.getSchema(),
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
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Node Types</h1>
        <p className="text-gray-600">Browse all registered node types</p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
        {typesData?.types.map((type) => {
          const schemaType = schema?.node_types.find(t => t.type_id === type.type_id)

          return (
            <Link
              key={type.type_id}
              to={`/types/${type.type_id}`}
              className="bg-white rounded-lg shadow hover:shadow-md transition-shadow p-6"
            >
              <div className="flex items-start justify-between">
                <div className="flex items-center gap-3">
                  <div className="p-2 bg-blue-100 rounded-lg">
                    <Box className="w-6 h-6 text-blue-600" />
                  </div>
                  <div>
                    <h3 className="font-semibold text-gray-900">{type.name}</h3>
                    <p className="text-sm text-gray-500">Type ID: {type.type_id}</p>
                  </div>
                </div>
                <ChevronRight className="w-5 h-5 text-gray-400" />
              </div>

              {schemaType && (
                <div className="mt-4 pt-4 border-t border-gray-100">
                  <p className="text-sm text-gray-600 mb-2">Fields:</p>
                  <div className="flex flex-wrap gap-2">
                    {schemaType.fields.slice(0, 5).map((field) => (
                      <span
                        key={field.field_id}
                        className="px-2 py-1 bg-gray-100 rounded text-xs text-gray-700"
                      >
                        {field.name}
                      </span>
                    ))}
                    {schemaType.fields.length > 5 && (
                      <span className="px-2 py-1 text-xs text-gray-500">
                        +{schemaType.fields.length - 5} more
                      </span>
                    )}
                  </div>
                </div>
              )}
            </Link>
          )
        })}
      </div>
    </div>
  )
}
