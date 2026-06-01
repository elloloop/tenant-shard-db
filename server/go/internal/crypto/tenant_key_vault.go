package crypto

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

const (
	vaultTableDDL = `
CREATE TABLE IF NOT EXISTS tenant_key_vault (
    tenant_id        TEXT PRIMARY KEY NOT NULL,
    wrapped_key      BLOB,
    created_at_ms    INTEGER NOT NULL,
    shredded_at_ms   INTEGER,
    key_origin       TEXT NOT NULL DEFAULT 'derived',
    prev_wrapped_key BLOB,
    CHECK (
        (wrapped_key IS NOT NULL AND shredded_at_ms IS NULL) OR
        (wrapped_key IS NULL AND shredded_at_ms IS NOT NULL)
    )
);
`
	nonceLength = 12

	// key_origin values. A tenant DEK is either deterministically derived
	// from the master key (legacy, re-derivable — finding #638) or a random
	// 256-bit key that exists only as the master-wrapped blob (so a shred is
	// irreversible). 'rekeying' marks a tenant mid-migration: wrapped_key
	// holds the NEW random key, prev_wrapped_key the OLD one, and the DB may
	// be encrypted under either until the migration completes.
	keyOriginDerived  = "derived"
	keyOriginRandom   = "random"
	keyOriginRekeying = "rekeying"
)

var (
	ErrTenantKeyNotFound           = errors.New("crypto: tenant key not found")
	ErrTenantKeyAlreadyProvisioned = errors.New("crypto: tenant key already provisioned")
	ErrVaultAuthentication         = errors.New("crypto: tenant key vault authentication failed")
)

type VaultRow struct {
	TenantID     string
	WrappedKey   []byte
	CreatedAtMS  int64
	ShreddedAtMS sql.NullInt64
}

type TenantKeyVault struct {
	db    *sql.DB
	aead  cipher.AEAD
	nowFn func() time.Time
	mu    sync.Mutex
}

type TenantKeyVaultOptions struct {
	DB        *sql.DB
	MasterKey []byte
	NowFn     func() time.Time
}

func NewTenantKeyVault(ctx context.Context, opts TenantKeyVaultOptions) (*TenantKeyVault, error) {
	if opts.DB == nil {
		return nil, errors.New("crypto: tenant key vault DB is required")
	}
	if len(opts.MasterKey) != KeyLength {
		return nil, fmt.Errorf("%w: got %d", ErrInvalidMasterKey, len(opts.MasterKey))
	}
	block, err := aes.NewCipher(opts.MasterKey)
	if err != nil {
		return nil, fmt.Errorf("crypto: create vault cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: create vault AEAD: %w", err)
	}
	nowFn := opts.NowFn
	if nowFn == nil {
		nowFn = time.Now
	}
	v := &TenantKeyVault{
		db:    opts.DB,
		aead:  aead,
		nowFn: nowFn,
	}
	if err := v.initSchema(ctx); err != nil {
		return nil, err
	}
	return v, nil
}

func (v *TenantKeyVault) initSchema(ctx context.Context) error {
	if _, err := v.db.ExecContext(ctx, vaultTableDDL); err != nil {
		return fmt.Errorf("crypto: create tenant key vault schema: %w", err)
	}
	// Migrate vault DBs created before #638: add the key_origin and
	// prev_wrapped_key columns if absent. Pre-#638 rows hold derived keys,
	// so key_origin defaults to 'derived' — exactly what the re-key
	// migration (MigrateDerivedTenants) looks for.
	return v.ensureColumns(ctx)
}

// ensureColumns adds columns introduced after the table's original shape,
// idempotently. SQLite lacks ADD COLUMN IF NOT EXISTS, so we probe
// table_info first. New columns use constant defaults (allowed by ALTER).
func (v *TenantKeyVault) ensureColumns(ctx context.Context) error {
	rows, err := v.db.QueryContext(ctx, `PRAGMA table_info(tenant_key_vault)`)
	if err != nil {
		return fmt.Errorf("crypto: inspect vault columns: %w", err)
	}
	have := map[string]struct{}{}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			_ = rows.Close()
			return fmt.Errorf("crypto: scan vault column: %w", err)
		}
		have[name] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("crypto: iterate vault columns: %w", err)
	}
	_ = rows.Close()

	adds := []struct {
		col, ddl string
	}{
		{"key_origin", `ALTER TABLE tenant_key_vault ADD COLUMN key_origin TEXT NOT NULL DEFAULT 'derived'`},
		{"prev_wrapped_key", `ALTER TABLE tenant_key_vault ADD COLUMN prev_wrapped_key BLOB`},
	}
	for _, a := range adds {
		if _, ok := have[a.col]; ok {
			continue
		}
		if _, err := v.db.ExecContext(ctx, a.ddl); err != nil {
			return fmt.Errorf("crypto: add vault column %s: %w", a.col, err)
		}
	}
	return nil
}

func (v *TenantKeyVault) GetRow(ctx context.Context, tenantID string) (*VaultRow, error) {
	if tenantID == "" {
		return nil, errors.New("crypto: tenant_id is required")
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.getRowLocked(ctx, tenantID)
}

func (v *TenantKeyVault) Get(ctx context.Context, tenantID string) ([]byte, error) {
	if tenantID == "" {
		return nil, errors.New("crypto: tenant_id is required")
	}
	v.mu.Lock()
	defer v.mu.Unlock()

	row, err := v.getRowLocked(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, fmt.Errorf("%w: %s", ErrTenantKeyNotFound, tenantID)
	}
	if row.ShreddedAtMS.Valid {
		return nil, &TenantShreddedError{TenantID: tenantID}
	}
	if len(row.WrappedKey) == 0 {
		return nil, fmt.Errorf("crypto: vault row %q has no wrapped key", tenantID)
	}
	return v.unwrap(row.WrappedKey, tenantID)
}

func (v *TenantKeyVault) Provision(ctx context.Context, tenantID string, dek []byte) error {
	if tenantID == "" {
		return errors.New("crypto: tenant_id is required")
	}
	if len(dek) != KeyLength {
		return fmt.Errorf("crypto: tenant DEK must be exactly %d bytes, got %d", KeyLength, len(dek))
	}
	v.mu.Lock()
	defer v.mu.Unlock()

	row, err := v.getRowLocked(ctx, tenantID)
	if err != nil {
		return err
	}
	if row != nil {
		if row.ShreddedAtMS.Valid {
			return &TenantShreddedError{TenantID: tenantID}
		}
		return fmt.Errorf("%w: %s", ErrTenantKeyAlreadyProvisioned, tenantID)
	}
	wrapped, err := v.wrap(dek, tenantID)
	if err != nil {
		return err
	}
	if _, err := v.db.ExecContext(ctx,
		`INSERT INTO tenant_key_vault (tenant_id, wrapped_key, created_at_ms, key_origin) VALUES (?, ?, ?, ?)`,
		tenantID, wrapped, v.nowFn().UnixMilli(), keyOriginRandom,
	); err != nil {
		return fmt.Errorf("crypto: provision tenant key %q: %w", tenantID, err)
	}
	return nil
}

func (v *TenantKeyVault) Shred(ctx context.Context, tenantID string) error {
	if tenantID == "" {
		return errors.New("crypto: tenant_id is required")
	}
	v.mu.Lock()
	defer v.mu.Unlock()

	row, err := v.getRowLocked(ctx, tenantID)
	if err != nil {
		return err
	}
	if row == nil {
		return fmt.Errorf("%w: %s", ErrTenantKeyNotFound, tenantID)
	}
	if row.ShreddedAtMS.Valid {
		return nil
	}
	if _, err := v.db.ExecContext(ctx,
		`UPDATE tenant_key_vault SET wrapped_key = NULL, shredded_at_ms = ? WHERE tenant_id = ?`,
		v.nowFn().UnixMilli(), tenantID,
	); err != nil {
		return fmt.Errorf("crypto: shred tenant key %q: %w", tenantID, err)
	}
	return nil
}

func (v *TenantKeyVault) IsShredded(ctx context.Context, tenantID string) (bool, error) {
	row, err := v.GetRow(ctx, tenantID)
	if err != nil {
		return false, err
	}
	return row != nil && row.ShreddedAtMS.Valid, nil
}

func (v *TenantKeyVault) RewrapWithNewMaster(ctx context.Context, newMasterKey []byte) (int, error) {
	if len(newMasterKey) != KeyLength {
		return 0, fmt.Errorf("%w: got %d", ErrInvalidMasterKey, len(newMasterKey))
	}
	block, err := aes.NewCipher(newMasterKey)
	if err != nil {
		return 0, fmt.Errorf("crypto: create new vault cipher: %w", err)
	}
	newAEAD, err := cipher.NewGCM(block)
	if err != nil {
		return 0, fmt.Errorf("crypto: create new vault AEAD: %w", err)
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	tx, err := v.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("crypto: begin vault rewrap: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, `SELECT tenant_id, wrapped_key FROM tenant_key_vault WHERE shredded_at_ms IS NULL`)
	if err != nil {
		return 0, fmt.Errorf("crypto: list active tenant keys: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type update struct {
		tenantID string
		wrapped  []byte
	}
	updates := []update{}
	for rows.Next() {
		var tenantID string
		var wrapped []byte
		if err := rows.Scan(&tenantID, &wrapped); err != nil {
			return 0, fmt.Errorf("crypto: scan tenant key for rewrap: %w", err)
		}
		dek, err := v.unwrap(wrapped, tenantID)
		if err != nil {
			return 0, err
		}
		newWrapped, err := wrapWithAEAD(newAEAD, dek, tenantID)
		if err != nil {
			return 0, err
		}
		updates = append(updates, update{tenantID: tenantID, wrapped: newWrapped})
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("crypto: iterate tenant keys for rewrap: %w", err)
	}
	if err := rows.Close(); err != nil {
		return 0, fmt.Errorf("crypto: close tenant key cursor for rewrap: %w", err)
	}
	for _, upd := range updates {
		if _, err := tx.ExecContext(ctx, `UPDATE tenant_key_vault SET wrapped_key = ? WHERE tenant_id = ?`, upd.wrapped, upd.tenantID); err != nil {
			return 0, fmt.Errorf("crypto: update rewrapped tenant key %q: %w", upd.tenantID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("crypto: commit vault rewrap: %w", err)
	}
	v.aead = newAEAD
	return len(updates), nil
}

// --- #638 re-key migration primitives (package-internal) ---

// rekeyCandidates returns the tenant_ids still needing migration to a random
// key: non-shredded rows whose origin is 'derived' (never migrated) or
// 'rekeying' (a prior migration was interrupted). Ordered for deterministic,
// resumable runs.
func (v *TenantKeyVault) rekeyCandidates(ctx context.Context) ([]string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	rows, err := v.db.QueryContext(ctx,
		`SELECT tenant_id FROM tenant_key_vault
		 WHERE shredded_at_ms IS NULL AND key_origin IN (?, ?)
		 ORDER BY tenant_id`, keyOriginDerived, keyOriginRekeying)
	if err != nil {
		return nil, fmt.Errorf("crypto: list rekey candidates: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("crypto: scan rekey candidate: %w", err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("crypto: iterate rekey candidates: %w", err)
	}
	return out, nil
}

// keyOriginOf returns the key_origin for tenantID, or "" if the row is absent.
func (v *TenantKeyVault) keyOriginOf(ctx context.Context, tenantID string) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	var origin string
	err := v.db.QueryRowContext(ctx,
		`SELECT key_origin FROM tenant_key_vault WHERE tenant_id = ?`, tenantID,
	).Scan(&origin)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("crypto: read key_origin %q: %w", tenantID, err)
	}
	return origin, nil
}

// prevKey unwraps the previous (pre-rekey) DEK staged for a tenant that is
// mid-migration. Used by recovery to re-key a DB still on its old key.
func (v *TenantKeyVault) prevKey(ctx context.Context, tenantID string) ([]byte, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	var prev []byte
	if err := v.db.QueryRowContext(ctx,
		`SELECT prev_wrapped_key FROM tenant_key_vault WHERE tenant_id = ?`, tenantID,
	).Scan(&prev); err != nil {
		return nil, fmt.Errorf("crypto: read prev_wrapped_key %q: %w", tenantID, err)
	}
	if len(prev) == 0 {
		return nil, fmt.Errorf("crypto: tenant %q has no staged previous key", tenantID)
	}
	return v.unwrap(prev, tenantID)
}

// stageRekey atomically moves a 'derived' row into 'rekeying': the current
// wrapped_key is preserved as prev_wrapped_key and the master-wrapped newDEK
// becomes the new wrapped_key. The WHERE clause pins the source state to
// 'derived', so a re-run after a crash (row already 'rekeying') is a no-op
// that cannot clobber the staged previous key.
func (v *TenantKeyVault) stageRekey(ctx context.Context, tenantID string, newDEK []byte) error {
	if len(newDEK) != KeyLength {
		return fmt.Errorf("crypto: rekey DEK must be %d bytes, got %d", KeyLength, len(newDEK))
	}
	wrapped, err := v.wrap(newDEK, tenantID)
	if err != nil {
		return err
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	res, err := v.db.ExecContext(ctx,
		`UPDATE tenant_key_vault
		    SET prev_wrapped_key = wrapped_key, wrapped_key = ?, key_origin = ?
		  WHERE tenant_id = ? AND key_origin = ?`,
		wrapped, keyOriginRekeying, tenantID, keyOriginDerived,
	)
	if err != nil {
		return fmt.Errorf("crypto: stage rekey %q: %w", tenantID, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("crypto: stage rekey %q: row not in %q state", tenantID, keyOriginDerived)
	}
	return nil
}

// completeRekey finalizes a migrated tenant: it drops the staged previous key
// and marks the origin 'random'. After this the old derived key is gone from
// the vault, so a subsequent shred is genuinely irreversible.
func (v *TenantKeyVault) completeRekey(ctx context.Context, tenantID string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if _, err := v.db.ExecContext(ctx,
		`UPDATE tenant_key_vault SET prev_wrapped_key = NULL, key_origin = ?
		  WHERE tenant_id = ?`,
		keyOriginRandom, tenantID,
	); err != nil {
		return fmt.Errorf("crypto: complete rekey %q: %w", tenantID, err)
	}
	return nil
}

// newRandomKey returns a fresh cryptographically-random 256-bit DEK.
func newRandomKey() ([]byte, error) {
	key := make([]byte, KeyLength)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("crypto: generate random tenant key: %w", err)
	}
	return key, nil
}

func (v *TenantKeyVault) getRowLocked(ctx context.Context, tenantID string) (*VaultRow, error) {
	row := v.db.QueryRowContext(ctx,
		`SELECT tenant_id, wrapped_key, created_at_ms, shredded_at_ms FROM tenant_key_vault WHERE tenant_id = ?`,
		tenantID,
	)
	var out VaultRow
	if err := row.Scan(&out.TenantID, &out.WrappedKey, &out.CreatedAtMS, &out.ShreddedAtMS); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("crypto: read tenant key row %q: %w", tenantID, err)
	}
	return &out, nil
}

func (v *TenantKeyVault) wrap(dek []byte, tenantID string) ([]byte, error) {
	return wrapWithAEAD(v.aead, dek, tenantID)
}

func (v *TenantKeyVault) unwrap(blob []byte, tenantID string) ([]byte, error) {
	if len(blob) <= nonceLength {
		return nil, fmt.Errorf("crypto: vault row %q is too short", tenantID)
	}
	nonce := blob[:nonceLength]
	ciphertext := blob[nonceLength:]
	dek, err := v.aead.Open(nil, nonce, ciphertext, []byte(tenantID))
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrVaultAuthentication, tenantID)
	}
	if len(dek) != KeyLength {
		return nil, fmt.Errorf("crypto: vault row %q unwrapped to %d bytes, want %d", tenantID, len(dek), KeyLength)
	}
	return dek, nil
}

func wrapWithAEAD(aead cipher.AEAD, dek []byte, tenantID string) ([]byte, error) {
	nonce := make([]byte, nonceLength)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("crypto: generate vault nonce: %w", err)
	}
	ciphertext := aead.Seal(nil, nonce, dek, []byte(tenantID))
	out := make([]byte, 0, len(nonce)+len(ciphertext))
	out = append(out, nonce...)
	out = append(out, ciphertext...)
	return out, nil
}
