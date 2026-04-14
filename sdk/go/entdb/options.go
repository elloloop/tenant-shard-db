package entdb

import "time"

// clientConfig holds all client configuration values.
type clientConfig struct {
	secure     bool
	apiKey     string
	maxRetries int
	timeout    time.Duration
}

// defaultConfig returns a clientConfig with sensible defaults.
func defaultConfig() clientConfig {
	return clientConfig{
		secure:     false,
		apiKey:     "",
		maxRetries: 3,
		timeout:    30 * time.Second,
	}
}

// ClientOption is a functional option for configuring DbClient.
type ClientOption func(*clientConfig)

// WithSecure enables TLS for the gRPC connection.
func WithSecure() ClientOption {
	return func(c *clientConfig) {
		c.secure = true
	}
}

// WithAPIKey sets the API key used for authentication.
func WithAPIKey(key string) ClientOption {
	return func(c *clientConfig) {
		c.apiKey = key
	}
}

// WithMaxRetries sets the maximum number of retry attempts for failed RPCs.
func WithMaxRetries(n int) ClientOption {
	return func(c *clientConfig) {
		if n >= 0 {
			c.maxRetries = n
		}
	}
}

// WithTimeout sets the default timeout for RPCs.
func WithTimeout(d time.Duration) ClientOption {
	return func(c *clientConfig) {
		c.timeout = d
	}
}

// ── Plan.Create options (2026-04-14 SDK v0.3 decision) ─────────────

// createConfig collects the per-op knobs that previously lived on
// parallel *WithACL / *InMailbox / *InPublic methods. Populated by the
// functional options passed to [Plan.Create].
type createConfig struct {
	acl          []ACLEntry
	storage      StorageMode
	targetUserID string
	alias        string
}

// CreateOption configures a single Plan.Create call.
//
// Options are the single-shape replacement for the old parallel
// CreateWithACL / CreateInMailbox / CreateInPublic methods. There is
// exactly one Create method; everything else is an option.
type CreateOption func(*createConfig)

// WithACL attaches explicit ACL entries to the new node.
func WithACL(acl ...ACLEntry) CreateOption {
	return func(c *createConfig) { c.acl = acl }
}

// InTenant stores the new node in the tenant's tenant.db (the
// default). Included for symmetry with [InMailbox] / [InPublic] —
// calling sites that explicitly name the storage mode read more
// clearly than ones that rely on the zero value.
func InTenant() CreateOption {
	return func(c *createConfig) { c.storage = StorageModeTenant }
}

// InMailbox stores the new node in the target user's private mailbox
// database. userID is the owning user (e.g. "alice", not "user:alice"
// — the SDK handles the wire translation).
//
// Storage mode is IMMUTABLE: a node created in a mailbox can never be
// moved to tenant.db or public.db. See docs/decisions/storage.md.
func InMailbox(userID string) CreateOption {
	return func(c *createConfig) {
		c.storage = StorageModeUserMailbox
		c.targetUserID = userID
	}
}

// InPublic stores the new node in the singleton public.db.
//
// Storage mode is IMMUTABLE: a node created in public can never be
// moved into a tenant or mailbox file.
func InPublic() CreateOption {
	return func(c *createConfig) { c.storage = StorageModePublic }
}

// As sets the alias used for this node when referenced by edges
// later in the same plan. If omitted, [Plan.Create] generates a
// unique alias automatically.
func As(alias string) CreateOption {
	return func(c *createConfig) { c.alias = alias }
}

// ── Scope.Query options ─────────────────────────────────────────────

type queryConfig struct {
	limit  int32
	offset int32
}

// QueryOption configures a single [Query] call. Placeholder for
// future pagination / ordering options — declared here so the
// free-function signature in scope.go doesn't churn every time a new
// knob lands.
type QueryOption func(*queryConfig)

// WithLimit caps the number of results returned.
func WithLimit(n int32) QueryOption {
	return func(c *queryConfig) { c.limit = n }
}

// WithOffset skips the first N results.
func WithOffset(n int32) QueryOption {
	return func(c *queryConfig) { c.offset = n }
}
