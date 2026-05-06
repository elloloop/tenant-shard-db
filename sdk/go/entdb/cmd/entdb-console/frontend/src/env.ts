// Runtime config injected by the Go server into index.html via
// `<script>window.__ENTDB_* = ...;</script>`. See spa.go:stampIndex.
//
// These values are server-configured (not user-editable) so they
// arrive on the very first render — no RPC required, which means the
// Sandbox tab decision (show / hide) doesn't flicker between mounts.
//
// Values are always strings; empty string means "not set."

declare global {
  interface Window {
    __ENTDB_SANDBOX_TENANT__?: string
    __ENTDB_SANDBOX_DEFAULT_ACTOR__?: string
    __ENTDB_API_KEY__?: string
  }
}

export function sandboxTenant(): string {
  if (typeof window === 'undefined') return ''
  return window.__ENTDB_SANDBOX_TENANT__ ?? ''
}

export function sandboxDefaultActor(): string {
  if (typeof window === 'undefined') return ''
  return window.__ENTDB_SANDBOX_DEFAULT_ACTOR__ ?? ''
}

export function sandboxEnabled(): boolean {
  return sandboxTenant() !== ''
}
