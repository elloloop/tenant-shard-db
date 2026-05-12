import { useEffect, useState } from 'react'
import { getApiKey, setApiKey } from '../api'
import {
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogOverlay,
  DialogTitle,
} from './ui'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
}

// Minimal settings dialog. PR 1 kept the API-key input client-side
// (localStorage); a future PR replaces this with server-side stamping.
// The hand-rolled <dialog>/<div> overlay this file used to ship has
// been retired in favour of the refraction Dialog wrapped under
// `@/components/ui` (see #496) so the console shares a design-system
// seam with the rest of the workspace.
export function SettingsDialog({ open, onOpenChange }: Props) {
  const [draft, setDraft] = useState('')

  useEffect(() => {
    if (open) setDraft(getApiKey())
  }, [open])

  const onSave = () => {
    setApiKey(draft.trim())
    onOpenChange(false)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogOverlay />
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Settings</DialogTitle>
          <DialogDescription>
            The API key is sent to the server as{' '}
            <code>authorization: Bearer …</code> on every Console request.
            Stored in localStorage on this browser only.
          </DialogDescription>
        </DialogHeader>

        <div>
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
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={onSave}>Save</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
