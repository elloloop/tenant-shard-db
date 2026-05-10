// SPDX-License-Identifier: AGPL-3.0-only

package apply

import (
	"fmt"

	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
)

// Sentinel errors returned by the applier. Each wraps an internal/errs
// sentinel so the gRPC layer maps them to the Python server's status
// codes (see docs/go-port/shared/error-mapping.md).
var (
	// ErrPoisonEvent signals an event the applier refuses to apply
	// because it is structurally malformed (missing required fields,
	// unknown op-type, etc.). Halts the consumer; mirrors the Python
	// halt_on_error=True behaviour.
	ErrPoisonEvent = fmt.Errorf("%w: poison event", errs.ErrInvalidArgument)

	// ErrUnknownOpType signals an op-type the applier does not know
	// about. A typo in a handler that raced ahead of the applier is the
	// usual cause; failing closed avoids silent data loss.
	ErrUnknownOpType = fmt.Errorf("%w: unknown op type", errs.ErrInvalidArgument)

	// ErrApplierClosed signals Run was called on a stopped applier (or
	// Stop was called and Run subsequently returned).
	ErrApplierClosed = fmt.Errorf("%w: applier closed", errs.ErrFailedPrecondition)
)
