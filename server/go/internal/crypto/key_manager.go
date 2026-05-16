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
		seed, err := m.deriveTenantKey(tenantID)
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
	seed, err := m.deriveTenantKey(tenantID)
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

func (m *KeyManager) deriveTenantKey(tenantID string) ([]byte, error) {
	reader := hkdf.New(sha256.New, m.masterKey, nil, []byte(hkdfInfoPrefix+tenantID))
	key := make([]byte, KeyLength)
	if _, err := io.ReadFull(reader, key); err != nil {
		return nil, fmt.Errorf("crypto: derive tenant key: %w", err)
	}
	return key, nil
}
