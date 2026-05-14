// SPDX-License-Identifier: AGPL-3.0-only

package apply

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/elloloop/tenant-shard-db/server/go/internal/errs"
	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
	"github.com/elloloop/tenant-shard-db/server/go/internal/wal"
)

type globalApplyFailure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// applyGlobalEvent materializes wal.ScopeGlobal events into globalstore.
// It reuses the canonical store's applied_events / applied_offsets
// machinery under wal.GlobalTenantID so global admin RPCs can use the
// same wait-applied contract as tenant-scoped writes.
func (a *Applier) applyGlobalEvent(ctx context.Context, ev *Event, res *Result) error {
	if ev.TenantID != wal.GlobalTenantID {
		return fmt.Errorf("%w: global event tenant_id must be %q", ErrPoisonEvent, wal.GlobalTenantID)
	}
	if a.global == nil {
		return fmt.Errorf("%w: global event without globalstore", ErrPoisonEvent)
	}
	if err := a.store.OpenTenant(ctx, wal.GlobalTenantID); err != nil {
		return fmt.Errorf("apply global: open sentinel tenant: %w", err)
	}

	rec, err := a.store.CheckIdempotencyStatus(ctx, wal.GlobalTenantID, ev.IdempotencyKey)
	if err != nil {
		return fmt.Errorf("apply global: idempotency probe: %w", err)
	}
	if rec.Present {
		if rec.Status == store.IdempotencyStatusFailedPrecondition {
			res.Status = StatusFailedPrecondition
			return nil
		}
		res.Status = StatusSkipped
		return nil
	}

	for i, op := range ev.Ops {
		if op == nil {
			continue
		}
		if err := a.dispatchGlobal(ctx, ev, op); err != nil {
			if isDeterministicGlobalFailure(err) {
				return a.memoizeGlobalApplyFailure(ctx, ev, res, err)
			}
			return fmt.Errorf("apply global op %d: %w", i, err)
		}
	}

	if err := a.store.RecordIdempotency(ctx, wal.GlobalTenantID, ev.IdempotencyKey, res.Position.String()); err != nil {
		if errors.Is(err, store.ErrIdempotencyViolation) {
			res.Status = StatusSkipped
			return nil
		}
		return fmt.Errorf("apply global: record idempotency: %w", err)
	}
	if err := a.store.UpdateAppliedOffset(ctx, wal.GlobalTenantID, res.Position.Topic, res.Position.Partition, res.Position.Offset); err != nil {
		return fmt.Errorf("apply global: offset: %w", err)
	}
	res.TenantID = wal.GlobalTenantID
	res.Status = StatusApplied
	return nil
}

func isDeterministicGlobalFailure(err error) bool {
	return errors.Is(err, errs.ErrAlreadyExists) || errors.Is(err, errs.ErrFailedPrecondition)
}

func (a *Applier) memoizeGlobalApplyFailure(ctx context.Context, ev *Event, res *Result, cause error) error {
	res.TenantID = wal.GlobalTenantID
	res.Status = StatusFailedPrecondition

	failure := globalApplyFailure{
		Code:    "FAILED_PRECONDITION",
		Message: cause.Error(),
	}
	if errors.Is(cause, errs.ErrAlreadyExists) {
		failure.Code = "ALREADY_EXISTS"
	}
	failureJSON, err := json.Marshal(failure)
	if err != nil {
		failureJSON = []byte("")
	}
	if err := a.store.RecordIdempotencyFailure(ctx, wal.GlobalTenantID, ev.IdempotencyKey, res.Position.String(), string(failureJSON)); err != nil {
		if !errors.Is(err, store.ErrIdempotencyViolation) {
			return fmt.Errorf("apply global: memoize deterministic failure: %w", err)
		}
	}
	if err := a.store.UpdateAppliedOffset(ctx, wal.GlobalTenantID, res.Position.Topic, res.Position.Partition, res.Position.Offset); err != nil {
		return fmt.Errorf("apply global: deterministic-failure offset: %w", err)
	}
	return nil
}

func (a *Applier) dispatchGlobal(ctx context.Context, ev *Event, op map[string]any) error {
	switch opTypeOf(op) {
	case OpTenantCreated:
		_, err := a.global.ApplyTenantCreated(ctx, globalstore.TenantCreatedApply{
			TenantID:    stringField(op, "tenant_id"),
			Name:        stringField(op, "name"),
			Region:      stringField(op, "region"),
			OwnerUserID: stringField(op, "owner_user_id"),
			CreatedAt:   int64Field(op, "created_at"),
		})
		return err
	case OpUserCreated:
		_, err := a.global.ApplyUserCreated(ctx, userApplyFromOp(op))
		return err
	case OpMemberAdded:
		return a.global.ApplyMemberAdded(ctx, memberApplyFromOp(op))
	case OpMemberRemoved:
		return a.global.ApplyMemberRemoved(ctx, stringField(op, "tenant_id"), stringField(op, "user_id"))
	case OpMemberRoleChanged:
		return a.global.ApplyMemberRoleChanged(ctx, memberApplyFromOp(op))
	case OpTenantArchived:
		return a.global.ApplyTenantArchived(ctx, stringField(op, "tenant_id"))
	case OpUserUpdated:
		return a.global.ApplyUserUpdated(ctx, userApplyFromOp(op))
	case OpLegalHoldSet:
		return a.global.ApplyLegalHoldSet(ctx, globalstore.LegalHoldApply{
			TenantID:  stringField(op, "tenant_id"),
			HeldBy:    stringField(op, "held_by"),
			Reason:    stringField(op, "reason"),
			CreatedAt: int64Field(op, "created_at"),
			Enabled:   boolField(op, "enabled"),
		})
	case OpUserDeletionScheduled:
		return a.global.ApplyUserDeletionScheduled(ctx, globalstore.DeletionApply{
			UserID:      stringField(op, "user_id"),
			RequestedAt: int64Field(op, "requested_at"),
			ExecuteAt:   int64Field(op, "execute_at"),
			Status:      stringField(op, "status"),
		})
	case OpUserDeletionCanceled:
		return a.global.ApplyUserDeletionCanceled(ctx, stringField(op, "user_id"), int64Field(op, "updated_at"))
	case OpUserFrozen:
		return a.global.ApplyUserFrozen(ctx, stringField(op, "user_id"), stringField(op, "status"), int64Field(op, "updated_at"))
	case OpAccessTransferred:
		return a.global.ApplyAccessTransferred(ctx, stringField(op, "tenant_id"), stringField(op, "to_user"), int64Field(op, "joined_at"))
	case OpAccessRevoked:
		return a.global.ApplyAccessRevoked(ctx, stringField(op, "tenant_id"), stringField(op, "user_id"))
	default:
		return fmt.Errorf("%w: %q", ErrUnknownOpType, opTypeOf(op))
	}
}

func userApplyFromOp(op map[string]any) globalstore.UserApply {
	return globalstore.UserApply{
		UserID:    stringField(op, "user_id"),
		Email:     stringField(op, "email"),
		Name:      stringField(op, "name"),
		Status:    stringField(op, "status"),
		CreatedAt: int64Field(op, "created_at"),
		UpdatedAt: int64Field(op, "updated_at"),
	}
}

func memberApplyFromOp(op map[string]any) globalstore.MemberApply {
	return globalstore.MemberApply{
		TenantID: stringField(op, "tenant_id"),
		UserID:   stringField(op, "user_id"),
		Role:     stringField(op, "role"),
		JoinedAt: int64Field(op, "joined_at"),
	}
}
