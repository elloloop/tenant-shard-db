import { useState } from 'react'
import { Copy, Check } from 'lucide-react'
import { SchemaParseResponse } from '../api'
import { Button } from '@/components/ui/button'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import { cn } from '@/lib/utils'

interface CodePreviewProps {
  parseResult: SchemaParseResponse | null
}

type TabType = 'python_schema' | 'python_usage' | 'python_data' | 'go_schema' | 'go_usage'

const TABS: { id: TabType; label: string }[] = [
  { id: 'python_schema', label: 'Python Schema' },
  { id: 'python_usage', label: 'Python Usage' },
  { id: 'python_data', label: 'Data Operations' },
  { id: 'go_schema', label: 'Go Schema' },
  { id: 'go_usage', label: 'Go Usage' },
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

  const availableTabs = TABS.filter(tab => {
    if (tab.id === 'python_data') {
      return parseResult?.python_data
    }
    return true
  })

  return (
    <div className="h-full flex flex-col">
      {/* Tabs header */}
      <div className="flex-none h-10 bg-muted/30 border-b flex items-center justify-between px-2">
        <Tabs value={activeTab} onValueChange={(v) => setActiveTab(v as TabType)} className="h-full">
          <TabsList className="h-8 bg-transparent p-0 gap-1">
            {availableTabs.map((tab) => (
              <TabsTrigger
                key={tab.id}
                value={tab.id}
                className={cn(
                  "h-7 px-3 text-xs rounded-md data-[state=active]:bg-background",
                  "data-[state=active]:shadow-sm"
                )}
              >
                {tab.label}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>

        <Button
          variant="ghost"
          size="sm"
          onClick={handleCopy}
          className="h-7 px-2 text-xs gap-1"
        >
          {copied ? (
            <>
              <Check className="w-3 h-3 text-green-500" />
              Copied
            </>
          ) : (
            <>
              <Copy className="w-3 h-3" />
              Copy
            </>
          )}
        </Button>
      </div>

      {/* Code display */}
      <div className="flex-1 min-h-0 overflow-auto bg-muted/20">
        <div className="flex min-h-full">
          {/* Line numbers */}
          <div className="flex-none w-12 bg-muted/50 border-r text-right py-3 pr-2 select-none">
            {Array.from({ length: lineCount }, (_, i) => (
              <div key={i} className="text-xs text-muted-foreground leading-6 font-mono">
                {i + 1}
              </div>
            ))}
          </div>

          {/* Code */}
          <pre className="flex-1 p-3 text-sm leading-6 font-mono text-foreground whitespace-pre overflow-x-auto">
            {code}
          </pre>
        </div>
      </div>
    </div>
  )
}
