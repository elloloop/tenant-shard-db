// Connect-Web client for the EntDB Console.
//
// The Go server hosts this SPA AND the Connect endpoint at
// `/entdb.console.v1.Console/...` on the same port, so `baseUrl: ''`
// (same-origin) is exactly right for production. In `npm run dev` the
// Vite proxy in `vite.config.ts` forwards that prefix to the Go server
// on :8080.
//
// Auth model (PR 1): the API key lives in `localStorage` under the key
// `entdb_api_key`. A request interceptor stamps it on every Connect
// call as `authorization: Bearer …`. A future PR will move this to a
// server-side stamp into `index.html` so the SPA never sees the raw
// key.
import { createPromiseClient, Interceptor } from '@connectrpc/connect'
import { createConnectTransport } from '@connectrpc/connect-web'
import { Console } from './gen/console_connect'

const API_KEY_STORAGE_KEY = 'entdb_api_key'

export function getApiKey(): string {
  try {
    return localStorage.getItem(API_KEY_STORAGE_KEY) ?? ''
  } catch {
    return ''
  }
}

export function setApiKey(key: string): void {
  try {
    if (key === '') localStorage.removeItem(API_KEY_STORAGE_KEY)
    else localStorage.setItem(API_KEY_STORAGE_KEY, key)
  } catch {
    /* ignore quota / privacy-mode errors */
  }
}

const authInterceptor: Interceptor = (next) => async (req) => {
  const apiKey = getApiKey()
  if (apiKey) req.header.set('authorization', `Bearer ${apiKey}`)
  return next(req)
}

const transport = createConnectTransport({
  baseUrl: '',
  interceptors: [authInterceptor],
})

export const consoleClient = createPromiseClient(Console, transport)
