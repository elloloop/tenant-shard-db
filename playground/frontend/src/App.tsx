import { useState, useCallback, useEffect } from 'react'
import { useMutation } from '@tanstack/react-query'
import { Play, Sparkles, Copy, Check, AlertCircle, ExternalLink, Code2, Loader2, Sun, Moon, Monitor } from 'lucide-react'
import { parseSchema, executeSchema, getAIPromptTemplate, SchemaParseResponse } from './api'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import { useTheme } from '@/components/theme-provider'
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
        {theme === 'light' && 'Light mode'}
        {theme === 'dark' && 'Dark mode'}
        {theme === 'system' && 'System mode'}
      </TooltipContent>
    </Tooltip>
  )
}

function App() {
  const [yaml, setYaml] = useState(DEFAULT_SCHEMA)
  const [parseResult, setParseResult] = useState<SchemaParseResponse | null>(null)
  const [showAIModal, setShowAIModal] = useState(false)
  const [copied, setCopied] = useState(false)
  const [executeResult, setExecuteResult] = useState<{ success: boolean; message: string; ids: string[] } | null>(null)

  const parseMutation = useMutation({
    mutationFn: (content: string) => parseSchema(content, 'yaml'),
    onSuccess: (data) => {
      setParseResult(data)
      setExecuteResult(null)
    },
  })

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
    <TooltipProvider>
      <div className="h-screen flex flex-col bg-background text-foreground">
        {/* Header */}
        <header className="flex-none h-14 border-b flex items-center justify-between px-4">
          <div className="flex items-center gap-3">
            <Code2 className="w-6 h-6 text-primary" />
            <h1 className="text-lg font-semibold">EntDB Playground</h1>
            <Badge variant="secondary" className="text-xs font-normal">
              Interactive SDK Simulator
            </Badge>
          </div>

          <div className="flex items-center gap-2">
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setShowAIModal(true)}
                  className="gap-2"
                >
                  <Sparkles className="w-4 h-4" />
                  AI Generate
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                Generate schema from natural language
              </TooltipContent>
            </Tooltip>

            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={handleCopyPrompt}
                  className="gap-2"
                >
                  {copied ? <Check className="w-4 h-4 text-green-500" /> : <Copy className="w-4 h-4" />}
                  {copied ? 'Copied' : 'Copy Prompt'}
                </Button>
              </TooltipTrigger>
              <TooltipContent>
                Copy AI prompt template to clipboard
              </TooltipContent>
            </Tooltip>

            <Button
              size="sm"
              onClick={handleExecute}
              disabled={!parseResult?.valid || executeMutation.isPending}
              className="gap-2"
            >
              {executeMutation.isPending ? (
                <Loader2 className="w-4 h-4 animate-spin" />
              ) : (
                <Play className="w-4 h-4" />
              )}
              {executeMutation.isPending ? 'Executing...' : 'Execute'}
            </Button>

            <div className="w-px h-6 bg-border mx-1" />

            <ThemeToggle />

            <Button variant="ghost" size="sm" asChild className="gap-1 text-muted-foreground">
              <a href="http://localhost:8080" target="_blank" rel="noopener noreferrer">
                <ExternalLink className="w-4 h-4" />
                Console
              </a>
            </Button>
          </div>
        </header>

        {/* Status bar */}
        {(parseResult || executeResult) && (
          <div className={`flex-none px-4 py-2 text-sm flex items-center gap-2 border-b ${
            executeResult
              ? (executeResult.success ? 'bg-green-500/10 text-green-600 dark:text-green-400' : 'bg-destructive/10 text-destructive')
              : (parseResult?.valid ? 'bg-primary/10 text-primary' : 'bg-yellow-500/10 text-yellow-600 dark:text-yellow-400')
          }`}>
            {executeResult ? (
              <>
                {executeResult.success ? <Check className="w-4 h-4" /> : <AlertCircle className="w-4 h-4" />}
                <span>{executeResult.message}</span>
                {executeResult.ids.length > 0 && (
                  <Badge variant="outline" className="ml-2 text-xs">
                    {executeResult.ids.length} node{executeResult.ids.length !== 1 ? 's' : ''} created
                  </Badge>
                )}
              </>
            ) : parseResult?.valid ? (
              <>
                <Check className="w-4 h-4" />
                <span>Schema valid</span>
                <Badge variant="secondary" className="ml-2 text-xs">
                  {(parseResult.schema_data?.node_types as unknown[])?.length || 0} node types
                </Badge>
                <Badge variant="secondary" className="text-xs">
                  {(parseResult.schema_data?.edge_types as unknown[])?.length || 0} edge types
                </Badge>
              </>
            ) : (
              <>
                <AlertCircle className="w-4 h-4" />
                <span>{parseResult?.errors?.[0] || 'Schema has errors'}</span>
              </>
            )}
          </div>
        )}

        {/* Main content - split view */}
        <div className="flex-1 flex min-h-0">
          {/* Left panel - YAML Editor */}
          <div className="w-1/2 flex flex-col border-r">
            <div className="flex-none h-10 bg-muted/30 border-b flex items-center justify-between px-4">
              <span className="text-sm text-muted-foreground font-medium">Schema (YAML)</span>
              {parseMutation.isPending && (
                <div className="flex items-center gap-2 text-xs text-muted-foreground">
                  <Loader2 className="w-3 h-3 animate-spin" />
                  Parsing...
                </div>
              )}
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
    </TooltipProvider>
  )
}

export default App
