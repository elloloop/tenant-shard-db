// Cross-implementation contract tests, kept in their own Go module so
// they can be invoked independently from the server module's test
// suite and so they can exercise the public `entdb-schema` CLI as a
// black box (rather than reaching into server/go/internal/*, which
// Go's internal-package rule would forbid from a sibling module
// anyway).
//
// Tests in this module build the CLI from source on first call (or
// honour ENTDB_SCHEMA_BIN if set to a path — used in CI when the
// released artifact is available).
module github.com/elloloop/tenant-shard-db/tests/contract

go 1.25.0
