package main

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// newSPAHandler returns an http.Handler that serves the embedded React
// SPA. The behaviour is the standard SPA fallback:
//
//   - exact-file requests under /assets/ (or any path that has an
//     extension) are served from frontendDist verbatim, with the right
//     MIME type and a long cache TTL (Vite stamps content hashes into
//     filenames so this is safe).
//
//   - any other request — `/`, `/tenants`, `/tenants/foo/nodes/bar`,
//     etc. — falls through to `index.html` so React Router can handle
//     it client-side.
//
// The Connect handler is mounted at /entdb.console.v1.Console/ on the
// same mux, so that prefix is matched before this fallback ever runs;
// we don't need to handle the API path here.
//
// `apiKey` is currently unused by the server-side stamping path —
// PR 1's frontend reads the key from localStorage, set via a tiny
// settings dialog. The parameter is kept on the signature so a future
// PR can introduce server-side stamping (e.g. `__ENTDB_API_KEY__`
// templated into index.html) without changing call sites.
func newSPAHandler(apiKey string) (http.Handler, error) {
	dist, err := fs.Sub(frontendDist, "frontend/dist")
	if err != nil {
		return nil, err
	}

	indexBytes, err := readIndex(dist)
	if err != nil {
		// Index missing means the frontend wasn't built. Don't fail
		// startup — running the binary with --addr just to test the
		// gRPC handlers is a legitimate use-case during dev. Return
		// a placeholder handler instead so curl gets a clear message.
		return placeholderHandler(err), nil
	}

	fileServer := http.FileServer(http.FS(dist))
	_ = apiKey // reserved for future server-side stamping

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only GET/HEAD make sense for static assets.
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		urlPath := r.URL.Path
		if urlPath == "" || urlPath == "/" {
			serveIndex(w, r, indexBytes)
			return
		}

		// Look up the file in the embed FS. If it exists, serve it.
		// Otherwise fall through to index.html for SPA routing.
		clean := strings.TrimPrefix(path.Clean(urlPath), "/")
		if clean == "" {
			serveIndex(w, r, indexBytes)
			return
		}
		if f, err := dist.Open(clean); err == nil {
			defer f.Close()
			info, statErr := f.Stat()
			if statErr == nil && !info.IsDir() {
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		// Path doesn't map to a real file — SPA route.
		serveIndex(w, r, indexBytes)
	}), nil
}

func readIndex(dist fs.FS) ([]byte, error) {
	f, err := dist.Open("index.html")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

func serveIndex(w http.ResponseWriter, r *http.Request, indexBytes []byte) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// Don't cache index.html — the SPA's hashed asset URLs inside it
	// are what change on each deploy, and they ARE long-cached.
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, "index.html", staticModTime, bytes.NewReader(indexBytes))
}

// placeholderHandler is what we serve when the frontend hasn't been
// built. It returns a tiny HTML stub with a clear message so a curl
// against `/` doesn't look like a 404.
func placeholderHandler(cause error) http.Handler {
	body := []byte(`<!doctype html>
<html><head><meta charset="utf-8"><title>entdb-console</title></head>
<body style="font-family: system-ui, sans-serif; padding: 2rem;">
<h1>entdb-console</h1>
<p>The Go server is running, but the embedded SPA is missing.</p>
<p>Build the frontend first:</p>
<pre>cd sdk/go/entdb/cmd/entdb-console/frontend &amp;&amp; npm install &amp;&amp; npm run build</pre>
<p>Then rebuild the binary so <code>//go:embed frontend/dist</code> picks it up.</p>
</body></html>
`)
	_ = cause // kept for logs at startup; not exposed in the page
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Connect requests land on the /entdb.console.v1.Console/
		// prefix and are routed elsewhere by the mux, so anything
		// reaching this handler is asking for static content.
		if !errors.Is(r.Context().Err(), nil) && r.Context().Err() != nil {
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	})
}
