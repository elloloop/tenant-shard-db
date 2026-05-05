// Package main — entdb-console: single-binary Go server that ships the
// React SPA via go:embed and exposes a curated, read-only ConnectRPC
// surface that pass-through-proxies to the upstream entdb-server.
package main

import "embed"

// frontendDist is the built React SPA. The build pipeline builds the
// frontend (`npm run build` → `frontend/dist/`) BEFORE compiling this
// Go binary; the embed directive captures the contents at compile time.
//
// `all:` so dotfiles (e.g. `.vite`) are included if Vite ever drops one.
// The fallback `frontend/dist/_placeholder` exists in source so `go vet`
// and `go test` work even when the frontend hasn't been built — without
// a fallback file, `go:embed` errors out at compile time.
//
//go:embed all:frontend/dist
var frontendDist embed.FS
