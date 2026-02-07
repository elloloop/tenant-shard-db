import { useRef, useCallback } from 'react'
import { AlertCircle } from 'lucide-react'
import { cn } from '@/lib/utils'

interface YamlEditorProps {
  value: string
  onChange: (value: string) => void
  errors?: string[]
}

export default function YamlEditor({ value, onChange, errors }: YamlEditorProps) {
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  const handleKeyDown = useCallback((e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Tab') {
      e.preventDefault()
      const textarea = textareaRef.current
      if (!textarea) return

      const start = textarea.selectionStart
      const end = textarea.selectionEnd

      const newValue = value.substring(0, start) + '  ' + value.substring(end)
      onChange(newValue)

      requestAnimationFrame(() => {
        textarea.selectionStart = textarea.selectionEnd = start + 2
      })
    }
  }, [value, onChange])

  const lineCount = value.split('\n').length

  return (
    <div className="h-full flex relative">
      {/* Line numbers */}
      <div className="flex-none w-12 bg-muted/50 border-r text-right py-3 pr-2 select-none overflow-hidden">
        {Array.from({ length: lineCount }, (_, i) => (
          <div key={i} className="text-xs text-muted-foreground leading-6 font-mono">
            {i + 1}
          </div>
        ))}
      </div>

      {/* Editor */}
      <div className="flex-1 relative">
        <textarea
          ref={textareaRef}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          onKeyDown={handleKeyDown}
          spellCheck={false}
          className={cn(
            "absolute inset-0 w-full h-full resize-none bg-background text-foreground",
            "editor-textarea p-3 leading-6 focus:outline-none focus:ring-0 border-0",
            "placeholder:text-muted-foreground"
          )}
          style={{ caretColor: 'hsl(var(--primary))' }}
        />
      </div>

      {/* Error panel */}
      {errors && errors.length > 0 && (
        <div className="absolute bottom-0 left-0 right-0 bg-destructive/10 border-t border-destructive/30 p-3 max-h-32 overflow-auto">
          <div className="flex items-center gap-2 text-sm text-destructive font-medium mb-1">
            <AlertCircle className="w-4 h-4" />
            Errors
          </div>
          {errors.map((err, i) => (
            <div key={i} className="text-xs text-destructive/80 font-mono pl-6">{err}</div>
          ))}
        </div>
      )}
    </div>
  )
}
