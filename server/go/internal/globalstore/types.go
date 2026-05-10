// Domain types and helpers shared across the globalstore package.
//
// We use plain Go structs rather than proto messages because the
// generated `entdb.v1.TenantInfo` only carries `tenant_id` — it cannot
// represent the full registry row (status, created_at, region). The
// gRPC handlers translate these structs to proto at the boundary.

package globalstore

import (
	"errors"
	"strings"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// User is one row of user_registry.
type User struct {
	UserID    string
	Email     string
	Name      string
	Status    string
	CreatedAt int64
	UpdatedAt int64
}

// Tenant is one row of tenant_registry.
type Tenant struct {
	TenantID  string
	Name      string
	Status    string
	CreatedAt int64
	Region    string
}

// Member is one row of tenant_members.
type Member struct {
	TenantID string
	UserID   string
	Role     string
	JoinedAt int64
}

// SharedEntry is one row of shared_index. The spec calls this a "hint"
// — authoritative ACLs live in the per-tenant store.
type SharedEntry struct {
	UserID       string
	SourceTenant string
	NodeID       string
	Permission   string
	SharedAt     int64
}

// DeletionEntry is one row of deletion_queue.
type DeletionEntry struct {
	UserID      string
	RequestedAt int64
	ExecuteAt   int64
	ExportPath  string // empty == NULL
	Status      string
}

// LegalHold is one row of legal_holds.
type LegalHold struct {
	TenantID  string
	HeldBy    string
	Reason    string
	CreatedAt int64
}

// QuotaConfig is one row of tenant_quotas.
type QuotaConfig struct {
	TenantID               string
	MaxWritesPerMonth      int64
	HardEnforce            bool
	MaxRPSSustained        int64
	MaxRPSBurst            int64
	MaxRPSPerUserSustained int64
	MaxRPSPerUserBurst     int64
	UpdatedAt              int64
}

// Usage is one row of tenant_usage.
type Usage struct {
	TenantID      string
	PeriodStartMs int64
	WritesCount   int64
	UpdatedAt     int64
}

// TransferResult is the outcome of TransferUserContent.
type TransferResult struct {
	TenantID          string
	FromUser          string
	ToUser            string
	MembershipCreated bool
}

// RevokeResult is the outcome of RevokeUserAccess.
type RevokeResult struct {
	TenantID          string
	UserID            string
	MembershipRemoved bool
	SharedRemoved     int64
}

// isUniqueViolation reports whether err is a SQLite UNIQUE-constraint
// failure (PRIMARY KEY collisions count). modernc.org/sqlite raises a
// *sqlite.Error with a SQLITE_CONSTRAINT_* extended code; the message
// "UNIQUE constraint failed: ..." is the user-visible signal.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var sqErr *sqlite.Error
	if errors.As(err, &sqErr) {
		switch sqErr.Code() {
		case sqlite3.SQLITE_CONSTRAINT_UNIQUE,
			sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY:
			return true
		}
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}
