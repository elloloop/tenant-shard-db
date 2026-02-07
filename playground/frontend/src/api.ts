const API_BASE = '/api/v1'

export interface SchemaParseResponse {
  valid: boolean
  errors: string[]
  schema_data: Record<string, unknown> | null
  python_schema: string | null
  python_usage: string | null
  go_schema: string | null
  go_usage: string | null
  python_data: string | null
}

export interface SchemaExecuteResponse {
  success: boolean
  message: string
  errors: string[]
  created_node_ids: string[]
  python_schema: string | null
  go_schema: string | null
}

export interface AIPromptResponse {
  template: string
  usage: string
  example_requirements: string
}

export async function parseSchema(content: string, format: 'yaml' | 'json' = 'yaml'): Promise<SchemaParseResponse> {
  const res = await fetch(`${API_BASE}/schema/parse`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ content, format }),
  })
  if (!res.ok) {
    throw new Error(`Parse failed: ${res.statusText}`)
  }
  return res.json()
}

export async function executeSchema(content: string, format: 'yaml' | 'json' = 'yaml'): Promise<SchemaExecuteResponse> {
  const res = await fetch(`${API_BASE}/schema/execute`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ content, format, execute_data: true }),
  })
  if (!res.ok) {
    throw new Error(`Execute failed: ${res.statusText}`)
  }
  return res.json()
}

export async function getAIPromptTemplate(): Promise<AIPromptResponse> {
  const res = await fetch(`${API_BASE}/schema/prompt`)
  if (!res.ok) {
    throw new Error(`Failed to get prompt: ${res.statusText}`)
  }
  return res.json()
}

export async function generateAIPrompt(requirements: string): Promise<{ prompt: string }> {
  const res = await fetch(`${API_BASE}/schema/prompt`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ requirements }),
  })
  if (!res.ok) {
    throw new Error(`Failed to generate prompt: ${res.statusText}`)
  }
  return res.json()
}
