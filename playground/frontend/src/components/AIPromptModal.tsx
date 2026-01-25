import { useState, useEffect } from 'react'
import { Sparkles, Copy, Check, Loader2, ArrowLeft, ArrowRight } from 'lucide-react'
import { getAIPromptTemplate, generateAIPrompt } from '../api'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

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
    <Dialog open onOpenChange={() => onClose()}>
      <DialogContent className="sm:max-w-2xl max-h-[85vh] flex flex-col">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Sparkles className="w-5 h-5 text-primary" />
            AI Schema Generator
          </DialogTitle>
          <DialogDescription>
            Generate EntDB schemas from natural language using any AI assistant
          </DialogDescription>
        </DialogHeader>

        <div className="flex-1 min-h-0 overflow-auto py-4">
          {step === 'requirements' && (
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium mb-2">
                  Describe your data model
                </label>
                <Textarea
                  value={requirements}
                  onChange={(e) => setRequirements(e.target.value)}
                  placeholder="e.g., Create a blog system with users, posts, and comments..."
                  className="h-40 resize-none font-mono text-sm"
                />
              </div>

              {exampleReqs && (
                <div className="text-sm">
                  <span className="text-muted-foreground">Example: </span>
                  <button
                    onClick={() => setRequirements(exampleReqs)}
                    className="text-primary hover:underline"
                  >
                    Use example
                  </button>
                </div>
              )}

              <div className="bg-muted/50 rounded-lg p-4 text-sm">
                <p className="font-medium mb-2">How it works:</p>
                <ol className="list-decimal list-inside space-y-1 text-muted-foreground">
                  <li>Describe your data model in plain English</li>
                  <li>We generate a prompt for ChatGPT/Claude</li>
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
                  <label className="text-sm font-medium">
                    Copy this prompt to ChatGPT or Claude:
                  </label>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={handleCopyPrompt}
                    className="h-7 text-xs gap-1"
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
                <pre className="w-full h-64 px-3 py-2 bg-muted rounded-md text-xs font-mono overflow-auto whitespace-pre-wrap text-muted-foreground">
                  {prompt}
                </pre>
              </div>
            </div>
          )}

          {step === 'paste' && (
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium mb-2">
                  Paste the generated YAML from the AI:
                </label>
                <Textarea
                  value={pastedSchema}
                  onChange={(e) => setPastedSchema(e.target.value)}
                  placeholder={`version: 1
tenant: playground

node_types:
  - type_id: 1
    name: User
    ...`}
                  className="h-64 resize-none font-mono text-sm"
                />
              </div>
            </div>
          )}
        </div>

        {/* Footer */}
        <div className="flex-none pt-4 border-t flex gap-2">
          {step === 'requirements' && (
            <Button
              onClick={handleGeneratePrompt}
              disabled={!requirements.trim() || loading}
              className="flex-1 gap-2"
            >
              {loading ? (
                <Loader2 className="w-4 h-4 animate-spin" />
              ) : (
                <ArrowRight className="w-4 h-4" />
              )}
              {loading ? 'Generating...' : 'Generate Prompt'}
            </Button>
          )}

          {step === 'prompt' && (
            <>
              <Button
                variant="outline"
                onClick={() => setStep('requirements')}
                className="gap-2"
              >
                <ArrowLeft className="w-4 h-4" />
                Back
              </Button>
              <Button
                onClick={() => setStep('paste')}
                className="flex-1 gap-2"
              >
                <ArrowRight className="w-4 h-4" />
                I copied the prompt
              </Button>
            </>
          )}

          {step === 'paste' && (
            <>
              <Button
                variant="outline"
                onClick={() => setStep('prompt')}
                className="gap-2"
              >
                <ArrowLeft className="w-4 h-4" />
                Back
              </Button>
              <Button
                onClick={handleUseSchema}
                disabled={!pastedSchema.trim()}
                className="flex-1 gap-2"
              >
                <Check className="w-4 h-4" />
                Use this schema
              </Button>
            </>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}
