/**
 * API client for EntDB Gateway.
 */

const API_BASE = '/api/v1'

export interface Node {
  id: string
  type_id: number
  tenant_id: string
  payload: Record<string, unknown>
  owner_actor: string
  created_at?: number
  updated_at?: number
}

export interface Edge {
  id: string
  edge_type_id: number
  from_id: string
  to_id: string
  tenant_id: string
  payload?: Record<string, unknown>
}

export interface SchemaField {
  field_id: number
  name: string
  kind: string
  required?: boolean
  deprecated?: boolean
}

export interface SchemaType {
  type_id: number
  name: string
  fields: SchemaField[]
  deprecated?: boolean
}

export interface Schema {
  node_types: SchemaType[]
  edge_types: { edge_id: number; name: string; from_type: number; to_type: number }[]
  fingerprint: string
}

export interface PaginatedResponse<T> {
  items: T[]
  total: number
  offset: number
  limit: number
  has_more: boolean
}

export interface GraphData {
  root_id: string
  nodes: Node[]
  edges: Edge[]
}

class ApiClient {
  private headers: Record<string, string> = {}

  setTenant(tenantId: string) {
    this.headers['X-Tenant-ID'] = tenantId
  }

  setActor(actor: string) {
    this.headers['X-Actor'] = actor
  }

  private async fetch<T>(path: string, options: RequestInit = {}): Promise<T> {
    const response = await fetch(`${API_BASE}${path}`, {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        ...this.headers,
        ...options.headers,
      },
    })

    if (!response.ok) {
      const error = await response.json().catch(() => ({ detail: 'Unknown error' }))
      throw new Error(error.detail || `HTTP ${response.status}`)
    }

    return response.json()
  }

  // Schema
  async getSchema(): Promise<Schema> {
    return this.fetch('/schema')
  }

  async getTypeSchema(typeId: number): Promise<SchemaType> {
    return this.fetch(`/schema/types/${typeId}`)
  }

  // Nodes
  async listNodes(params: {
    type_id?: number
    offset?: number
    limit?: number
  } = {}): Promise<PaginatedResponse<Node>> {
    const searchParams = new URLSearchParams()
    if (params.type_id !== undefined) searchParams.set('type_id', String(params.type_id))
    if (params.offset !== undefined) searchParams.set('offset', String(params.offset))
    if (params.limit !== undefined) searchParams.set('limit', String(params.limit))

    return this.fetch(`/nodes?${searchParams}`)
  }

  async getNode(nodeId: string): Promise<Node> {
    return this.fetch(`/nodes/${nodeId}`)
  }

  async createNode(data: {
    type_id: number
    payload: Record<string, unknown>
  }): Promise<Node> {
    return this.fetch('/nodes', {
      method: 'POST',
      body: JSON.stringify(data),
    })
  }

  async updateNode(nodeId: string, payload: Record<string, unknown>): Promise<Node> {
    return this.fetch(`/nodes/${nodeId}`, {
      method: 'PATCH',
      body: JSON.stringify({ payload }),
    })
  }

  async deleteNode(nodeId: string): Promise<void> {
    await fetch(`${API_BASE}/nodes/${nodeId}`, {
      method: 'DELETE',
      headers: this.headers,
    })
  }

  // Edges
  async getOutgoingEdges(nodeId: string, edgeTypeId?: number): Promise<Edge[]> {
    const params = edgeTypeId !== undefined ? `?edge_type_id=${edgeTypeId}` : ''
    return this.fetch(`/nodes/${nodeId}/edges/out${params}`)
  }

  async getIncomingEdges(nodeId: string, edgeTypeId?: number): Promise<Edge[]> {
    const params = edgeTypeId !== undefined ? `?edge_type_id=${edgeTypeId}` : ''
    return this.fetch(`/nodes/${nodeId}/edges/in${params}`)
  }

  // Search
  async search(query: string, limit = 50): Promise<{ query: string; results: unknown[] }> {
    return this.fetch(`/search?q=${encodeURIComponent(query)}&limit=${limit}`)
  }

  // Graph
  async getGraph(nodeId: string, depth = 1): Promise<GraphData> {
    return this.fetch(`/browse/graph/${nodeId}?depth=${depth}`)
  }

  // Browse
  async getTypes(): Promise<{ types: { type_id: number; name: string; field_count: number }[] }> {
    return this.fetch('/browse/types')
  }
}

export const api = new ApiClient()
