// api_keys CRUD. Backs auth.PersistentAPIKeyManager: durable, rotatable,
// scopeable, revocable API keys.
//
// globalstore stores only the argon2id PHC hash (produced by the auth
// package) -- never the plaintext secret. This file is pure persistence;
// hashing and constant-time verification live in
// server/go/internal/auth/apikey.go.
//
// Rotation model: multiple rows can be status='active' for the same
// tenant/name at once. The migration window is "issue the new key,
// flip clients over, then RevokeAPIKey the old key_id". ListActiveAPIKeys
// returns every still-valid key so the auth layer can accept any of them
// during the overlap.

package globalstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// encodeScopes joins a scope slice into the comma-separated TEXT column.
// Empty/whitespace scopes are dropped so a round-trip is stable.
func encodeScopes(scopes []string) string {
	out := make([]string, 0, len(scopes))
	for _, s := range scopes {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return strings.Join(out, ",")
}

// decodeScopes splits the stored TEXT column back into a slice. The
// empty string decodes to a nil slice (not []string{""}).
func decodeScopes(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// PutAPIKey inserts a new API-key row. The key_id is the primary key;
// re-inserting an existing key_id is a UNIQUE violation surfaced as an
// error (callers generate a fresh key_id per issued key, so a collision
// is a bug, not a re-issue path). Hash is the argon2id PHC string;
// globalstore does not validate its shape.
func (g *GlobalStore) PutAPIKey(ctx context.Context, rec APIKeyRecord) error {
	if rec.KeyID == "" {
		return errors.New("globalstore: PutAPIKey requires key_id")
	}
	if rec.Hash == "" {
		return errors.New("globalstore: PutAPIKey requires hash")
	}
	status := rec.Status
	if status == "" {
		status = APIKeyStatusActive
	}
	created := rec.CreatedAt
	if created == 0 {
		created = g.now()
	}
	_, err := g.db.ExecContext(ctx,
		`INSERT INTO api_keys
		     (key_id, tenant_id, name, hash, scopes, status,
		      created_at, expires_at, revoked_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.KeyID, rec.TenantID, rec.Name, rec.Hash,
		encodeScopes(rec.Scopes), status,
		created, rec.ExpiresAt, rec.RevokedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("globalstore: api key %q already exists: %w", rec.KeyID, err)
		}
		return fmt.Errorf("globalstore: put api key %q: %w", rec.KeyID, err)
	}
	return nil
}

// GetAPIKey returns the row for key_id, or (nil, nil) if absent.
func (g *GlobalStore) GetAPIKey(ctx context.Context, keyID string) (*APIKeyRecord, error) {
	row := g.db.QueryRowContext(ctx,
		`SELECT key_id, tenant_id, name, hash, scopes, status,
		        created_at, expires_at, revoked_at
		 FROM api_keys WHERE key_id = ?`,
		keyID,
	)
	rec, err := scanAPIKeyRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("globalstore: get api key %q: %w", keyID, err)
	}
	return rec, nil
}

// ListAPIKeys returns every api_keys row, newest first. tenantID == ""
// lists across all tenants (the persistent manager loads the full set
// into memory at boot, then re-syncs on demand). limit <= 0 -> unlimited.
func (g *GlobalStore) ListAPIKeys(ctx context.Context, tenantID string, limit int) ([]*APIKeyRecord, error) {
	if limit <= 0 {
		limit = -1
	}
	var (
		rows *sql.Rows
		err  error
	)
	if tenantID == "" {
		rows, err = g.db.QueryContext(ctx,
			`SELECT key_id, tenant_id, name, hash, scopes, status,
			        created_at, expires_at, revoked_at
			 FROM api_keys ORDER BY created_at DESC LIMIT ?`,
			limit,
		)
	} else {
		rows, err = g.db.QueryContext(ctx,
			`SELECT key_id, tenant_id, name, hash, scopes, status,
			        created_at, expires_at, revoked_at
			 FROM api_keys WHERE tenant_id = ?
			 ORDER BY created_at DESC LIMIT ?`,
			tenantID, limit,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("globalstore: list api keys (tenant=%q): %w", tenantID, err)
	}
	defer rows.Close()

	out := []*APIKeyRecord{}
	for rows.Next() {
		rec, scanErr := scanAPIKeyRows(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("globalstore: scan api key row: %w", scanErr)
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("globalstore: iterate api key rows: %w", err)
	}
	return out, nil
}

// ListActiveAPIKeys returns the rows the auth layer should accept right
// now: status='active' AND not past expiry. This is the set the rotation
// migration window relies on -- old + new keys both come back while both
// are active, so a client cutover never sees a gap. now is the caller's
// notion of "current second"; pass 0 to use the store clock.
func (g *GlobalStore) ListActiveAPIKeys(ctx context.Context, now int64) ([]*APIKeyRecord, error) {
	if now == 0 {
		now = g.now()
	}
	rows, err := g.db.QueryContext(ctx,
		`SELECT key_id, tenant_id, name, hash, scopes, status,
		        created_at, expires_at, revoked_at
		 FROM api_keys
		 WHERE status = ?
		   AND (expires_at = 0 OR expires_at > ?)
		 ORDER BY created_at DESC`,
		APIKeyStatusActive, now,
	)
	if err != nil {
		return nil, fmt.Errorf("globalstore: list active api keys: %w", err)
	}
	defer rows.Close()

	out := []*APIKeyRecord{}
	for rows.Next() {
		rec, scanErr := scanAPIKeyRows(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("globalstore: scan api key row: %w", scanErr)
		}
		out = append(out, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("globalstore: iterate active api key rows: %w", err)
	}
	return out, nil
}

// RevokeAPIKey flips a key to status='revoked' and stamps revoked_at.
// Returns true iff a still-active row was flipped (idempotent: revoking
// an already-revoked or unknown key returns false, nil). This is the
// "retire the old key" half of the rotation migration window.
func (g *GlobalStore) RevokeAPIKey(ctx context.Context, keyID string) (bool, error) {
	res, err := g.db.ExecContext(ctx,
		`UPDATE api_keys
		 SET status = ?, revoked_at = ?
		 WHERE key_id = ? AND status = ?`,
		APIKeyStatusRevoked, g.now(), keyID, APIKeyStatusActive,
	)
	if err != nil {
		return false, fmt.Errorf("globalstore: revoke api key %q: %w", keyID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("globalstore: rows affected: %w", err)
	}
	return n > 0, nil
}

// DeleteAPIKey hard-deletes a key row. Used by GDPR / ops cleanup; the
// normal retire path is RevokeAPIKey (keeps the audit trail). Returns
// true iff a row was removed.
func (g *GlobalStore) DeleteAPIKey(ctx context.Context, keyID string) (bool, error) {
	res, err := g.db.ExecContext(ctx,
		`DELETE FROM api_keys WHERE key_id = ?`, keyID,
	)
	if err != nil {
		return false, fmt.Errorf("globalstore: delete api key %q: %w", keyID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("globalstore: rows affected: %w", err)
	}
	return n > 0, nil
}

// scanAPIKeyRow scans a single *sql.Row into an APIKeyRecord.
func scanAPIKeyRow(row *sql.Row) (*APIKeyRecord, error) {
	var (
		rec    APIKeyRecord
		scopes string
	)
	if err := row.Scan(
		&rec.KeyID, &rec.TenantID, &rec.Name, &rec.Hash, &scopes,
		&rec.Status, &rec.CreatedAt, &rec.ExpiresAt, &rec.RevokedAt,
	); err != nil {
		return nil, err
	}
	rec.Scopes = decodeScopes(scopes)
	return &rec, nil
}

// scanAPIKeyRows scans the current *sql.Rows cursor row.
func scanAPIKeyRows(rows *sql.Rows) (*APIKeyRecord, error) {
	var (
		rec    APIKeyRecord
		scopes string
	)
	if err := rows.Scan(
		&rec.KeyID, &rec.TenantID, &rec.Name, &rec.Hash, &scopes,
		&rec.Status, &rec.CreatedAt, &rec.ExpiresAt, &rec.RevokedAt,
	); err != nil {
		return nil, err
	}
	rec.Scopes = decodeScopes(scopes)
	return &rec, nil
}
