import { useEffect, useState } from 'react'
import { getApiKey, setApiKey } from '../api'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
}

// Minimal settings dialog. PR 1 keeps the API-key input client-side
// (localStorage); a future PR replaces this with server-side stamping.
// We deliberately use a plain HTML `<dialog>` instead of pulling in
// radix here — the goal is to keep the primitive footprint small
// in PR 1 and let later PRs grow the design system.
export function SettingsDialog({ open, onOpenChange }: Props) {
  const [draft, setDraft] = useState('')

  useEffect(() => {
    if (open) setDraft(getApiKey())
  }, [open])

  if (!open) return null

  const onSave = () => {
    setApiKey(draft.trim())
    onOpenChange(false)
  }

  return (
    <div
      role="dialog"
      aria-modal="true"
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
      onClick={() => onOpenChange(false)}
    >
      <div
        className="bg-card border rounded-lg shadow-lg p-6 w-full max-w-md"
        onClick={(e) => e.stopPropagation()}
      >
        <h2 className="text-lg font-semibold mb-2">Settings</h2>
        <p className="text-sm text-muted-foreground mb-4">
          The API key is sent to the server as <code>authorization: Bearer …</code>{' '}
          on every Console request. Stored in localStorage on this browser only.
        </p>
        <label className="block text-sm font-medium mb-1" htmlFor="api-key-input">
          API key
        </label>
        <input
          id="api-key-input"
          type="password"
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          className="w-full rounded-md border bg-background px-3 py-2 text-sm font-mono"
          placeholder="Bearer token"
          autoFocus
        />
        <div className="mt-6 flex justify-end gap-2">
          <button
            onClick={() => onOpenChange(false)}
            className="rounded-md border px-3 py-2 text-sm hover:bg-accent"
          >
            Cancel
          </button>
          <button
            onClick={onSave}
            className="rounded-md bg-primary px-3 py-2 text-sm text-primary-foreground hover:opacity-90"
          >
            Save
          </button>
        </div>
      </div>
    </div>
  )
}
