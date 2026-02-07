import { useRef, useCallback } from 'react'

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

      // Insert 2 spaces for tab
      const newValue = value.substring(0, start) + '  ' + value.substring(end)
      onChange(newValue)

      // Move cursor after the inserted spaces
      requestAnimationFrame(() => {
        textarea.selectionStart = textarea.selectionEnd = start + 2
      })
    }
  }, [value, onChange])

  const lineCount = value.split('\n').length

  return (
    <div className="h-full flex bg-slate-950">
      {/* Line numbers */}
      <div className="flex-none w-12 bg-slate-900 border-r border-slate-800 text-right py-3 pr-2 select-none overflow-hidden">
        {Array.from({ length: lineCount }, (_, i) => (
          <div key={i} className="text-xs text-slate-600 leading-6 font-mono">
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
          className="absolute inset-0 w-full h-full resize-none bg-transparent text-slate-100
                     editor-textarea p-3 leading-6 focus:outline-none"
          style={{ caretColor: '#3b82f6' }}
        />
      </div>

      {/* Error panel */}
      {errors && errors.length > 0 && (
        <div className="absolute bottom-0 left-0 right-0 bg-red-900/90 border-t border-red-700 p-3 max-h-32 overflow-auto">
          <div className="text-sm text-red-200 font-medium mb-1">Errors:</div>
          {errors.map((err, i) => (
            <div key={i} className="text-xs text-red-300 font-mono">{err}</div>
          ))}
        </div>
      )}
    </div>
  )
}
