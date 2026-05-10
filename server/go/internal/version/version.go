// Package version exposes the build-stamped server semver returned by the
// entdb.v1.EntDBService/Health RPC.
//
// The default value is "dev". Release builds override it with a linker flag,
// e.g.:
//
//	go build -ldflags "-X github.com/elloloop/tenant-shard-db/server/go/internal/version.Version=v1.2.3" ./cmd/entdb-server
//
// Mirrors server/python/entdb_server/_version.py (which hard-codes the
// Python server's version at module level).
package version

// Version is the server semver. Defaults to "dev" and is overridden at link
// time via `-ldflags "-X .../version.Version=<semver>"`.
var Version = "dev"

// String returns the current Version.
//
// Health handlers should prefer reading [Version] directly when populating
// HealthResponse.Version; String exists for callers that want a stringer-style
// helper (e.g. logging).
func String() string {
	return Version
}
