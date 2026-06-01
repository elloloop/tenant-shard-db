// Package crypto contains the encryption-at-rest key hierarchy used by
// the Go server port. It owns key material, KMS bootstrap, SQLCipher
// DSN helpers, and vault persistence; tenant connection pooling lives
// in the store package.
package crypto

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sort"
	"sync"

	"golang.org/x/crypto/hkdf"
)

const (
	KeyLength      = 32
	hkdfInfoPrefix = "entdb-tenant-key:"
)

var (
	ErrInvalidMasterKey = errors.New("crypto: master key must be exactly 32 bytes")
	ErrTenantShredded   = errors.New("crypto: tenant key has been shredded")
)

type TenantShreddedError struct {
	TenantID string
}

func (e *TenantShreddedError) Error() string {
	return fmt.Sprintf("crypto: tenant %q key has been shredded", e.TenantID)
}

func (e *TenantShreddedError) Unwrap() error { return ErrTenantShredded }

type KeyManager struct {
	masterKey []byte
	vault     *TenantKeyVault

	mu       sync.Mutex
	cache    map[string][]byte
	shredded map[string]struct{}
}

func NewKeyManager(masterKey []byte, vault *TenantKeyVault) (*KeyManager, error) {
	if len(masterKey) != KeyLength {
		return nil, fmt.Errorf("%w: got %d", ErrInvalidMasterKey, len(masterKey))
	}
	return &KeyManager{
		masterKey: append([]byte(nil), masterKey...),
		vault:     vault,
		cache:     map[string][]byte{},
		shredded:  map[string]struct{}{},
	}, nil
}

func ParseMasterKeyHex(s string) ([]byte, error) {
	key, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("crypto: parse master key hex: %w", err)
	}
	if len(key) != KeyLength {
		return nil, fmt.Errorf("%w: got %d", ErrInvalidMasterKey, len(key))
	}
	return key, nil
}

func (m *KeyManager) TenantKey(ctx context.Context, tenantID string) ([]byte, error) {
	if tenantID == "" {
		return nil, errors.New("crypto: tenant_id is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.shredded[tenantID]; ok {
		return nil, &TenantShreddedError{TenantID: tenantID}
	}
	if key, ok := m.cache[tenantID]; ok {
		return append([]byte(nil), key...), nil
	}

	var key []byte
	var err error
	if m.vault != nil {
		key, err = m.fetchOrProvisionVault(ctx, tenantID)
	} else {
		key, err = m.deriveTenantKey(tenantID)
	}
	if err != nil {
		if errors.Is(err, ErrTenantShredded) {
			m.shredded[tenantID] = struct{}{}
		}
		return nil, err
	}
	m.cache[tenantID] = append([]byte(nil), key...)
	return append([]byte(nil), key...), nil
}

func (m *KeyManager) ShredTenant(ctx context.Context, tenantID string) error {
	if tenantID == "" {
		return errors.New("crypto: tenant_id is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.cache, tenantID)
	if m.vault == nil {
		m.shredded[tenantID] = struct{}{}
		return nil
	}
	row, err := m.vault.GetRow(ctx, tenantID)
	if err != nil {
		return err
	}
	if row == nil {
		seed, err := newRandomKey()
		if err != nil {
			return err
		}
		if err := m.vault.Provision(ctx, tenantID, seed); err != nil && !errors.Is(err, ErrTenantKeyAlreadyProvisioned) {
			return err
		}
	}
	if err := m.vault.Shred(ctx, tenantID); err != nil {
		return err
	}
	m.shredded[tenantID] = struct{}{}
	return nil
}

func (m *KeyManager) IsShredded(ctx context.Context, tenantID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.shredded[tenantID]; ok {
		return true, nil
	}
	if m.vault == nil {
		return false, nil
	}
	shredded, err := m.vault.IsShredded(ctx, tenantID)
	if err != nil {
		return false, err
	}
	if shredded {
		m.shredded[tenantID] = struct{}{}
	}
	return shredded, nil
}

func (m *KeyManager) CachedTenantIDs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.cache))
	for tenantID := range m.cache {
		out = append(out, tenantID)
	}
	sort.Strings(out)
	return out
}

func (m *KeyManager) fetchOrProvisionVault(ctx context.Context, tenantID string) ([]byte, error) {
	key, err := m.vault.Get(ctx, tenantID)
	if err == nil {
		return key, nil
	}
	if errors.Is(err, ErrTenantShredded) {
		return nil, err
	}
	if !errors.Is(err, ErrTenantKeyNotFound) {
		return nil, err
	}
	// #638: provision a fresh RANDOM DEK, not a master-derived one. Only the
	// master-wrapped copy is persisted (vault.Provision), so nulling it on
	// shred is irreversible. Legacy rows that still wrap a derived key are
	// migrated by MigrateDerivedTenants.
	seed, err := newRandomKey()
	if err != nil {
		return nil, err
	}
	if err := m.vault.Provision(ctx, tenantID, seed); err != nil {
		if errors.Is(err, ErrTenantKeyAlreadyProvisioned) {
			return m.vault.Get(ctx, tenantID)
		}
		return nil, err
	}
	return seed, nil
}

// deriveTenantKey is the legacy deterministic key derivation. As of #638 it
// is used ONLY in vault-less mode (KeyManager built with a nil vault — a
// dev/test convenience): with no vault there is nowhere to persist a random
// key, so the DEK must be reproducible, and crypto-shred is necessarily
// non-durable (it survives only as the in-memory `shredded` flag and does not
// outlast the process). Vault-backed deployments provision random keys
// (fetchOrProvisionVault) and never call this. The function is retained both
// for that vault-less path and so existing vault rows minted before #638 —
// which wrap a derived key — remain openable until MigrateDerivedTenants
// re-keys them.
func (m *KeyManager) deriveTenantKey(tenantID string) ([]byte, error) {
	reader := hkdf.New(sha256.New, m.masterKey, nil, []byte(hkdfInfoPrefix+tenantID))
	key := make([]byte, KeyLength)
	if _, err := io.ReadFull(reader, key); err != nil {
		return nil, fmt.Errorf("crypto: derive tenant key: %w", err)
	}
	return key, nil
}

// RekeyDBFunc re-encrypts tenantID's database so it ends encrypted under
// newKey. It is called knowing the DB is currently under oldKey, but MUST be
// idempotent: a crash can leave the DB on either key, so the implementation
// should succeed whether the file is already on newKey, still on oldKey, or
// absent (a never-written tenant — a no-op). Implemented by the store package
// via SQLCipher PRAGMA rekey.
type RekeyDBFunc func(ctx context.Context, tenantID string, oldKey, newKey []byte) error

// RekeyReport summarizes a MigrateDerivedTenants run.
type RekeyReport struct {
	// Rekeyed lists tenants migrated from a derived key to a fresh random key.
	Rekeyed []string
	// Recovered lists tenants whose interrupted ('rekeying') migration was
	// detected and completed on this run.
	Recovered []string
}

// DerivedTenantIDs returns the tenants still on a legacy derived key (or with
// an interrupted migration) — the set MigrateDerivedTenants would act on.
// Intended for a dry-run / status check. Empty when there is no vault.
func (m *KeyManager) DerivedTenantIDs(ctx context.Context) ([]string, error) {
	if m.vault == nil {
		return nil, nil
	}
	return m.vault.rekeyCandidates(ctx)
}

// MigrateDerivedTenants re-keys every tenant whose DEK is still the legacy
// deterministic derived key (finding #638) to a fresh random key, so a later
// crypto-shred is irreversible. Idempotent and crash-safe:
//
//   - derived tenant: generate a random DEK, stage it in the vault (the old
//     key is preserved as prev_wrapped_key), re-key the DB, then finalize.
//   - 'rekeying' tenant (a prior run crashed mid-flight): the new key is
//     already the vault's wrapped_key and the old key is prev_wrapped_key;
//     re-run the (idempotent) DB re-key and finalize.
//
// Already-random tenants are skipped. Requires a vault. Run as an operator
// step with the server stopped, so no pooled handle holds a tenant DB open
// while it is re-keyed.
func (m *KeyManager) MigrateDerivedTenants(ctx context.Context, rekeyDB RekeyDBFunc) (*RekeyReport, error) {
	if m.vault == nil {
		return nil, errors.New("crypto: tenant key migration requires a key vault")
	}
	if rekeyDB == nil {
		return nil, errors.New("crypto: rekeyDB callback is required")
	}
	candidates, err := m.vault.rekeyCandidates(ctx)
	if err != nil {
		return nil, err
	}
	report := &RekeyReport{}
	for _, tenantID := range candidates {
		origin, err := m.vault.keyOriginOf(ctx, tenantID)
		if err != nil {
			return report, err
		}
		switch origin {
		case keyOriginRekeying:
			// Resume an interrupted migration: wrapped_key is already the new
			// key, prev_wrapped_key the old one.
			oldKey, err := m.vault.prevKey(ctx, tenantID)
			if err != nil {
				return report, err
			}
			newKey, err := m.vault.Get(ctx, tenantID)
			if err != nil {
				return report, err
			}
			if err := rekeyDB(ctx, tenantID, oldKey, newKey); err != nil {
				return report, fmt.Errorf("crypto: resume rekey %q: %w", tenantID, err)
			}
			if err := m.vault.completeRekey(ctx, tenantID); err != nil {
				return report, err
			}
			report.Recovered = append(report.Recovered, tenantID)
		case keyOriginDerived:
			oldKey, err := m.vault.Get(ctx, tenantID)
			if err != nil {
				return report, err
			}
			newKey, err := newRandomKey()
			if err != nil {
				return report, err
			}
			if err := m.vault.stageRekey(ctx, tenantID, newKey); err != nil {
				return report, err
			}
			if err := rekeyDB(ctx, tenantID, oldKey, newKey); err != nil {
				return report, fmt.Errorf("crypto: rekey %q: %w", tenantID, err)
			}
			if err := m.vault.completeRekey(ctx, tenantID); err != nil {
				return report, err
			}
			report.Rekeyed = append(report.Rekeyed, tenantID)
		default:
			// 'random' (or unknown) — already safe, nothing to do.
		}
		// Drop any cached copy so a long-lived manager re-reads the new key.
		m.mu.Lock()
		delete(m.cache, tenantID)
		m.mu.Unlock()
	}
	return report, nil
}
