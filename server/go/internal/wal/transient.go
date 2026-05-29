// SPDX-License-Identifier: AGPL-3.0-only

package wal

import "errors"

// ErrTransient marks a WAL error that is operationally transient — the
// caller (the applier consumer loop) should retry with backoff rather
// than treat it as fatal and exit the process.
//
// Backends wrap their transient causes with this sentinel (in addition
// to ErrWal) so the applier can distinguish "retry" from "deployment
// fatal" without re-classifying backend-specific error types itself.
// Transient causes include idle-connection reaping (io.EOF), network
// read/write timeouts, broker rebalances, and leader elections — the
// errors that recur against managed Kafka surfaces such as Azure Event
// Hubs and must not crash-loop the server. See issue #627.
var ErrTransient = errors.New("wal: transient error")

// IsTransient reports whether err is an operationally-transient WAL
// error that the consumer loop should retry with backoff instead of
// treating as fatal. It is the public classifier each backend's
// transient tagging feeds into.
func IsTransient(err error) bool {
	return errors.Is(err, ErrTransient)
}
