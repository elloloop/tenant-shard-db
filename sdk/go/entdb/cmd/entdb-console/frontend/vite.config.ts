import path from "path"
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// Vite config for the entdb-console SPA.
//
// During `npm run dev` we proxy the ConnectRPC path prefix to the Go
// binary running on :8080 so the same-origin Connect client works
// without CORS gymnastics. In production the Go binary serves both
// the SPA and the Connect endpoint on the same port, so the SPA's
// `baseUrl: ''` (same-origin) wires up automatically.

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/entdb.console.v1.Console': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    // Keep `dist/_placeholder` (and the `dist/.gitignore` exception)
    // around between builds so a fresh checkout — where `dist/` only
    // contains the placeholder — can still satisfy the Go binary's
    // `//go:embed all:frontend/dist` directive even before
    // `npm run build` runs.
    emptyOutDir: false,
    sourcemap: true,
  },
})
