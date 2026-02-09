const API_BASE = '/api/v1'

export interface PlaygroundResponse {
  success: boolean
  message: string
  data: {
    node_id?: string
    type_id?: number
    type_name?: string
    payload?: Record<string, unknown>
    edge_type_id?: number
    from_id?: string
    to_id?: string
    props?: Record<string, unknown>
  } | null
}

export async function createNode(
  typeId: number,
  typeName: string,
  payload: Record<string, unknown>,
): Promise<PlaygroundResponse> {
  const res = await fetch(`${API_BASE}/nodes`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ type_id: typeId, type_name: typeName, payload }),
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({ detail: res.statusText }))
    throw new Error(err.detail || `HTTP ${res.status}`)
  }
  return res.json()
}

export async function createEdge(
  edgeTypeId: number,
  fromId: string,
  toId: string,
  props?: Record<string, unknown>,
): Promise<PlaygroundResponse> {
  const res = await fetch(`${API_BASE}/edges`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ edge_type_id: edgeTypeId, from_id: fromId, to_id: toId, props }),
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({ detail: res.statusText }))
    throw new Error(err.detail || `HTTP ${res.status}`)
  }
  return res.json()
}
