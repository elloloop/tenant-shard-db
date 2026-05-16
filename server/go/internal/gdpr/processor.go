// Package gdpr executes due account-deletion jobs and performs durable
// crypto-shred for personal tenants.
package gdpr

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	entcrypto "github.com/elloloop/tenant-shard-db/server/go/internal/crypto"
	"github.com/elloloop/tenant-shard-db/server/go/internal/globalstore"
	"github.com/elloloop/tenant-shard-db/server/go/internal/store"
)

// Options configures a deletion processor.
type Options struct {
	Global                *globalstore.GlobalStore
	Store                 *store.CanonicalStore
	Keys                  *entcrypto.KeyManager
	NowFn                 func() int64
	RemoveCiphertextFiles bool
}

// Processor runs executable GDPR deletion_queue entries.
type Processor struct {
	global                *globalstore.GlobalStore
	store                 *store.CanonicalStore
	keys                  *entcrypto.KeyManager
	nowFn                 func() int64
	removeCiphertextFiles bool
}

// Summary reports one RunOnce pass.
type Summary struct {
	EntriesCompleted   int
	PersonalTenants    int
	TenantKeysShredded int
	TenantFilesDeleted int
}

// New constructs a Processor.
func New(opts Options) (*Processor, error) {
	if opts.Global == nil {
		return nil, errors.New("gdpr: global store is required")
	}
	nowFn := opts.NowFn
	if nowFn == nil {
		nowFn = func() int64 { return time.Now().Unix() }
	}
	return &Processor{
		global:                opts.Global,
		store:                 opts.Store,
		keys:                  opts.Keys,
		nowFn:                 nowFn,
		removeCiphertextFiles: opts.RemoveCiphertextFiles,
	}, nil
}

// Run executes due deletions immediately and then at interval until ctx
// is cancelled.
func (p *Processor) Run(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		return errors.New("gdpr: interval must be positive")
	}
	for {
		if _, err := p.RunOnce(ctx); err != nil {
			return err
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

// RunOnce processes every due pending deletion.
func (p *Processor) RunOnce(ctx context.Context) (*Summary, error) {
	entries, err := p.global.GetExecutableDeletions(ctx, p.nowFn())
	if err != nil {
		return nil, err
	}
	summary := &Summary{}
	for _, entry := range entries {
		if err := p.processEntry(ctx, entry, summary); err != nil {
			return summary, err
		}
	}
	return summary, nil
}

func (p *Processor) processEntry(ctx context.Context, entry *globalstore.DeletionEntry, summary *Summary) error {
	memberships, err := p.global.GetUserTenants(ctx, entry.UserID)
	if err != nil {
		return fmt.Errorf("gdpr: list user tenants %q: %w", entry.UserID, err)
	}
	for _, membership := range memberships {
		personal, err := p.isPersonalTenant(ctx, entry.UserID, membership.TenantID)
		if err != nil {
			return err
		}
		if !personal {
			continue
		}
		summary.PersonalTenants++
		if err := p.cryptoShredTenant(ctx, membership.TenantID, summary); err != nil {
			return err
		}
		if ok, err := p.global.SetTenantStatus(ctx, membership.TenantID, "deleted"); err != nil {
			return fmt.Errorf("gdpr: mark tenant %q deleted: %w", membership.TenantID, err)
		} else if !ok {
			return fmt.Errorf("gdpr: tenant %q disappeared during deletion", membership.TenantID)
		}
	}
	if _, err := p.global.RemoveAllSharedForUser(ctx, entry.UserID); err != nil {
		return err
	}
	if _, err := p.global.RemoveAllMembershipsForUser(ctx, entry.UserID); err != nil {
		return err
	}
	if ok, err := p.global.DeleteUser(ctx, entry.UserID); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("gdpr: user %q disappeared during deletion", entry.UserID)
	}
	if ok, err := p.global.MarkDeletionCompleted(ctx, entry.UserID); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("gdpr: deletion row for %q disappeared during deletion", entry.UserID)
	}
	summary.EntriesCompleted++
	return nil
}

func (p *Processor) isPersonalTenant(ctx context.Context, userID, tenantID string) (bool, error) {
	members, err := p.global.GetTenantMembers(ctx, tenantID)
	if err != nil {
		return false, fmt.Errorf("gdpr: list tenant members %q: %w", tenantID, err)
	}
	if len(members) != 1 {
		return false, nil
	}
	return members[0].UserID == userID && members[0].Role == "owner", nil
}

func (p *Processor) cryptoShredTenant(ctx context.Context, tenantID string, summary *Summary) error {
	if p.store != nil {
		if err := p.store.CloseTenant(tenantID); err != nil {
			return fmt.Errorf("gdpr: close tenant %q before shred: %w", tenantID, err)
		}
	}
	if p.keys != nil {
		if err := p.keys.ShredTenant(ctx, tenantID); err != nil && !errors.Is(err, entcrypto.ErrTenantShredded) {
			return fmt.Errorf("gdpr: shred tenant key %q: %w", tenantID, err)
		}
		summary.TenantKeysShredded++
	}
	if p.removeCiphertextFiles && p.store != nil {
		deleted, err := p.deleteTenantFiles(tenantID)
		if err != nil {
			return err
		}
		summary.TenantFilesDeleted += deleted
	}
	return nil
}

func (p *Processor) deleteTenantFiles(tenantID string) (int, error) {
	path, err := p.store.TenantDBPath(tenantID)
	if err != nil {
		return 0, err
	}
	deleted := 0
	for _, candidate := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.Remove(candidate); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return deleted, fmt.Errorf("gdpr: delete tenant file %q: %w", candidate, err)
		}
		deleted++
	}
	return deleted, nil
}
