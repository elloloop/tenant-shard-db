// SPDX-License-Identifier: MIT
package entdb

import (
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

// clientConfig holds all client configuration values.
type clientConfig struct {
	secure       bool
	apiKey       string
	maxRetries   int
	timeout      time.Duration
	nodeResolver NodeResolver
	// schema is the client-side, NAME-FREE schema registry built from the
	// proto message types passed to [WithSchema]. When set, the SDK rides a
	// SchemaDescriptor on ExecuteAtomic until the server confirms the
	// matching fingerprint (SELF-DESCRIBING WRITES, ADR-031). nil leaves
	// the server schema-less for this client (the issue #545 path).
	schema *schemaRegistry
	// retryBudget bounds the total wall-clock time spent retrying a
	// single call (backoff sleeps included). Zero means use
	// defaultRetryWallClockBudget.
	retryBudget time.Duration
	// retryJitter is the randomness source for the transient-retry
	// backoff. Left nil in production (a clock-seeded source is
	// created on Connect); in-package tests set it to a
	// deterministic source so backoff durations are reproducible.
	retryJitter jitterSource
	// dialOptions is appended to the grpc.NewClient option list on
	// every dial — both the primary endpoint and any redirect
	// sub-channels. Tests use it to install a contextDialer for
	// bufconn-backed in-process servers.
	dialOptions []grpc.DialOption
	// unaryClientInterceptors are caller-supplied interceptors that
	// run before SDK-owned interceptors such as redirect handling.
	unaryClientInterceptors []grpc.UnaryClientInterceptor
	// streamClientInterceptors are caller-supplied stream interceptors.
	streamClientInterceptors []grpc.StreamClientInterceptor
	// disableTracePropagation turns off the SDK-owned interceptor that
	// injects the caller's active W3C trace context (traceparent) into
	// outgoing gRPC metadata. Propagation is ON by default and is a
	// no-op when the caller has no active OpenTelemetry span. ADR-033 §6.
	disableTracePropagation bool
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

// WithRetryBudget caps the total wall-clock time the SDK spends
// retrying a single failed RPC, including the exponential-backoff
// sleeps between attempts. Once the elapsed time since the first
// attempt would exceed the budget, the last error is returned even if
// retry attempts (per [WithMaxRetries]) remain.
//
// This bounds tail latency under an outage: without it, a high
// maxRetries combined with exponential backoff can block a caller for
// minutes. A non-positive duration restores the 30s default.
func WithRetryBudget(d time.Duration) ClientOption {
	return func(c *clientConfig) {
		if d > 0 {
			c.retryBudget = d
		}
	}
}

// WithSchema registers the client's schema from its proto message types so
// writes are self-describing (SELF-DESCRIBING WRITES, ADR-031).
//
// Pass a zero value of every node/edge message type your app writes (the
// SDK reads only their descriptors — (entdb.node)/(entdb.edge)/(entdb.field)
// options — never the instances):
//
//	client, _ := entdb.NewClient(addr,
//	    entdb.WithSchema(&shop.Product{}, &shop.User{}, &shop.PurchaseEdge{}))
//
// On the first ExecuteAtomic the SDK attaches a NAME-FREE SchemaDescriptor
// and the schema fingerprint; the server materializes the types via a
// leading register_schema WAL op (establish-or-reject) before the data ops.
// Once the server confirms the matching fingerprint the descriptor is
// omitted (lean steady state) and re-attached on a SCHEMA_MISMATCH.
//
// Messages without an (entdb.node)/(entdb.edge) option are ignored. Omitting
// WithSchema leaves the client schema-less: writes carry no descriptor and
// the server enforces nothing (the issue #545 numeric-field-id path still
// works).
func WithSchema(msgs ...proto.Message) ClientOption {
	return func(c *clientConfig) {
		c.schema = newSchemaRegistry(msgs)
	}
}

// WithNodeResolver installs a custom [NodeResolver] used to map
// server-issued “node_id“ redirect hints to dial-able endpoints.
//
// Use this with [StaticMapResolver] for deployments without DNS,
// or with a custom implementation backed by service discovery.
//
// If both [WithNodeResolver] and [WithBaseDomain] are supplied,
// the last one wins.
func WithNodeResolver(r NodeResolver) ClientOption {
	return func(c *clientConfig) { c.nodeResolver = r }
}

// WithBaseDomain wires a [DNSTemplateResolver] for the given base
// domain. Sub-channels are dialed at “<node_id>.<baseDomain>:50051“
// when the server returns a redirect hint via the
// “entdb-redirect-node“ trailing metadata header.
//
// This is the recommended way to configure node redirection on
// Kubernetes — point “baseDomain“ at the headless service and
// the Pod-per-shard StatefulSet will be reachable via the
// per-pod DNS record.
//
//	client, _ := entdb.NewClient("entdb.svc.cluster.local:50051",
//	    entdb.WithBaseDomain("entdb.svc.cluster.local"))
func WithBaseDomain(baseDomain string) ClientOption {
	return func(c *clientConfig) {
		c.nodeResolver = &DNSTemplateResolver{BaseDomain: baseDomain}
	}
}

// WithoutTracePropagation disables the SDK-owned interceptor that
// injects the caller's active W3C trace context (traceparent) into
// outgoing gRPC metadata. Propagation is ON by default and is a no-op
// when there is no active OpenTelemetry span, so this is only needed to
// suppress propagation entirely. See ADR-033 §6.
func WithoutTracePropagation() ClientOption {
	return func(c *clientConfig) {
		c.disableTracePropagation = true
	}
}

// WithUnaryClientInterceptors prepends caller-supplied unary gRPC
// client interceptors to the SDK's internal interceptor chain.
//
// This is intended for cross-cutting concerns such as OpenTelemetry
// tracing, metrics, request correlation, or custom propagation. When
// SDK-owned interceptors are also installed, for example node redirect
// handling, these interceptors run first and wrap the full outbound RPC.
func WithUnaryClientInterceptors(interceptors ...grpc.UnaryClientInterceptor) ClientOption {
	return func(c *clientConfig) {
		for _, interceptor := range interceptors {
			if interceptor != nil {
				c.unaryClientInterceptors = append(c.unaryClientInterceptors, interceptor)
			}
		}
	}
}

// WithStreamClientInterceptors prepends caller-supplied stream gRPC
// client interceptors to the SDK's internal interceptor chain.
//
// The current EntDB wire API is unary-only, but exposing the stream hook
// keeps the SDK option surface aligned with grpc-go and future-proofs
// streaming RPC additions.
func WithStreamClientInterceptors(interceptors ...grpc.StreamClientInterceptor) ClientOption {
	return func(c *clientConfig) {
		for _, interceptor := range interceptors {
			if interceptor != nil {
				c.streamClientInterceptors = append(c.streamClientInterceptors, interceptor)
			}
		}
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
	id           string
	onConflict   NodeConflictPolicy
}

// NodeConflictPolicy is the wire-aligned policy carried on
// CreateNodeOp for the v2.2 single-RTT InsertIfNotExists path (issue
// #599). Default ConflictError mirrors the pre-v2.2 behaviour — a
// tripped unique index aborts the batch and surfaces a typed
// *UniqueConstraintError.
type NodeConflictPolicy int32

const (
	// ConflictError aborts the batch on a unique violation. Default.
	ConflictError NodeConflictPolicy = 0
	// ConflictSkip swallows the violation server-side and returns the
	// pre-existing row's id in CommitResult.ExistingNodeIDs at the
	// op's index. Powers InsertIfNotExists's single-RTT path.
	ConflictSkip NodeConflictPolicy = 1
)

// OnConflict sets the unique-violation policy for a create. v2.2 /
// issue #599. Pre-v2.2 servers ignore the option and surface the
// legacy *UniqueConstraintError; the SDK helpers using SKIP detect
// that and fall back to the v2.1.x two-RTT GetNodeByKey lookup so
// the same SDK release works against either server version.
func OnConflict(p NodeConflictPolicy) CreateOption {
	return func(c *createConfig) { c.onConflict = p }
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
// moved to tenant.db or public.db. See
// docs/adr/020-immutable-storage-mode.md.
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

// WithID requests the server use the supplied id when creating the
// node, rather than generating a fresh UUID. Useful for idempotent
// upserts where the caller derives the id from external content
// (e.g. “uuid.NewSHA1(ns, []byte(slug))“).
//
// The id must be a valid UUID string; validation happens server-side
// — malformed ids surface as INVALID_ARGUMENT and duplicate ids in
// the same tenant + type scope surface as ALREADY_EXISTS, which the
// SDK reports as [*UniqueConstraintError]. Clients building
// idempotent flows should treat ALREADY_EXISTS as "use Get instead".
func WithID(id string) CreateOption {
	return func(c *createConfig) { c.id = id }
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

// ── Admin.CreateTenant options ─────────────────────────────────────

// createTenantConfig holds optional parameters for
// [Admin.CreateTenant].
type createTenantConfig struct {
	region string
}

// CreateTenantOption configures a single [Admin.CreateTenant] call.
type CreateTenantOption func(*createTenantConfig)

// WithRegion pins the new tenant to a specific geographic region
// (e.g. “"us-east-1"“, “"eu-west-1"“). Once set, every request
// that touches the tenant must hit a server that serves the same
// region; cross-region calls are rejected with FAILED_PRECONDITION.
//
// When this option is omitted the server defaults the tenant's
// region to its own served region, so single-region deployments do
// not need to set it.
func WithRegion(region string) CreateTenantOption {
	return func(c *createTenantConfig) { c.region = region }
}
