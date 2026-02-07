import { useState, useEffect } from 'react'
import { X, Wand2, Copy, Check, Loader2 } from 'lucide-react'
import { getAIPromptTemplate, generateAIPrompt } from '../api'

interface AIPromptModalProps {
  onClose: () => void
  onGenerate: (schema: string) => void
}

export default function AIPromptModal({ onClose, onGenerate }: AIPromptModalProps) {
  const [step, setStep] = useState<'requirements' | 'prompt' | 'paste'>('requirements')
  const [requirements, setRequirements] = useState('')
  const [prompt, setPrompt] = useState('')
  const [pastedSchema, setPastedSchema] = useState('')
  const [loading, setLoading] = useState(false)
  const [copied, setCopied] = useState(false)
  const [exampleReqs, setExampleReqs] = useState('')

  // Load example on mount
  useEffect(() => {
    getAIPromptTemplate().then((data) => {
      setExampleReqs(data.example_requirements)
    }).catch(console.error)
  }, [])

  const handleGeneratePrompt = async () => {
    if (!requirements.trim()) return
    setLoading(true)
    try {
      const data = await generateAIPrompt(requirements)
      setPrompt(data.prompt)
      setStep('prompt')
    } catch (e) {
      console.error('Failed to generate prompt:', e)
    } finally {
      setLoading(false)
    }
  }

  const handleCopyPrompt = async () => {
    await navigator.clipboard.writeText(prompt)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const handleUseSchema = () => {
    if (pastedSchema.trim()) {
      onGenerate(pastedSchema)
    }
  }

  return (
    <div className="fixed inset-0 bg-black/70 flex items-center justify-center z-50 p-4">
      <div className="bg-slate-800 rounded-lg w-full max-w-2xl max-h-[80vh] flex flex-col shadow-2xl">
        {/* Header */}
        <div className="flex-none flex items-center justify-between p-4 border-b border-slate-700">
          <div className="flex items-center gap-2">
            <Wand2 className="w-5 h-5 text-purple-400" />
            <h2 className="text-lg font-semibold">AI Schema Generator</h2>
          </div>
          <button
            onClick={onClose}
            className="p-1 text-slate-400 hover:text-white transition-colors"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Content */}
        <div className="flex-1 min-h-0 overflow-auto p-4">
          {step === 'requirements' && (
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-2">
                  Describe your data model
                </label>
                <textarea
                  value={requirements}
                  onChange={(e) => setRequirements(e.target.value)}
                  placeholder="e.g., Create a blog system with users, posts, and comments..."
                  className="w-full h-40 px-3 py-2 bg-slate-900 border border-slate-700 rounded-md
                           text-slate-100 placeholder:text-slate-500 focus:outline-none focus:ring-2
                           focus:ring-purple-500 resize-none"
                />
              </div>

              {exampleReqs && (
                <div className="text-sm">
                  <span className="text-slate-400">Example: </span>
                  <button
                    onClick={() => setRequirements(exampleReqs)}
                    className="text-purple-400 hover:text-purple-300 transition-colors"
                  >
                    Use example
                  </button>
                </div>
              )}

              <div className="bg-slate-900/50 rounded-lg p-3 text-sm text-slate-400">
                <p className="font-medium text-slate-300 mb-1">How it works:</p>
                <ol className="list-decimal list-inside space-y-1">
                  <li>Describe your data model in plain English</li>
                  <li>We'll generate a prompt for ChatGPT/Claude</li>
                  <li>Copy the prompt and paste it into your AI assistant</li>
                  <li>Paste the generated YAML back here</li>
                </ol>
              </div>
            </div>
          )}

          {step === 'prompt' && (
            <div className="space-y-4">
              <div>
                <div className="flex items-center justify-between mb-2">
                  <label className="text-sm font-medium text-slate-300">
                    Copy this prompt to ChatGPT or Claude:
                  </label>
                  <button
                    onClick={handleCopyPrompt}
                    className="flex items-center gap-1 px-2 py-1 text-xs bg-slate-700 hover:bg-slate-600
                             rounded transition-colors"
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
                <pre className="w-full h-64 px-3 py-2 bg-slate-900 border border-slate-700 rounded-md
                              text-slate-300 text-xs font-mono overflow-auto whitespace-pre-wrap">
                  {prompt}
                </pre>
              </div>

              <button
                onClick={() => setStep('paste')}
                className="w-full py-2 bg-purple-600 hover:bg-purple-500 rounded-md text-sm font-medium
                         transition-colors"
              >
                I've copied the prompt, let me paste the result
              </button>
            </div>
          )}

          {step === 'paste' && (
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-slate-300 mb-2">
                  Paste the generated YAML from the AI:
                </label>
                <textarea
                  value={pastedSchema}
                  onChange={(e) => setPastedSchema(e.target.value)}
                  placeholder="version: 1
tenant: playground

node_types:
  - type_id: 1
    name: User
    ..."
                  className="w-full h-64 px-3 py-2 bg-slate-900 border border-slate-700 rounded-md
                           text-slate-100 placeholder:text-slate-500 focus:outline-none focus:ring-2
                           focus:ring-purple-500 font-mono text-sm resize-none"
                />
              </div>

              <div className="flex gap-2">
                <button
                  onClick={() => setStep('prompt')}
                  className="px-4 py-2 bg-slate-700 hover:bg-slate-600 rounded-md text-sm transition-colors"
                >
                  Back to prompt
                </button>
                <button
                  onClick={handleUseSchema}
                  disabled={!pastedSchema.trim()}
                  className="flex-1 py-2 bg-green-600 hover:bg-green-500 disabled:bg-slate-700
                           disabled:text-slate-500 rounded-md text-sm font-medium transition-colors"
                >
                  Use this schema
                </button>
              </div>
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="flex-none p-4 border-t border-slate-700">
          {step === 'requirements' && (
            <button
              onClick={handleGeneratePrompt}
              disabled={!requirements.trim() || loading}
              className="w-full py-2 bg-purple-600 hover:bg-purple-500 disabled:bg-slate-700
                       disabled:text-slate-500 rounded-md text-sm font-medium transition-colors
                       flex items-center justify-center gap-2"
            >
              {loading ? (
                <>
                  <Loader2 className="w-4 h-4 animate-spin" />
                  Generating...
                </>
              ) : (
                <>
                  <Wand2 className="w-4 h-4" />
                  Generate Prompt
                </>
              )}
            </button>
          )}

          {step !== 'requirements' && (
            <button
              onClick={() => setStep('requirements')}
              className="w-full py-2 text-sm text-slate-400 hover:text-white transition-colors"
            >
              Start over with new requirements
            </button>
          )}
        </div>
      </div>
    </div>
  )
}
