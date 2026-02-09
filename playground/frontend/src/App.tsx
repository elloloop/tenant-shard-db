import { useState, useCallback, useEffect } from 'react'
import { Plus, Link, Trash2, Check, AlertCircle, ExternalLink, Sun, Moon, Monitor, Copy } from 'lucide-react'
import { createNode, createEdge } from './api'
import { Button } from '@/components/ui/button'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import { useTheme } from '@/components/theme-provider'

// --- localStorage helpers ---

interface SessionNode {
  node_id: string
  type_id: number
  type_name: string
  payload: Record<string, unknown>
  created_at: string
}

interface SessionEdge {
  edge_type_id: number
  from_id: string
  to_id: string
  created_at: string
}

const STORAGE_KEY_NODES = 'entdb-playground-nodes'
const STORAGE_KEY_EDGES = 'entdb-playground-edges'

function loadNodes(): SessionNode[] {
  try {
    return JSON.parse(localStorage.getItem(STORAGE_KEY_NODES) || '[]')
  } catch {
    return []
  }
}

function saveNodes(nodes: SessionNode[]) {
  localStorage.setItem(STORAGE_KEY_NODES, JSON.stringify(nodes))
}

function loadEdges(): SessionEdge[] {
  try {
    return JSON.parse(localStorage.getItem(STORAGE_KEY_EDGES) || '[]')
  } catch {
    return []
  }
}

function saveEdges(edges: SessionEdge[]) {
  localStorage.setItem(STORAGE_KEY_EDGES, JSON.stringify(edges))
}

// --- Theme toggle ---

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
        {theme === 'light' ? 'Light mode' : theme === 'dark' ? 'Dark mode' : 'System mode'}
      </TooltipContent>
    </Tooltip>
  )
}

// --- Main App ---

function App() {
  const [tab, setTab] = useState<'node' | 'edge'>('node')

  // Node form
  const [typeId, setTypeId] = useState('1')
  const [typeName, setTypeName] = useState('User')
  const [payload, setPayload] = useState('{\n  "name": "Alice",\n  "email": "alice@example.com"\n}')
  const [nodeError, setNodeError] = useState('')
  const [nodeSuccess, setNodeSuccess] = useState('')
  const [nodeLoading, setNodeLoading] = useState(false)

  // Edge form
  const [edgeTypeId, setEdgeTypeId] = useState('101')
  const [fromId, setFromId] = useState('')
  const [toId, setToId] = useState('')
  const [edgeError, setEdgeError] = useState('')
  const [edgeSuccess, setEdgeSuccess] = useState('')
  const [edgeLoading, setEdgeLoading] = useState(false)

  // Session history
  const [nodes, setNodes] = useState<SessionNode[]>(loadNodes)
  const [edges, setEdges] = useState<SessionEdge[]>(loadEdges)

  useEffect(() => { saveNodes(nodes) }, [nodes])
  useEffect(() => { saveEdges(edges) }, [edges])

  const handleCreateNode = useCallback(async () => {
    setNodeError('')
    setNodeSuccess('')

    const tid = parseInt(typeId, 10)
    if (isNaN(tid) || tid < 1) {
      setNodeError('Type ID must be a positive integer')
      return
    }

    let parsed: Record<string, unknown>
    try {
      parsed = JSON.parse(payload)
    } catch {
      setNodeError('Invalid JSON payload')
      return
    }

    setNodeLoading(true)
    try {
      const res = await createNode(tid, typeName, parsed)
      if (res.success && res.data?.node_id) {
        setNodeSuccess(`Created node ${res.data.node_id}`)
        setNodes(prev => [{
          node_id: res.data!.node_id!,
          type_id: tid,
          type_name: typeName || `Type_${tid}`,
          payload: parsed,
          created_at: new Date().toISOString(),
        }, ...prev])
      } else {
        setNodeError(res.message || 'Failed to create node')
      }
    } catch (e) {
      setNodeError(String(e))
    } finally {
      setNodeLoading(false)
    }
  }, [typeId, typeName, payload])

  const handleCreateEdge = useCallback(async () => {
    setEdgeError('')
    setEdgeSuccess('')

    const eid = parseInt(edgeTypeId, 10)
    if (isNaN(eid) || eid < 1) {
      setEdgeError('Edge type ID must be a positive integer')
      return
    }
    if (!fromId || !toId) {
      setEdgeError('Select both source and target nodes')
      return
    }

    setEdgeLoading(true)
    try {
      const res = await createEdge(eid, fromId, toId)
      if (res.success) {
        setEdgeSuccess(`Created edge from ${fromId.slice(0, 8)}... to ${toId.slice(0, 8)}...`)
        setEdges(prev => [{
          edge_type_id: eid,
          from_id: fromId,
          to_id: toId,
          created_at: new Date().toISOString(),
        }, ...prev])
      } else {
        setEdgeError(res.message || 'Failed to create edge')
      }
    } catch (e) {
      setEdgeError(String(e))
    } finally {
      setEdgeLoading(false)
    }
  }, [edgeTypeId, fromId, toId])

  const clearHistory = useCallback(() => {
    setNodes([])
    setEdges([])
  }, [])

  const copyNodeId = useCallback(async (id: string) => {
    await navigator.clipboard.writeText(id)
  }, [])

  return (
    <TooltipProvider>
      <div className="h-screen flex flex-col bg-background text-foreground">
        {/* Header */}
        <header className="flex-none h-14 border-b flex items-center justify-between px-4">
          <h1 className="text-lg font-semibold">EntDB Playground</h1>
          <div className="flex items-center gap-2">
            <ThemeToggle />
            <Button variant="ghost" size="sm" asChild className="gap-1 text-muted-foreground">
              <a href="http://localhost:8080" target="_blank" rel="noopener noreferrer">
                <ExternalLink className="w-4 h-4" />
                Console
              </a>
            </Button>
          </div>
        </header>

        <div className="flex-1 flex min-h-0">
          {/* Left panel - Form */}
          <div className="w-1/2 border-r flex flex-col">
            {/* Tab bar */}
            <div className="flex-none border-b flex">
              <button
                className={`px-4 py-2.5 text-sm font-medium border-b-2 transition-colors ${
                  tab === 'node'
                    ? 'border-primary text-primary'
                    : 'border-transparent text-muted-foreground hover:text-foreground'
                }`}
                onClick={() => setTab('node')}
              >
                <Plus className="w-4 h-4 inline mr-1.5 -mt-0.5" />
                Create Node
              </button>
              <button
                className={`px-4 py-2.5 text-sm font-medium border-b-2 transition-colors ${
                  tab === 'edge'
                    ? 'border-primary text-primary'
                    : 'border-transparent text-muted-foreground hover:text-foreground'
                }`}
                onClick={() => setTab('edge')}
              >
                <Link className="w-4 h-4 inline mr-1.5 -mt-0.5" />
                Create Edge
              </button>
            </div>

            <div className="flex-1 overflow-y-auto p-4">
              {tab === 'node' ? (
                <div className="space-y-4">
                  <div className="grid grid-cols-2 gap-3">
                    <div>
                      <label className="block text-sm font-medium mb-1">Type ID</label>
                      <input
                        type="number"
                        min="1"
                        value={typeId}
                        onChange={e => setTypeId(e.target.value)}
                        className="w-full rounded-md border bg-background px-3 py-2 text-sm"
                        placeholder="1"
                      />
                    </div>
                    <div>
                      <label className="block text-sm font-medium mb-1">Type Name</label>
                      <input
                        type="text"
                        value={typeName}
                        onChange={e => setTypeName(e.target.value)}
                        className="w-full rounded-md border bg-background px-3 py-2 text-sm"
                        placeholder="User"
                      />
                    </div>
                  </div>

                  <div>
                    <label className="block text-sm font-medium mb-1">Payload (JSON)</label>
                    <textarea
                      value={payload}
                      onChange={e => setPayload(e.target.value)}
                      className="w-full rounded-md border bg-background px-3 py-2 text-sm font-mono min-h-[200px] resize-y"
                      placeholder='{ "key": "value" }'
                      spellCheck={false}
                    />
                  </div>

                  {nodeError && (
                    <div className="flex items-center gap-2 text-sm text-destructive">
                      <AlertCircle className="w-4 h-4 flex-shrink-0" />
                      {nodeError}
                    </div>
                  )}
                  {nodeSuccess && (
                    <div className="flex items-center gap-2 text-sm text-green-600 dark:text-green-400">
                      <Check className="w-4 h-4 flex-shrink-0" />
                      {nodeSuccess}
                    </div>
                  )}

                  <Button onClick={handleCreateNode} disabled={nodeLoading} className="w-full">
                    {nodeLoading ? 'Creating...' : 'Create Node'}
                  </Button>
                </div>
              ) : (
                <div className="space-y-4">
                  <div>
                    <label className="block text-sm font-medium mb-1">Edge Type ID</label>
                    <input
                      type="number"
                      min="1"
                      value={edgeTypeId}
                      onChange={e => setEdgeTypeId(e.target.value)}
                      className="w-full rounded-md border bg-background px-3 py-2 text-sm"
                      placeholder="101"
                    />
                  </div>

                  <div>
                    <label className="block text-sm font-medium mb-1">From Node</label>
                    {nodes.length > 0 ? (
                      <select
                        value={fromId}
                        onChange={e => setFromId(e.target.value)}
                        className="w-full rounded-md border bg-background px-3 py-2 text-sm"
                      >
                        <option value="">Select source node...</option>
                        {nodes.map(n => (
                          <option key={n.node_id} value={n.node_id}>
                            {n.type_name} - {n.node_id.slice(0, 12)}...
                          </option>
                        ))}
                      </select>
                    ) : (
                      <p className="text-sm text-muted-foreground py-2">
                        Create some nodes first to link them.
                      </p>
                    )}
                  </div>

                  <div>
                    <label className="block text-sm font-medium mb-1">To Node</label>
                    {nodes.length > 0 ? (
                      <select
                        value={toId}
                        onChange={e => setToId(e.target.value)}
                        className="w-full rounded-md border bg-background px-3 py-2 text-sm"
                      >
                        <option value="">Select target node...</option>
                        {nodes.map(n => (
                          <option key={n.node_id} value={n.node_id}>
                            {n.type_name} - {n.node_id.slice(0, 12)}...
                          </option>
                        ))}
                      </select>
                    ) : (
                      <p className="text-sm text-muted-foreground py-2">
                        Create some nodes first to link them.
                      </p>
                    )}
                  </div>

                  {edgeError && (
                    <div className="flex items-center gap-2 text-sm text-destructive">
                      <AlertCircle className="w-4 h-4 flex-shrink-0" />
                      {edgeError}
                    </div>
                  )}
                  {edgeSuccess && (
                    <div className="flex items-center gap-2 text-sm text-green-600 dark:text-green-400">
                      <Check className="w-4 h-4 flex-shrink-0" />
                      {edgeSuccess}
                    </div>
                  )}

                  <Button
                    onClick={handleCreateEdge}
                    disabled={edgeLoading || nodes.length === 0}
                    className="w-full"
                  >
                    {edgeLoading ? 'Creating...' : 'Create Edge'}
                  </Button>
                </div>
              )}
            </div>
          </div>

          {/* Right panel - Session History */}
          <div className="w-1/2 flex flex-col">
            <div className="flex-none h-10 bg-muted/30 border-b flex items-center justify-between px-4">
              <span className="text-sm text-muted-foreground font-medium">
                Session History ({nodes.length} nodes, {edges.length} edges)
              </span>
              {(nodes.length > 0 || edges.length > 0) && (
                <Button variant="ghost" size="sm" onClick={clearHistory} className="h-7 gap-1 text-xs">
                  <Trash2 className="w-3 h-3" />
                  Clear
                </Button>
              )}
            </div>

            <div className="flex-1 overflow-y-auto">
              {nodes.length === 0 && edges.length === 0 ? (
                <div className="flex items-center justify-center h-full text-muted-foreground text-sm">
                  Created nodes and edges will appear here.
                </div>
              ) : (
                <div className="divide-y">
                  {nodes.map(n => (
                    <div key={n.node_id} className="px-4 py-3 hover:bg-muted/30">
                      <div className="flex items-center justify-between mb-1">
                        <span className="text-sm font-medium">{n.type_name}</span>
                        <span className="text-xs text-muted-foreground">
                          {new Date(n.created_at).toLocaleTimeString()}
                        </span>
                      </div>
                      <div className="flex items-center gap-1.5 mb-1.5">
                        <code className="text-xs font-mono text-muted-foreground">
                          {n.node_id}
                        </code>
                        <button
                          onClick={() => copyNodeId(n.node_id)}
                          className="text-muted-foreground hover:text-foreground"
                        >
                          <Copy className="w-3 h-3" />
                        </button>
                      </div>
                      <pre className="text-xs font-mono bg-muted/50 rounded px-2 py-1.5 overflow-x-auto">
                        {JSON.stringify(n.payload, null, 2)}
                      </pre>
                    </div>
                  ))}

                  {edges.map((e, i) => (
                    <div key={`edge-${i}`} className="px-4 py-3 hover:bg-muted/30">
                      <div className="flex items-center justify-between mb-1">
                        <span className="text-sm font-medium">
                          <Link className="w-3 h-3 inline mr-1 -mt-0.5" />
                          Edge (type {e.edge_type_id})
                        </span>
                        <span className="text-xs text-muted-foreground">
                          {new Date(e.created_at).toLocaleTimeString()}
                        </span>
                      </div>
                      <div className="text-xs font-mono text-muted-foreground">
                        {e.from_id.slice(0, 12)}... → {e.to_id.slice(0, 12)}...
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        </div>
      </div>
    </TooltipProvider>
  )
}

export default App
