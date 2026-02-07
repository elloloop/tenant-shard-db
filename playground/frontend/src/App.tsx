import { useState, useCallback, useEffect } from 'react'
import { useMutation } from '@tanstack/react-query'
import { Play, Wand2, Copy, Check, AlertCircle, ExternalLink, FileCode, ChevronDown } from 'lucide-react'
import { parseSchema, executeSchema, getAIPromptTemplate, SchemaParseResponse } from './api'
import YamlEditor from './components/YamlEditor'
import CodePreview from './components/CodePreview'
import AIPromptModal from './components/AIPromptModal'

const DEFAULT_SCHEMA = `# EntDB Playground Schema
# Define your node types, edge types, and sample data

version: 1
tenant: playground

node_types:
  - type_id: 1
    name: User
    description: Application user
    fields:
      - id: 1
        name: email
        kind: string
        required: true
        indexed: true
      - id: 2
        name: name
        kind: string
        required: true
        searchable: true
      - id: 3
        name: created_at
        kind: int64

  - type_id: 2
    name: Post
    description: Blog post
    fields:
      - id: 1
        name: title
        kind: string
        required: true
        searchable: true
      - id: 2
        name: content
        kind: string
      - id: 3
        name: status
        kind: enum
        values: [draft, published, archived]
        default: draft
      - id: 4
        name: created_at
        kind: int64

edge_types:
  - edge_id: 101
    name: authored
    description: User authored a post
    from_type: 1  # User
    to_type: 2    # Post

data:
  - operation: create_node
    type_id: 1
    as: user1
    payload:
      email: alice@example.com
      name: Alice Smith

  - operation: create_node
    type_id: 2
    as: post1
    payload:
      title: Hello World
      content: My first post!
      status: published

  - operation: create_edge
    edge_id: 101
    from_id: "@user1"
    to_id: "@post1"
`

function App() {
  const [yaml, setYaml] = useState(DEFAULT_SCHEMA)
  const [parseResult, setParseResult] = useState<SchemaParseResponse | null>(null)
  const [showAIModal, setShowAIModal] = useState(false)
  const [copied, setCopied] = useState(false)
  const [executeResult, setExecuteResult] = useState<{ success: boolean; message: string; ids: string[] } | null>(null)

  // Parse mutation
  const parseMutation = useMutation({
    mutationFn: (content: string) => parseSchema(content, 'yaml'),
    onSuccess: (data) => {
      setParseResult(data)
      setExecuteResult(null)
    },
  })

  // Execute mutation
  const executeMutation = useMutation({
    mutationFn: (content: string) => executeSchema(content, 'yaml'),
    onSuccess: (data) => {
      setExecuteResult({
        success: data.success,
        message: data.message,
        ids: data.created_node_ids,
      })
    },
  })

  // Auto-parse on change (debounced)
  useEffect(() => {
    const timer = setTimeout(() => {
      if (yaml.trim()) {
        parseMutation.mutate(yaml)
      }
    }, 500)
    return () => clearTimeout(timer)
  }, [yaml])

  const handleExecute = useCallback(() => {
    if (parseResult?.valid) {
      executeMutation.mutate(yaml)
    }
  }, [yaml, parseResult])

  const handleCopyPrompt = useCallback(async () => {
    try {
      const data = await getAIPromptTemplate()
      await navigator.clipboard.writeText(data.template)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch (e) {
      console.error('Failed to copy:', e)
    }
  }, [])

  const handleSchemaGenerated = useCallback((schema: string) => {
    setYaml(schema)
    setShowAIModal(false)
  }, [])

  return (
    <div className="h-screen flex flex-col bg-slate-900 text-slate-100">
      {/* Header */}
      <header className="flex-none h-14 border-b border-slate-700 flex items-center justify-between px-4">
        <div className="flex items-center gap-3">
          <FileCode className="w-6 h-6 text-blue-400" />
          <h1 className="text-lg font-semibold">EntDB Playground</h1>
          <span className="text-xs text-slate-500 bg-slate-800 px-2 py-0.5 rounded">Interactive SDK Simulator</span>
        </div>

        <div className="flex items-center gap-2">
          <button
            onClick={() => setShowAIModal(true)}
            className="flex items-center gap-2 px-3 py-1.5 text-sm bg-purple-600 hover:bg-purple-500 rounded-md transition-colors"
          >
            <Wand2 className="w-4 h-4" />
            AI Generate
          </button>

          <button
            onClick={handleCopyPrompt}
            className="flex items-center gap-2 px-3 py-1.5 text-sm bg-slate-700 hover:bg-slate-600 rounded-md transition-colors"
          >
            {copied ? <Check className="w-4 h-4 text-green-400" /> : <Copy className="w-4 h-4" />}
            Copy Prompt
          </button>

          <button
            onClick={handleExecute}
            disabled={!parseResult?.valid || executeMutation.isPending}
            className="flex items-center gap-2 px-4 py-1.5 text-sm bg-green-600 hover:bg-green-500 disabled:bg-slate-700 disabled:text-slate-500 rounded-md transition-colors"
          >
            <Play className="w-4 h-4" />
            {executeMutation.isPending ? 'Executing...' : 'Execute'}
          </button>

          <a
            href="http://localhost:8080"
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-1 px-3 py-1.5 text-sm text-slate-400 hover:text-white transition-colors"
          >
            <ExternalLink className="w-4 h-4" />
            Console
          </a>
        </div>
      </header>

      {/* Status bar */}
      {(parseResult || executeResult) && (
        <div className={`flex-none px-4 py-2 text-sm flex items-center gap-2 ${
          executeResult
            ? (executeResult.success ? 'bg-green-900/50 text-green-300' : 'bg-red-900/50 text-red-300')
            : (parseResult?.valid ? 'bg-blue-900/50 text-blue-300' : 'bg-yellow-900/50 text-yellow-300')
        }`}>
          {executeResult ? (
            <>
              {executeResult.success ? <Check className="w-4 h-4" /> : <AlertCircle className="w-4 h-4" />}
              {executeResult.message}
              {executeResult.ids.length > 0 && (
                <span className="text-xs opacity-75">
                  (IDs: {executeResult.ids.slice(0, 3).join(', ')}{executeResult.ids.length > 3 && '...'})
                </span>
              )}
            </>
          ) : parseResult?.valid ? (
            <>
              <Check className="w-4 h-4" />
              Schema valid - {parseResult.schema_data?.node_types?.length || 0} node types, {parseResult.schema_data?.edge_types?.length || 0} edge types
            </>
          ) : (
            <>
              <AlertCircle className="w-4 h-4" />
              {parseResult?.errors?.[0] || 'Schema has errors'}
            </>
          )}
        </div>
      )}

      {/* Main content - split view */}
      <div className="flex-1 flex min-h-0">
        {/* Left panel - YAML Editor */}
        <div className="w-1/2 flex flex-col border-r border-slate-700">
          <div className="flex-none h-10 bg-slate-800 border-b border-slate-700 flex items-center px-4">
            <span className="text-sm text-slate-400">Schema (YAML)</span>
            <div className="ml-auto flex items-center gap-2">
              {parseMutation.isPending && (
                <span className="text-xs text-slate-500">Parsing...</span>
              )}
            </div>
          </div>
          <div className="flex-1 min-h-0">
            <YamlEditor value={yaml} onChange={setYaml} errors={parseResult?.errors} />
          </div>
        </div>

        {/* Right panel - Code Preview */}
        <div className="w-1/2 flex flex-col">
          <CodePreview parseResult={parseResult} />
        </div>
      </div>

      {/* AI Modal */}
      {showAIModal && (
        <AIPromptModal
          onClose={() => setShowAIModal(false)}
          onGenerate={handleSchemaGenerated}
        />
      )}
    </div>
  )
}

export default App
