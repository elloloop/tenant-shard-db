package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/elloloop/tenant-shard-db/sdk/go/entdb/internal/console/v1/consolev1connect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// staticModTime is the mtime reported for embedded static assets. We
// use process start time so `If-Modified-Since` works within a single
// run; cross-deploy freshness is handled by Vite's hashed filenames
// for everything except index.html (which we explicitly mark
// no-cache in spa.go).
var staticModTime = time.Now()

func main() {
	addr := flag.String("addr", envOr("ENTDB_CONSOLE_ADDR", ":8080"), "HTTP listen address")
	upstream := flag.String("upstream", envOr("ENTDB_UPSTREAM", "localhost:50051"), "upstream entdb-server gRPC address (host:port)")
	apiKey := flag.String("api-key", os.Getenv("ENTDB_API_KEY"), "API key forwarded to upstream as `authorization: Bearer …`")
	shutdownTimeout := flag.Duration("shutdown-timeout", 15*time.Second, "graceful shutdown timeout")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Dial upstream eagerly so a misconfigured --upstream fails fast
	// rather than masquerading as a healthy console that 503s every
	// RPC. NewClient is lazy; we don't actually open a TCP connection
	// until the first RPC, but address parsing happens here.
	uc, err := dialUpstream(ctx, *upstream, *apiKey)
	if err != nil {
		log.Fatalf("entdb-console: dial upstream: %v", err)
	}
	defer func() { _ = uc.Close() }()

	srv := newConsoleServer(uc)

	mux := http.NewServeMux()

	// ConnectRPC: serves Connect, gRPC, and gRPC-Web on the same path
	// prefix. The path is `/<package>.<service>/<method>` which for us
	// is `/entdb.console.v1.Console/<Method>`.
	connectPath, connectHandler := consolev1connect.NewConsoleHandler(srv)
	mux.Handle(connectPath, connectHandler)

	// SPA fallback for everything else.
	spa, err := newSPAHandler(*apiKey)
	if err != nil {
		log.Fatalf("entdb-console: build SPA handler: %v", err)
	}
	mux.Handle("/", spa)

	// h2c so non-TLS gRPC clients (the in-cluster admin tooling that
	// hits the console binary directly via gRPC, not just browsers)
	// can negotiate HTTP/2 without ALPN. Browsers reach us via HTTP/1
	// + Connect protocol; both work on the same listener.
	httpServer := &http.Server{
		Addr:              *addr,
		Handler:           h2c.NewHandler(mux, &http2.Server{}),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("entdb-console: listening on %s, upstream=%s, connect-path=%s", *addr, *upstream, connectPath)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("entdb-console: serve: %v", err)
		}
	}()

	<-ctx.Done()
	log.Printf("entdb-console: shutting down (timeout=%s)", *shutdownTimeout)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), *shutdownTimeout)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("entdb-console: shutdown: %v", err)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
