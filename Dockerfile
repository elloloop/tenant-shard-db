# EntDB Server (Go) Dockerfile
#
# Multi-stage build that compiles server/go/cmd/entdb-server into a
# minimal distroless runtime image. Uses modernc.org/sqlite (pure-Go
# SQLite) so CGO_ENABLED=0 and the binary lands in a "static" distroless
# base — no glibc, no shell, no package manager.
#
# Wave 5 (EPIC #407) ships this image alongside the existing Python
# server image (see ../Dockerfile) so the E2E suite can target either
# implementation by switching ENTDB_SERVER_TARGET={python,go}.
#
# Build:
#   docker build -f Dockerfile.go -t entdb-server-go:test .
#
# Run (server picks up flags, NOT env vars — the Python server uses env;
# the Go binary takes -addr / -data-dir / -wal-backend / -seed-tenant):
#   docker run --rm -p 50051:50051 entdb-server-go:test \
#       -addr=:50051 -data-dir=/var/lib/entdb -wal-backend=memory
#
# Inspect flags:
#   docker run --rm entdb-server-go:test -help

# =============================================================================
# Builder stage — compile the binary with CGO disabled
# =============================================================================
FROM golang:1.25-bookworm AS builder

WORKDIR /src

# Cache modules layer separately from source. The Go server module lives
# at server/go (separate go.mod from the SDK).
COPY server/go/go.mod server/go/go.sum ./server/go/
RUN cd server/go && go mod download

# Bring in the rest of the Go server source.
COPY server/go/ ./server/go/

# CGO disabled so the resulting binary is statically linked and runs on
# `gcr.io/distroless/static-debian12`. modernc.org/sqlite is pure Go, so
# this works for the persistence layer too.
RUN cd server/go && \
    CGO_ENABLED=0 GOOS=linux go build \
        -trimpath -ldflags="-s -w" \
        -o /entdb-server \
        ./cmd/entdb-server

# Pre-create the data directory with the right ownership for the
# nonroot distroless user (uid/gid 65532). distroless/static has no
# shell, so we cannot mkdir at runtime — the directory has to be
# baked into the final image with the right ownership.
RUN mkdir -p /staging/var/lib/entdb && chown -R 65532:65532 /staging

# =============================================================================
# Runtime stage — distroless static, just the binary
# =============================================================================
FROM gcr.io/distroless/static-debian12:nonroot AS server

ARG VERSION=dev
ARG COMMIT=unknown

LABEL org.opencontainers.image.title="EntDB Server (Go)" \
      org.opencontainers.image.description="Go reimplementation of the EntDB tenant-sharded graph gRPC server" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.source="https://github.com/elloloop/tenant-shard-db" \
      org.opencontainers.image.licenses="AGPL-3.0-only" \
      org.opencontainers.image.vendor="elloloop"

COPY --from=builder /entdb-server /entdb-server
COPY --from=builder --chown=65532:65532 /staging/var/lib/entdb /var/lib/entdb

# Default port. The Go binary accepts -addr=host:port; docker-compose /
# the E2E harness override the flag explicitly when needed.
EXPOSE 50051

USER nonroot:nonroot

# distroless/static has no shell, so we must use the exec form. The
# default flags wire up an in-memory WAL (the only backend the Go server
# supports today — Kafka/etc land in a later wave) and point the data
# directory at /var/lib/entdb. Override the full command at `docker run`
# / `docker compose run` time to pass --seed-tenant or other test flags.
ENTRYPOINT ["/entdb-server"]
CMD ["-addr=:50051", "-data-dir=/var/lib/entdb", "-wal-backend=memory"]
