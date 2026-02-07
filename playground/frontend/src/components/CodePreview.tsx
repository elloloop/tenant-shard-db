import { useState } from 'react'
import { Copy, Check } from 'lucide-react'
import { SchemaParseResponse } from '../api'

interface CodePreviewProps {
  parseResult: SchemaParseResponse | null
}

type TabType = 'python_schema' | 'python_usage' | 'python_data' | 'go_schema' | 'go_usage'

const TABS: { id: TabType; label: string; lang: 'python' | 'go' }[] = [
  { id: 'python_schema', label: 'Python Schema', lang: 'python' },
  { id: 'python_usage', label: 'Python Usage', lang: 'python' },
  { id: 'python_data', label: 'Python Data', lang: 'python' },
  { id: 'go_schema', label: 'Go Schema', lang: 'go' },
  { id: 'go_usage', label: 'Go Usage', lang: 'go' },
]

export default function CodePreview({ parseResult }: CodePreviewProps) {
  const [activeTab, setActiveTab] = useState<TabType>('python_schema')
  const [copied, setCopied] = useState(false)

  const getCode = (): string => {
    if (!parseResult?.valid) return '# Parse your schema to see generated code'

    switch (activeTab) {
      case 'python_schema':
        return parseResult.python_schema || '# No schema code generated'
      case 'python_usage':
        return parseResult.python_usage || '# No usage code generated'
      case 'python_data':
        return parseResult.python_data || '# No data operations in schema'
      case 'go_schema':
        return parseResult.go_schema || '// No schema code generated'
      case 'go_usage':
        return parseResult.go_usage || '// No usage code generated'
      default:
        return ''
    }
  }

  const handleCopy = async () => {
    const code = getCode()
    await navigator.clipboard.writeText(code)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const code = getCode()
  const lineCount = code.split('\n').length

  return (
    <div className="h-full flex flex-col">
      {/* Tabs */}
      <div className="flex-none h-10 bg-slate-800 border-b border-slate-700 flex items-center px-2">
        <div className="flex gap-1">
          {TABS.map((tab) => {
            // Hide python_data tab if no data
            if (tab.id === 'python_data' && !parseResult?.python_data) {
              return null
            }

            return (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id)}
                className={`px-3 py-1 text-xs rounded transition-colors ${
                  activeTab === tab.id
                    ? 'bg-slate-700 text-white'
                    : 'text-slate-400 hover:text-white hover:bg-slate-700/50'
                }`}
              >
                {tab.label}
              </button>
            )
          })}
        </div>

        <button
          onClick={handleCopy}
          className="ml-auto flex items-center gap-1 px-2 py-1 text-xs text-slate-400 hover:text-white transition-colors"
        >
          {copied ? (
            <>
              <Check className="w-3 h-3 text-green-400" />
              <span className="text-green-400">Copied!</span>
            </>
          ) : (
            <>
              <Copy className="w-3 h-3" />
              Copy
            </>
          )}
        </button>
      </div>

      {/* Code display */}
      <div className="flex-1 min-h-0 overflow-auto bg-slate-950">
        <div className="flex min-h-full">
          {/* Line numbers */}
          <div className="flex-none w-12 bg-slate-900 border-r border-slate-800 text-right py-3 pr-2 select-none">
            {Array.from({ length: lineCount }, (_, i) => (
              <div key={i} className="text-xs text-slate-600 leading-6 font-mono">
                {i + 1}
              </div>
            ))}
          </div>

          {/* Code */}
          <pre className="flex-1 p-3 text-sm leading-6 font-mono text-slate-300 whitespace-pre overflow-x-auto">
            {code}
          </pre>
        </div>
      </div>
    </div>
  )
}
