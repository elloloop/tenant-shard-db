package store

// export_test.go exposes unexported test seams to the external
// store_test package. Compiled only under `go test`.

// WithPreCommitHook returns a copy of opts with the test-only
// pre-commit hook installed. The hook fires inside BatchTxn.Commit
// after every write (including the UpdateAppliedOffsetTx offset row)
// but strictly before the SQL COMMIT — used by the ADR-026 condition-2
// regression test to make the broadcast-before-commit read-after-write
// window deterministic.
func WithPreCommitHook(opts Options, hook func()) Options {
	opts.preCommitHook = hook
	return opts
}
