<!-- GENERATED FILE — do not edit by hand.
     Regenerate with: python scripts/generate_api_docs.py
     Source of truth is the proto + SDK code, not this file. -->

# Go SDK Reference (generated)

Module `github.com/elloloop/tenant-shard-db/sdk/go/entdb`. Canonical rendered docs: <https://pkg.go.dev/github.com/elloloop/tenant-shard-db/sdk/go/entdb>.

```text
package entdb // import "github.com/elloloop/tenant-shard-db/sdk/go/entdb"

SPDX-License-Identifier: MIT

SPDX-License-Identifier: MIT

SPDX-License-Identifier: MIT Package entdb provides a Go client SDK for the
EntDB multi-tenant graph database.

SPDX-License-Identifier: MIT

SPDX-License-Identifier: MIT

SPDX-License-Identifier: MIT

SPDX-License-Identifier: MIT

SPDX-License-Identifier: MIT

SPDX-License-Identifier: MIT

SPDX-License-Identifier: MIT

SPDX-License-Identifier: MIT

SPDX-License-Identifier: MIT

SPDX-License-Identifier: MIT

SPDX-License-Identifier: MIT

SPDX-License-Identifier: MIT

SPDX-License-Identifier: MIT

VARIABLES

var ErrPreconditionFailed = errors.New("entdb: precondition failed")
    ErrPreconditionFailed is the sentinel error returned when a conditional
    UpdateNode op's precondition is not met. Match with “errors.Is(err,
    entdb.ErrPreconditionFailed)“. See PreconditionFailure for the typed wrapper
    that carries the failure coordinates (op index, field, expected, observed).
    GitHub issue #500.


FUNCTIONS

func Delete[T proto.Message](p *Plan, nodeID string)
    Delete accumulates a delete-node operation.

    Delete is a free function rather than a method on Plan because Go does not
    support generic methods. The type parameter T is a compile-time witness —
    the SDK reads the “(entdb.node).type_id“ from T's descriptor without needing
    a message instance:

        entdb.Delete[*shop.Product](plan, "node-42")

    Passing a non-pointer or non-proto T is a compile-time error; passing a T
    with no “(entdb.node)“ option panics at call time.

func DeleteWhere[T proto.Message](p *Plan, where []Filter, limit int)
    DeleteWhere accumulates a single-RPC predicate-based sweeper op (GitHub
    issue #504). It deletes every node of type T whose payload matches ALL of
    the AND-ed Filter predicates, capped best-effort by limit, in one round trip
    — collapsing the QueryWhere + Delete loop the TTL-sweeper pattern otherwise
    needs.

    Like Delete, DeleteWhere is a free function (Go has no generic methods)
    and T is a compile-time type witness: the SDK reads "(entdb.node).type_id"
    from T's descriptor without a message instance. The Filter field names
    are resolved to payload field ids server-side, exactly like QueryWhere,
    so no client registry is needed for the predicate.

    limit is best-effort (Postgres DELETE … LIMIT semantics): at most limit
    nodes are deleted by this op. Pass 0 for the server default; the server
    clamps to its own hard ceiling so a runaway predicate cannot pin the single
    applier goroutine for a tenant. To drain a large backlog, sweep in a loop
    until a sweep deletes nothing.

    At least one filter is required — an unconditional bulk delete is rejected
    server-side. Passing T with no "(entdb.node)" option panics at call time.

        entdb.DeleteWhere[*auth.WebAuthnChallenge](plan,
            []entdb.Filter{{Field: "expires_at", Op: entdb.FilterLt, Value: now}},
            1000,
        )

    Schema-less servers (issue #545): a server started without a schema cannot
    resolve a field NAME to a payload field id. Pass the numeric payload field
    id as the Filter.Field string instead — exactly the schema-optional escape
    hatch QueryWhere already accepts for numeric filter keys. The SDK forwards
    Field verbatim; the server treats a digit-only key as a raw field id with no
    schema lookup:

        // "expires_at" is proto field 4 on the caller's own schema.
        entdb.DeleteWhere[*auth.WebAuthnChallenge](plan,
            []entdb.Filter{{Field: "4", Op: entdb.FilterLt, Value: now}},
            1000,
        )

    Against a schema-configured server both forms work; against a schema-less
    server only the numeric-id form does (a name key returns INVALID_ARGUMENT:
    "cannot translate filter key … without a schema").

func EdgeCreate[T proto.Message](p *Plan, from, to string)
    EdgeCreate accumulates a create-edge operation.

    The type parameter T is an edge proto message type (annotated with
    “(entdb.edge)“), used purely as a compile-time witness for the edge_id:

        entdb.EdgeCreate[*shop.PurchaseEdge](plan, "user-1", "product-42")

func EdgeDelete[T proto.Message](p *Plan, from, to string)
    EdgeDelete accumulates a delete-edge operation. See EdgeCreate for the
    type-witness pattern.

func ExtractSchemaJSON(fds *descriptorpb.FileDescriptorSet) ([]byte, error)
    ExtractSchemaJSON reads an EntDB schema out of a compiled protobuf
    descriptor set and returns the JSON bytes the EntDB server expects in
    “SCHEMA_FILE“.

    The descriptor set is what “protoc --descriptor_set_out=FILE
    --include_imports“ produces — the binary “descriptorpb.FileDescriptorSet“
    form. Every message annotated with “(entdb.node)“ or “(entdb.edge)“
    becomes a node or edge type in the output JSON. Every field on a
    node or edge gets its kind inferred from the proto field type (with
    the “(entdb.field).kind“ string override winning when set), and the
    “(entdb.field).{required,indexed,searchable,unique}“ flags flow through.

    Output is the same wrapped envelope produced by “entdb-schema snapshot“
    (Python) — “{"version": 1, "fingerprint": "...", "schema": {...}}“ — so the
    same server boot path (“_load_schema_file“) consumes both.

    Errors come back when the descriptor set is malformed or when a node has a
    missing required field id (“(entdb.node).type_id == 0“). Messages with no
    “(entdb.node)“ / “(entdb.edge)“ option are silently skipped — same as the
    Python “register_proto_schema“ behaviour.

func Get[T proto.Message](ctx context.Context, s *Scope, nodeID string) (T, error)
    Get fetches a node by id.

    T is the proto message type; the SDK reads the “(entdb.node).type_id“ from
    its descriptor:

        product, err := entdb.Get[*shop.Product](ctx, scope, "node-42")

func GetMany[T proto.Message](ctx context.Context, s *Scope, nodeIDs []string) ([]T, []string, error)
    GetMany batch-fetches nodes by id. Returns the list of unmarshaled messages
    plus the list of ids the server reported as missing.

        products, missing, err := entdb.GetMany[*shop.Product](
            ctx, scope, []string{"node-1", "node-2", "node-3"})

    Order is not guaranteed; callers needing input-order alignment should index
    by “Node.NodeID“.

func Query[T proto.Message](ctx context.Context, s *Scope, filter map[string]any, opts ...QueryOption) ([]T, error)
    Query fetches nodes by filter.

    T determines the type_id and the result type. The filter map accepts
    MongoDB-style operators described in the 2026-04-14 SDK v0.3 decision
    (sdk_api.md):

        $eq (default), $ne, $gt, $gte, $lt, $lte, $in, $nin, $like,
        $between, plus top-level $and / $or.

    Field names are proto field names; the server resolves them to field ids at
    the ingress boundary.

    For the typed-Filter form preferred by issue #501 see QueryWhere.

func QueryWhere[T proto.Message](ctx context.Context, s *Scope, filters []Filter, opts ...QueryOption) ([]T, error)
    QueryWhere is the typed counterpart of Query: callers pass a slice of Filter
    values rather than a MongoDB-style map. The two helpers share the same
    transport, so the wire result is identical.

    All filters are AND-ed. Issue #501 (entdb v1) scopes the operator set to
    Eq/Ne/Lt/Le/Gt/Ge — there is no OR, no nesting, no sorting, no cursor.
    Pagination is via WithLimit / WithOffset.

    Example sweeper pattern:

        expired, err := entdb.QueryWhere[*authn.Challenge](ctx, scope,
            []entdb.Filter{
                {Field: "expires_at", Op: entdb.FilterLt, Value: nowMs},
            },
            entdb.WithLimit(500))

    Index usage caveat: FilterNe forces a full type scan because a B-tree
    expression index cannot serve a not-equal predicate. Prefer a positive
    predicate for sweeper-style hot paths.

func Search[T proto.Message](ctx context.Context, s *Scope, query string, opts ...QueryOption) ([]T, error)
    Search performs full-text search across searchable fields of a node type.

    T determines the type_id and the result type. The query string is an FTS5
    match expression supporting AND, OR, NOT, phrase ("..."), and prefix (word*)
    syntax. Only fields declared with (entdb.field).searchable = true are
    searched.

    ADR-029 FTS carve-out: search is relevance-ranked top-N and is OFFSET-paged,
    NOT cursor-paged — it does NOT auto-follow to completion the way Query /
    Scope.EdgesFrom / Scope.SharedWithMe do. Use WithLimit to set the page
    size (alias for page_size) and WithOffset to advance within the ranked
    result set. To learn whether more ranked rows exist beyond the page,
    use SearchPage.

        results, err := entdb.Search[*shop.Product](ctx, scope, "widget",
            entdb.WithLimit(20), entdb.WithOffset(20))

func SearchPage[T proto.Message](ctx context.Context, s *Scope, query string, opts ...QueryOption) ([]T, bool, error)
    SearchPage is Search that also reports has_more — true when the ranked
    result set has rows beyond this page. Page with WithOffset to fetch the next
    slice. There is no cursor for search (ADR-029 FTS carve-out).

func WithAfterOffset(ctx context.Context, streamPosition string) context.Context
    WithAfterOffset returns a context that pins the next read to the given
    WAL stream position, overriding automatic offset tracking for that call.
    Equivalent to passing an explicit “after_offset“ string in the Python SDK.

        ctx = entdb.WithAfterOffset(ctx, receipt.StreamPosition)
        p, err := entdb.Get[*shop.Product](ctx, scope, "node-42")

func WithoutOffsetTracking(ctx context.Context) context.Context
    WithoutOffsetTracking returns a context that opts the next read out of
    automatic offset tracking — no “after_offset“ is sent even if the client has
    written for this tenant. Equivalent to passing “after_offset=None“ in the
    Python SDK.


TYPES

type ACLEntry struct {
	Grantee    Actor      `json:"grantee"`
	Permission Permission `json:"permission"`
	ExpiresAt  int64      `json:"expires_at,omitempty"`
}
    ACLEntry represents an access control list entry.

type AccessDeniedError struct {
	EntDBError
	Actor              string
	ResourceID         string
	RequiredPermission string
}
    AccessDeniedError indicates an authorization failure.

func NewAccessDeniedError(message, actor, resourceID, requiredPermission string) *AccessDeniedError
    NewAccessDeniedError creates a new AccessDeniedError.

type Actor struct {
	// Has unexported fields.
}
    Actor is a typed principal identifier that replaces raw "user:bob" strings.
    Use the constructor functions UserActor, GroupActor, ServiceActor.

func GroupActor(id string) Actor
    GroupActor creates an Actor for a group principal.

func ParseActor(s string) (Actor, error)
    ParseActor parses a "kind:id" string into an Actor.

func ServiceActor(id string) Actor
    ServiceActor creates an Actor for a service principal.

func UserActor(id string) Actor
    UserActor creates an Actor for a user principal.

func (a Actor) ID() string
    ID returns the actor identifier.

func (a Actor) Kind() string
    Kind returns the actor kind (user, group, service).

func (a Actor) String() string
    String returns the "kind:id" wire format.

type Admin struct {
	// Has unexported fields.
}
    Admin groups EntDB's cross-tenant identity operations: creating tenants
    and users in the global registry, managing memberships, and bulk ACL admin
    (transfer, delegate, revoke).

    These calls cross tenant boundaries — they take “actor“ as a raw string
    (typically “"system:admin"“ or a tenant owner/admin actor) rather than a
    scoped Actor. They live on “DbClient“ rather than “Scope“ because most of
    them either operate above any single tenant (CreateTenant, CreateUser,
    GetUserTenants) or want the raw “user_id“ form to match the global
    registry's identifier shape.

    Get a handle via DbClient.Admin:

        admin := client.Admin()
        tenant, err := admin.CreateTenant(ctx, "system:admin", "acme", "Acme Corp")
        user, err := admin.CreateUser(ctx, "system:admin", "alice", "alice@acme.test", "Alice")
        err = admin.AddTenantMember(ctx, "system:admin", "acme", "alice", "member")

    Like the rest of the SDK, every admin write goes through the WAL — they are
    auditable and replayable, not direct SQLite mutations.

func (a *Admin) AddTenantMember(ctx context.Context, actor, tenantID, userID, role string) error
    AddTenantMember adds “userID“ to “tenantID“ with the given “role“ (“"owner"“
    / “"admin"“ / “"member"“ / “"viewer"“). Both the user and tenant must exist.

func (a *Admin) CancelUserDeletion(ctx context.Context, actor, userID string) error
    CancelUserDeletion lifts a pending deletion request. Idempotent — calling
    it on a user with no pending deletion returns success with no observable
    change.

func (a *Admin) ChangeMemberRole(ctx context.Context, actor, tenantID, userID, newRole string) error
    ChangeMemberRole promotes or demotes a member. The server treats roles as
    opaque strings — capability mappings are configured on the server side,
    not enforced here.

func (a *Admin) CreateTenant(ctx context.Context, actor, tenantID, name string, opts ...CreateTenantOption) (*TenantDetail, error)
    CreateTenant registers a new tenant in the global registry. The new tenant
    has no members; call Admin.AddTenantMember to invite users. “actor“ must be
    “"system:admin"“ (or another principal with the admin capability).

    Re-creating a tenant that already exists raises an underlying uniqueness
    error from the global SQLite — there is no "create or get" form by design.

    Pass WithRegion to pin the tenant to a specific geographic region (e.g.
    “WithRegion("eu-west-1")“); when omitted the server defaults the
    tenant to its own served region. The pinned region is reflected on
    [TenantDetail.Region].

func (a *Admin) CreateUser(ctx context.Context, actor, userID, email, name string) (*UserInfo, error)
    CreateUser registers a new user in the global registry. “email“ is unique
    across all tenants — re-using an email raises an AlreadyExists error from
    the server.

func (a *Admin) DelegateAccess(ctx context.Context, actor, tenantID, fromUser, toUser, permission string, expiresAtMs int64) (*DelegateResult, error)
    DelegateAccess grants “toUser“ the listed “permission“ on every node
    “fromUser“ owns in “tenantID“ — useful for vacation cover, audit reviewers,
    and short-lived support sessions. Pass “expiresAtMs = 0“ for a permanent
    delegation; otherwise the grant becomes invisible to the ACL check after
    that absolute epoch-millis timestamp (no background sweep needed — the check
    evaluates the timestamp at read time).

func (a *Admin) DeleteUser(ctx context.Context, actor, userID string, graceDays int32) (*DeletionScheduled, error)
    DeleteUser schedules deletion of “userID“. The deletion is not immediate
    — the GDPR worker executes it after a grace period (default 30 days;
    pass “graceDays > 0“ to override). The user enters “"deletion_pending"“
    status right away; reads continue to work until the grace expires.
    Use Admin.CancelUserDeletion to lift the schedule before then.

    Returns the absolute requested-at and execute-at timestamps so you can show
    "your data will be removed at <date>" UI without a follow-up read.

func (a *Admin) ExportUserData(ctx context.Context, actor, userID string) (string, error)
    ExportUserData returns the GDPR Article 20 portability bundle — a
    JSON-serialized export of every node the user owns across all tenants.
    The exact JSON shape is documented server-side; treat it as opaque from the
    SDK's perspective.

func (a *Admin) FreezeUser(ctx context.Context, actor, userID string, enabled bool) (string, error)
    FreezeUser flips “userID“'s status. With “enabled = true“ the user is frozen
    — their authentication continues to succeed but every read or write is
    rejected until they are unfrozen. Their data is preserved (no deletion clock
    starts). Pass “enabled = false“ to lift the freeze.

    Returns the new status string (typically “"frozen"“ or “"active"“).

func (a *Admin) GetTenantMembers(ctx context.Context, actor, tenantID string) ([]TenantMember, error)
    GetTenantMembers returns every membership row for “tenantID“ — who is in
    this tenant, with what role.

func (a *Admin) GetUserTenants(ctx context.Context, actor, userID string) ([]TenantMember, error)
    GetUserTenants returns every tenant the user is a member of — where this
    user has access, with what role.

func (a *Admin) RemoveTenantMember(ctx context.Context, actor, tenantID, userID string) error
    RemoveTenantMember drops a membership row. It does NOT reassign
    nodes the user owns or revoke their direct ACL grants — for that see
    Admin.TransferUserContent and Admin.RevokeAllUserAccess. Combine all three
    for a complete off-board.

func (a *Admin) RevokeAllUserAccess(ctx context.Context, actor, tenantID, userID string) (*RevokeAllResult, error)
    RevokeAllUserAccess emergency-removes every direct grant, group membership,
    and shared-index entry the user has in “tenantID“, and drops their
    “tenant_members“ row. After this returns the user has zero visibility into
    the tenant. Does NOT delete or anonymize the user's owned nodes — for that,
    see Admin.TransferUserContent (offboarding) or the GDPR delete flow
    (right-to-erasure with grace period).

func (a *Admin) TransferUserContent(ctx context.Context, actor, tenantID, fromUser, toUser string) (int32, error)
    TransferUserContent reassigns every node owned by “fromUser“ in “tenantID“
    to “toUser“ — the classic offboarding flow. Server-side this also ensures
    “toUser“ is a member of the tenant, adding them as “"member"“ if not.
    It does NOT revoke “fromUser“'s direct ACL grants on other people's nodes —
    pair with Admin.RevokeAllUserAccess for a full off-board. Returns the count
    of transferred nodes.

type ClientOption func(*clientConfig)
    ClientOption is a functional option for configuring DbClient.

func WithAPIKey(key string) ClientOption
    WithAPIKey sets the API key used for authentication.

func WithBaseDomain(baseDomain string) ClientOption
    WithBaseDomain wires a DNSTemplateResolver for the given base domain.
    Sub-channels are dialed at “<node_id>.<baseDomain>:50051“ when the server
    returns a redirect hint via the “entdb-redirect-node“ trailing metadata
    header.

    This is the recommended way to configure node redirection on Kubernetes —
    point “baseDomain“ at the headless service and the Pod-per-shard StatefulSet
    will be reachable via the per-pod DNS record.

        client, _ := entdb.NewClient("entdb.svc.cluster.local:50051",
            entdb.WithBaseDomain("entdb.svc.cluster.local"))

func WithMaxRetries(n int) ClientOption
    WithMaxRetries sets the maximum number of retry attempts for failed RPCs.

func WithNodeResolver(r NodeResolver) ClientOption
    WithNodeResolver installs a custom NodeResolver used to map server-issued
    “node_id“ redirect hints to dial-able endpoints.

    Use this with StaticMapResolver for deployments without DNS, or with a
    custom implementation backed by service discovery.

    If both WithNodeResolver and WithBaseDomain are supplied, the last one wins.

func WithRetryBudget(d time.Duration) ClientOption
    WithRetryBudget caps the total wall-clock time the SDK spends retrying
    a single failed RPC, including the exponential-backoff sleeps between
    attempts. Once the elapsed time since the first attempt would exceed
    the budget, the last error is returned even if retry attempts (per
    WithMaxRetries) remain.

    This bounds tail latency under an outage: without it, a high maxRetries
    combined with exponential backoff can block a caller for minutes.
    A non-positive duration restores the 30s default.

func WithSecure() ClientOption
    WithSecure enables TLS for the gRPC connection.

func WithStreamClientInterceptors(interceptors ...grpc.StreamClientInterceptor) ClientOption
    WithStreamClientInterceptors prepends caller-supplied stream gRPC client
    interceptors to the SDK's internal interceptor chain.

    The current EntDB wire API is unary-only, but exposing the stream hook keeps
    the SDK option surface aligned with grpc-go and future-proofs streaming RPC
    additions.

func WithTimeout(d time.Duration) ClientOption
    WithTimeout sets the default timeout for RPCs.

func WithUnaryClientInterceptors(interceptors ...grpc.UnaryClientInterceptor) ClientOption
    WithUnaryClientInterceptors prepends caller-supplied unary gRPC client
    interceptors to the SDK's internal interceptor chain.

    This is intended for cross-cutting concerns such as OpenTelemetry tracing,
    metrics, request correlation, or custom propagation. When SDK-owned
    interceptors are also installed, for example node redirect handling,
    these interceptors run first and wrap the full outbound RPC.

type CommitResult struct {
	Success        bool     `json:"success"`
	Receipt        *Receipt `json:"receipt,omitempty"`
	CreatedNodeIDs []string `json:"created_node_ids,omitempty"`
	Applied        bool     `json:"applied"`
	Error          string   `json:"error,omitempty"`
}
    CommitResult is the result of committing a Plan.

type ConnectionError struct {
	EntDBError
	Address string
}
    ConnectionError indicates a failure to connect to the EntDB server.

func NewConnectionError(message, address string) *ConnectionError
    NewConnectionError creates a new ConnectionError.

type CreateOption func(*createConfig)
    CreateOption configures a single Plan.Create call.

    Options are the single-shape replacement for the old parallel CreateWithACL
    / CreateInMailbox / CreateInPublic methods. There is exactly one Create
    method; everything else is an option.

func As(alias string) CreateOption
    As sets the alias used for this node when referenced by edges later in the
    same plan. If omitted, Plan.Create generates a unique alias automatically.

func InMailbox(userID string) CreateOption
    InMailbox stores the new node in the target user's private mailbox database.
    userID is the owning user (e.g. "alice", not "user:alice" — the SDK handles
    the wire translation).

    Storage mode is IMMUTABLE: a node created in a mailbox can never be moved to
    tenant.db or public.db. See docs/adr/020-immutable-storage-mode.md.

func InPublic() CreateOption
    InPublic stores the new node in the singleton public.db.

    Storage mode is IMMUTABLE: a node created in public can never be moved into
    a tenant or mailbox file.

func InTenant() CreateOption
    InTenant stores the new node in the tenant's tenant.db (the default).
    Included for symmetry with InMailbox / InPublic — calling sites that
    explicitly name the storage mode read more clearly than ones that rely on
    the zero value.

func WithACL(acl ...ACLEntry) CreateOption
    WithACL attaches explicit ACL entries to the new node.

func WithID(id string) CreateOption
    WithID requests the server use the supplied id when creating the node,
    rather than generating a fresh UUID. Useful for idempotent upserts where
    the caller derives the id from external content (e.g. “uuid.NewSHA1(ns,
    []byte(slug))“).

    The id must be a valid UUID string; validation happens server-side —
    malformed ids surface as INVALID_ARGUMENT and duplicate ids in the same
    tenant + type scope surface as ALREADY_EXISTS, which the SDK reports as
    *UniqueConstraintError. Clients building idempotent flows should treat
    ALREADY_EXISTS as "use Get instead".

type CreateTenantOption func(*createTenantConfig)
    CreateTenantOption configures a single Admin.CreateTenant call.

func WithRegion(region string) CreateTenantOption
    WithRegion pins the new tenant to a specific geographic region (e.g.
    “"us-east-1"“, “"eu-west-1"“). Once set, every request that touches the
    tenant must hit a server that serves the same region; cross-region calls are
    rejected with FAILED_PRECONDITION.

    When this option is omitted the server defaults the tenant's region to its
    own served region, so single-region deployments do not need to set it.

type DNSTemplateResolver struct {
	// BaseDomain is the parent DNS zone — typically the headless
	// service domain in Kubernetes (e.g.
	// ``entdb.svc.cluster.local``).
	BaseDomain string
	// Port is the gRPC port on each node. Zero means 50051.
	Port int
}
    DNSTemplateResolver is the default NodeResolver. It composes the endpoint
    as “<nodeID>.<BaseDomain>:<Port>“. “Port“ defaults to 50051 (the EntDB gRPC
    port) when zero.

func (r *DNSTemplateResolver) Resolve(nodeID string) (string, error)
    Resolve composes “<nodeID>.<BaseDomain>:<Port>“.

type DbClient struct {
	// Has unexported fields.
}
    DbClient is the main entry point for the EntDB Go SDK.

    DbClient owns the gRPC transport and hands out TenantScope / Scope handles
    for typed access. It deliberately does NOT expose a flat “Get(tenantID,
    actor, typeID, nodeID)“ API — every user-facing read or write goes through
    a typed scope so that type_id and field_id only appear inside the SDK,
    never in user code.

        client, _ := entdb.NewClient("localhost:50051",
            entdb.WithSecure(), entdb.WithAPIKey("sk-..."))
        defer client.Close()
        _ = client.Connect(ctx)
        scope := client.Tenant("acme").Actor(entdb.UserActor("bob"))
        product, err := entdb.Get[*shop.Product](ctx, scope, "node-42")

func NewClient(address string, opts ...ClientOption) (*DbClient, error)
    NewClient creates a new DbClient targeting the given server address.

    The client is not connected until Connect is called.

func (c *DbClient) Address() string
    Address returns the server address this client targets.

func (c *DbClient) Admin() *Admin
    Admin returns a typed handle for the cross-tenant admin RPCs. See Admin for
    the full surface.

func (c *DbClient) ClearOffsets()
    ClearOffsets drops every tracked per-tenant write offset, so the
    next read no longer pins to a past write. Mirrors the Python SDK's
    “DbClient.clear_offsets()“ — useful in tests and when a caller deliberately
    wants to stop enforcing read-after-write.

    Automatic offset tracking otherwise needs zero application code: every
    Commit records “receipt.stream_position“ for its tenant and every subsequent
    read auto-attaches it as “after_offset“.

func (c *DbClient) Close() error
    Close tears down the gRPC connection.

func (c *DbClient) Config() clientConfig
    Config returns a copy of the client configuration (for inspection/testing).

func (c *DbClient) Connect(ctx context.Context) error
    Connect establishes the gRPC connection to the server.

func (c *DbClient) GetReceiptStatus(ctx context.Context, tenantID, actor, idempotencyKey string) (ReceiptStatus, string, error)
    GetReceiptStatus polls the status of a transaction by its idempotency key.
    Returns the status enum, an optional error message (when status is FAILED),
    and any transport error.

    Useful when a network blip dropped the original commit response — re-issuing
    the same idempotency_key on a new ExecuteAtomic call is also safe (the WAL
    deduplicates) but “GetReceiptStatus“ is the cheap "did it actually land?"
    probe.

func (c *DbClient) GetTenantQuota(ctx context.Context, actor, tenantID string) (*TenantQuota, error)
    GetTenantQuota fetches the tenant's quota configuration and current usage in
    a single round-trip. It is a direct client- level call (rather than scoped)
    because quota data is admin- oriented — there is no typed node behind it.

func (c *DbClient) Health(ctx context.Context) (*HealthStatus, error)
    Health returns a snapshot of server health for liveness / readiness probes.
    The response includes a boolean healthy flag, the server version string,
    and a per-component status map (e.g. “"wal"“ → “"ok"“).

func (c *DbClient) NewPlan(tenantID, actor string) *Plan
    NewPlan creates a new Plan for batching operations atomically.

func (c *DbClient) NewPlanWithKey(tenantID, actor, idempotencyKey string) *Plan
    NewPlanWithKey creates a new Plan with an explicit idempotency key.

func (c *DbClient) Tenant(tenantID string) *TenantScope
    Tenant returns a TenantScope for the given tenant.

        scope := client.Tenant("acme").Actor(entdb.UserActor("bob"))
        product, err := entdb.Get[*shop.Product](ctx, scope, "node-42")

func (c *DbClient) Transport() Transport
    Transport returns the underlying Transport this client uses.

    It is a read-only accessor intended for advanced consumers (e.g. custom
    tooling or tests) that need to reach the transport without resorting to
    reflection and unsafe. The returned value must be treated as read-only;
    mutating transport state out from under the client is unsupported.

func (c *DbClient) WaitForOffset(ctx context.Context, tenantID, actor, streamPosition string, timeoutMs int32) (bool, string, error)
    WaitForOffset blocks server-side until the WAL applier has reached
    “streamPosition“ for “tenantID“, or “timeoutMs“ elapses. Returns “(reached,
    currentPosition, err)“. “reached“ is true when the offset was met;
    “currentPosition“ lets the caller see how far behind the applier is when it
    wasn't.

    This is the explicit form of the “wait_applied“ option callers can pass to
    Plan.Commit — useful for read-after-write across independent processes that
    share an idempotency key but not a commit chain.

type DelegateResult struct {
	Delegated   int32
	ExpiresAtMs int64
}
    DelegateResult is returned by “Admin.DelegateAccess“ — how many nodes were
    affected and the absolute expiry timestamp.

type DeletionScheduled struct {
	RequestedAtMs int64
	ExecuteAtMs   int64
	Status        string // typically "deletion_pending"
}
    DeletionScheduled is what “Admin.DeleteUser“ returns — the GDPR-mandated
    timestamps so callers can show "your data will be permanently removed at
    <date>" UI without a follow-up read.

type Edge struct {
	TenantID   string         `json:"tenant_id"`
	EdgeTypeID int            `json:"edge_type_id"`
	FromNodeID string         `json:"from_node_id"`
	ToNodeID   string         `json:"to_node_id"`
	Props      map[string]any `json:"props,omitempty"`
	CreatedAt  int64          `json:"created_at"`
}
    Edge represents a graph edge in EntDB.

type EntDBError struct {
	Message string
	Code    string
	Details map[string]any
}
    EntDBError is the base error type for all EntDB SDK errors.

func (e *EntDBError) Error() string

type Filter struct {
	Field string
	Op    FilterOp
	Value any
}
    Filter is one AND-ed predicate passed to QueryWhere (and DeleteWhere).
    Field is the proto field name (the gRPC boundary resolves it to a payload
    field_id). Op selects the comparison operator. Value is bound as a SQLite
    parameter and supports the JSON-marshallable scalars SQLite understands
    (string, int, int64, float64, bool, nil).

    Schema-less escape hatch (issue #545): Field may instead be the digit-only
    numeric payload field id (e.g. "4"). The SDK forwards Field unchanged;
    a server with no schema treats a digit-only key as a raw field id and skips
    name resolution. This is the only way to filter against a schema-less server
    — a field NAME there returns INVALID_ARGUMENT ("cannot translate filter key
    … without a schema"). A schema-configured server accepts either form.

    Multiple filters on the same field are permitted — a half-open range is
    expressed as two filters:

        []Filter{
            {Field: "expires_at", Op: FilterGe, Value: lo},
            {Field: "expires_at", Op: FilterLt, Value: hi},
        }

type FilterOp string
    FilterOp is the comparison operator carried by a Filter. The values
    mirror the wire FilterOp enum's Eq/Ne/Lt/Le/Gt/Ge subset (see
    proto/entdb/v1/entdb.proto). Issue #501 scopes the SDK to the six AND-able
    comparison operators; CONTAINS / IN remain server-side TBD and are not
    exposed here.

const (
	FilterEq FilterOp = "eq"
	FilterNe FilterOp = "ne"
	FilterLt FilterOp = "lt"
	FilterLe FilterOp = "le"
	FilterGt FilterOp = "gt"
	FilterGe FilterOp = "ge"
)
    Comparison operators accepted by Filter. Each compiles to the matching SQL
    operator on the indexed payload column.

    Index usage caveat:
      - FilterEq/Lt/Le/Gt/Ge use the existing “idx_query_t<type>_f<field>“
        expression index for B-tree lookups.
      - FilterNe (not-equal) CANNOT use a B-tree index — it forces a scan
        over every row of the requested type. Use sparingly on large tables;
        prefer a positive predicate (“> v OR < v“ is not expressible in this v1
        cut — re-issue the query if you need both branches).

type HealthStatus struct {
	Healthy    bool
	Version    string
	Components map[string]string
}
    HealthStatus mirrors “pb.HealthResponse“ — the boolean healthy flag, the
    server version string, and a map of named components to their status (e.g.
    "wal" → "ok", "global_store" → "ok").

type Node struct {
	TenantID   string         `json:"tenant_id"`
	NodeID     string         `json:"node_id"`
	TypeID     int            `json:"type_id"`
	Payload    map[string]any `json:"payload"`
	CreatedAt  int64          `json:"created_at"`
	UpdatedAt  int64          `json:"updated_at"`
	OwnerActor Actor          `json:"owner_actor"`
	ACL        []ACLEntry     `json:"acl,omitempty"`
}
    Node represents a graph node in EntDB.

func GetByKey[T any](ctx context.Context, s *Scope, key UniqueKey[T], value T) (*Node, error)
    GetByKey fetches a node by a unique key token. The token is produced by the
    protoc-gen-entdb-keys codegen plugin — user code cannot construct one by
    hand.

        product, err := entdb.GetByKey(ctx, scope, shop.ProductSKU, "WIDGET-1")

    The generic T on UniqueKey[T] constrains the value argument at compile time:
    passing an int where the key declares UniqueKey[string] does not type-check.

type NodeResolver interface {
	// Resolve returns the dial-target endpoint for ``nodeID``,
	// or an error if the node is unknown.
	Resolve(nodeID string) (endpoint string, err error)
}
    NodeResolver maps a server-issued “node_id“ (e.g. "node-a") to a dial-able
    gRPC endpoint (“host:port“). The SDK calls this when the server returns a
    redirect hint via the “entdb-redirect-node“ trailing metadata header.

    Two implementations ship with the SDK:

      - DNSTemplateResolver (default) treats “node_id“ as a DNS subdomain
        — “node-a“ resolves to “node-a.<BaseDomain>:<Port>“. This is the
        right choice for Kubernetes StatefulSet headless services and for any
        deployment that follows the convention "one DNS name per node id".
      - StaticMapResolver looks up the endpoint in an explicit map. Use it for
        tests, or for static deployments where DNS isn't available.

    Custom implementations can plug in via WithNodeResolver — for example a
    service-discovery client (Consul, etcd, AWS Cloud Map).

type NotFoundError struct {
	EntDBError
	ResourceType string
	ResourceID   string
}
    NotFoundError indicates a missing resource.

func NewNotFoundError(message, resourceType, resourceID string) *NotFoundError
    NewNotFoundError creates a new NotFoundError.

type Operation struct {
	Type       OperationType  `json:"type"`
	TypeID     int            `json:"type_id,omitempty"`
	NodeID     string         `json:"node_id,omitempty"`
	Alias      string         `json:"alias,omitempty"`
	Data       map[string]any `json:"data,omitempty"`
	Patch      map[string]any `json:"patch,omitempty"`
	EdgeTypeID int            `json:"edge_type_id,omitempty"`
	FromNodeID string         `json:"from_node_id,omitempty"`
	ToNodeID   string         `json:"to_node_id,omitempty"`
	ACL        []ACLEntry     `json:"acl,omitempty"`

	// StorageMode for create_node ops (2026-04-13 storage decision).
	// Defaults to StorageModeTenant when unset. Immutable after
	// creation — Update cannot change it.
	StorageMode StorageMode `json:"storage_mode,omitempty"`

	// TargetUserID is required when StorageMode is
	// StorageModeUserMailbox; ignored otherwise.
	TargetUserID string `json:"target_user_id,omitempty"`

	// Precondition is the optional CAS predicate for OpUpdateNode.
	// When set, the applier compares the node's current value at
	// Precondition.FieldID against Precondition.Equals BEFORE applying
	// the patch. A mismatch aborts the ENTIRE batch (no ops commit)
	// and the failure is memoized in the idempotency cache so a
	// retry with the same key replays the cached failure. Callers
	// re-evaluate by minting a new idempotency key. See GitHub
	// issues #500 and #525.
	//
	// Only valid on OpUpdateNode. Ignored for other op types.
	Precondition *Precondition `json:"precondition,omitempty"`

	// Where carries the AND-ed predicate for OpDeleteWhere (GitHub
	// issue #504). Field names are resolved to payload field ids at
	// the SDK boundary, exactly like QueryWhere. Only valid on
	// OpDeleteWhere; ignored for other op types.
	Where []Filter `json:"where,omitempty"`

	// Limit is the best-effort cap on the number of nodes deleted by
	// an OpDeleteWhere op (Postgres DELETE … LIMIT semantics). Zero
	// selects the server default; the server clamps to its own hard
	// ceiling. Ignored for other op types.
	Limit int `json:"limit,omitempty"`
}
    Operation represents a single mutation in a Plan.

    Data and Patch are keyed by proto field id (as decimal string) — never
    by field name. Translation from proto names happens at the SDK boundary
    (marshal.go). See the "Field IDs, not field names, on disk" invariant in
    CLAUDE.md for the rationale.

type OperationType int
    OperationType enumerates the kinds of operations in a Plan.

const (
	OpCreateNode OperationType = iota
	OpUpdateNode
	OpDeleteNode
	OpCreateEdge
	OpDeleteEdge
	// OpDeleteWhere is the single-RPC predicate-based sweeper
	// (GitHub issue #504): delete every node of a type matching an
	// AND-ed [Filter] predicate in one round trip, instead of a
	// QueryWhere + Delete loop.
	OpDeleteWhere
)
type Permission string
    Permission represents an ACL permission level.

const (
	PermissionRead  Permission = "read"
	PermissionWrite Permission = "write"
	PermissionAdmin Permission = "admin"
)
type Plan struct {
	// Has unexported fields.
}
    Plan collects mutation operations to be committed atomically.

    The 2026-04-14 SDK v0.3 API has exactly one way to perform each mutation:
    Plan.Create for nodes (options go through CreateOption), Plan.Update
    for partial updates, and the free generic functions Delete, EdgeCreate,
    EdgeDelete for the ops that need a compile-time type witness without a
    message instance.

    Go does not support generic methods, so the three type-witness ops are
    package-level functions that take the *Plan as their first argument:

        plan := scope.Plan()
        plan.Create(&shop.Product{Sku: "WIDGET-1", Name: "Widget"})
        plan.Update("node-42", &shop.Product{PriceCents: 1999})
        entdb.Delete[*shop.Product](plan, "node-old")
        entdb.EdgeCreate[*shop.PurchaseEdge](plan, "user-1", "node-42")
        entdb.EdgeDelete[*shop.PurchaseEdge](plan, "user-1", "node-42")
        _, err := plan.Commit(ctx)

    A Plan cannot be reused after Commit.

func (p *Plan) Actor() string
    Actor returns the actor this plan is bound to.

func (p *Plan) Commit(ctx context.Context) (*CommitResult, error)
    Commit sends all accumulated operations to the server as a single atomic
    transaction. The Plan cannot be used after Commit is called.

func (p *Plan) Create(msg proto.Message, opts ...CreateOption) string
    Create accumulates a create-node operation.

    The type_id is read from the message's “(entdb.node).type_id“ proto option;
    field values are encoded by proto field id. Returns the alias usable as an
    edge endpoint later in the same plan.

    Options configure storage mode, ACL, and the alias string:

        alias := plan.Create(&shop.Product{Sku: "WIDGET-1"},
            entdb.WithACL(entdb.ACLEntry{Grantee: entdb.UserActor("bob"),
                                         Permission: entdb.PermissionRead}),
            entdb.As("first-product"),
        )

    If “msg“ is not an EntDB node type (no “(entdb.node)“ option) Create
    panics — the failure is at compile-time-adjacent developer speed, surfaced
    immediately in unit tests.

func (p *Plan) IdempotencyKey() string
    IdempotencyKey returns the idempotency key for this plan.

func (p *Plan) Operations() []Operation
    Operations returns a copy of the accumulated operations (for
    inspection/testing).

func (p *Plan) TenantID() string
    TenantID returns the tenant this plan is bound to.

func (p *Plan) Update(nodeID string, msg proto.Message)
    Update accumulates an update-node operation.

    Only fields set on “msg“ are sent: proto3 “Range“ iterates only non-default
    scalars and explicitly-set “optional“ fields and submessages, which matches
    the intuitive "patch = fields I actually touched" semantics. The type_id
    is read from the message's “(entdb.node)“ option — callers do not pass it
    separately.

func (p *Plan) UpdateFields(nodeID string, msg proto.Message, fields ...string)
    UpdateFields accumulates an update-node operation for an EXPLICIT set of
    fields, including fields whose value is the proto3 zero value (false / 0
    / ""). Use it when you need to set a field BACK to its zero value — the
    default Plan.Update sends only non-default fields (proto3 cannot tell "set
    to zero" from "unset" for scalars), so a true→false / non-empty→"" / n→0
    update is otherwise silently dropped (#574).

        // clear the verified flag (Update would drop the false)
        plan.UpdateFields("node-1", &auth.TotpCredential{}, "verified")

    field names are proto field names (the SDK resolves them to field ids).
    Naming a field reads its current value off msg — set the ones you want to
    change on msg first, then list them here. At least one field is required;
    an unknown field name panics at call time.

func (p *Plan) UpdateIf(nodeID string, msg proto.Message, field string, equals any)
    UpdateIf accumulates a conditional update-node operation with a single-field
    equality precondition (CAS). The applier compares the node's current
    “field“ against “equals“ before applying the patch; on mismatch the whole
    batch aborts and Commit returns a *PreconditionFailure error matching
    ErrPreconditionFailed.

    Use this for state-machine transitions and single-use tokens — anywhere two
    concurrent writers must not both win. See GitHub issue #500 for the design.

    The idempotency cache memoizes both success and CAS-miss outcomes,
    so a retry with the SAME idempotency key returns the cached result without
    re-evaluating. To re-evaluate against current state, mint a new idempotency
    key.

type Precondition struct {
	// Field is the developer-facing node field name as declared in
	// the proto schema. The SDK resolves it to FieldID before sending
	// the request and keeps the name for diagnostics.
	Field string `json:"field"`

	// FieldID is the stable numeric field id used by the server and
	// on-disk payloads. Plan.UpdateIf populates this from the proto
	// descriptor so schema-less servers do not need a registry for CAS.
	FieldID int `json:"field_id,omitempty"`

	// Equals is the expected current value of Field. Scalar Go
	// types (string, bool, integers, floats) are encoded as the
	// corresponding google.protobuf.Value scalars; nil maps to
	// NullValue (matches "field absent" by JSON-null comparison).
	Equals any `json:"equals"`
}
    Precondition is a single-field equality predicate evaluated by the applier
    against the materialized node state before a patch is applied. Equality is
    the only operator in v1.

type PreconditionFailure struct {
	// OpIndex is the zero-based index of the failing op within the
	// ExecuteAtomic request.
	OpIndex int
	// Field is the node field name the precondition referenced.
	Field string
	// ExpectedValue is the value the caller's precondition expected.
	ExpectedValue any
	// ObservedValue is the value the applier read at evaluation
	// time. A nil value means the field was absent from the stored
	// payload (proto NullValue on the wire).
	ObservedValue any
}
    PreconditionFailure is the typed error returned by ExecuteAtomic when an
    op's precondition fails. It unwraps to ErrPreconditionFailed for the common
    "did this batch lose a CAS race?" branch and carries the structured detail
    so callers can log/decide without a second round trip.

    The batch as a whole did not commit — every op (the conditional one and any
    side-effect ops in the same batch) is a no-op.

func (e *PreconditionFailure) Error() string

func (e *PreconditionFailure) Unwrap() error
    Unwrap reports ErrPreconditionFailed so callers can use “errors.Is“ for the
    common "did any precondition fail?" branch.

type QueryOption func(*queryConfig)
    QueryOption configures a single Query call. Placeholder for future
    pagination / ordering options — declared here so the free-function signature
    in scope.go doesn't churn every time a new knob lands.

func WithLimit(n int32) QueryOption
    WithLimit caps the number of results returned.

func WithOffset(n int32) QueryOption
    WithOffset skips the first N results.

type RateLimitError struct {
	EntDBError
	// RetryAfterMs is the server-suggested wait in milliseconds before
	// retrying. Zero when the server did not provide one.
	RetryAfterMs int64
	// Limit is the quota limit that was hit (monthly writes, or RPS).
	// Zero when the server did not surface a numeric limit.
	Limit int64
	// Used is the consumed amount in the current period. Zero when the
	// server did not surface a numeric usage count.
	Used int64
}
    RateLimitError indicates the caller was throttled by one of the three
    quota layers (monthly write quota, per-tenant RPS, per-user RPS).
    It mirrors the Python SDK's RateLimitError so callers can write a single
    handler that covers all three phases of the rate-limit model frozen in
    docs/adr/024-three-layer-rate-limit-model.md.

    The server sends:
      - gRPC status code RESOURCE_EXHAUSTED
      - a "retry-after" trailing metadata entry (seconds, decimal string)
      - a human-readable status message

    Callers should treat a non-zero RetryAfterMs as a hint — the underlying
    reason may be a burst-RPS throttle (sub-second wait) or a monthly quota
    exhaustion (wait until the next calendar month). The typical retry strategy
    is exponential backoff capped at RetryAfterMs.

func NewRateLimitError(message string, retryAfterMs int64) *RateLimitError
    NewRateLimitError constructs a RateLimitError with the given message and
    server-suggested retry window (in milliseconds).

func (e *RateLimitError) Error() string
    Error implements the error interface.

type Receipt struct {
	TenantID       string `json:"tenant_id"`
	IdempotencyKey string `json:"idempotency_key"`
	StreamPosition string `json:"stream_position,omitempty"`
}
    Receipt tracks a committed transaction.

type ReceiptStatus int32
    ReceiptStatus mirrors the wire enum (UNKNOWN / PENDING / APPLIED / FAILED).

const (
	ReceiptStatusUnknown ReceiptStatus = 0
	ReceiptStatusPending ReceiptStatus = 1
	ReceiptStatusApplied ReceiptStatus = 2
	ReceiptStatusFailed  ReceiptStatus = 3
)
type RevokeAllResult struct {
	RevokedGrants int32
	RevokedGroups int32
	RevokedShared int32
}
    RevokeAllResult tallies what “Admin.RevokeAllUserAccess“ removed.

type SchemaError struct {
	EntDBError
	ExpectedFingerprint string
	ActualFingerprint   string
}
    SchemaError indicates a schema-related problem.

func NewSchemaError(message, expected, actual string) *SchemaError
    NewSchemaError creates a new SchemaError.

type Scope struct {
	// Has unexported fields.
}
    Scope is a fully-scoped handle with (client, tenantID, actor) pre-bound.

    The typed accessors Get, GetByKey, and Query are package- level free
    functions rather than methods on Scope because Go does not allow generic
    methods. The canonical call shape is:

        scope := client.Tenant("acme").Actor(entdb.UserActor("bob"))
        product, err := entdb.Get[*shop.Product](ctx, scope, "node-42")
        match, err := entdb.GetByKey(ctx, scope, shop.ProductSKU, "WIDGET-1")
        results, err := entdb.Query[*shop.Product](ctx, scope,
            map[string]any{"price_cents": map[string]any{"$gte": 1000}})

    The free-function-with-scope-as-first-arg pattern is the canonical Go
    workaround for generic methods.

func (s *Scope) ActorID() Actor
    ActorID returns the actor this scope is bound to.

func (s *Scope) AddGroupMember(ctx context.Context, groupID string, member Actor, role string) error
    AddGroupMember adds “member“ to the ACL group identified by “groupID“ (a
    node id of the group node). “role“ is an optional free-form label like
    “"admin"“ or “"member"“.

func (s *Scope) Connected(ctx context.Context, nodeID string, edgeTypeID int) ([]*Node, error)
    Connected returns nodes reachable from “nodeID“ via edges of “edgeTypeID“.
    Server-side ACL filtering is applied.

func (s *Scope) EdgesFrom(ctx context.Context, fromNodeID string, edgeTypeID int) ([]*Edge, error)
    EdgesFrom retrieves outgoing edges from a node via this scope.

    Kept non-generic because the caller passes the edge_type_id as an int —
    a typed “EdgeType[*shop.PurchaseEdge]“ overload can be added as a strict
    replacement when the need is concrete.

func (s *Scope) EdgesTo(ctx context.Context, toNodeID string, edgeTypeID int) ([]*Edge, error)
    EdgesTo retrieves incoming edges to a node via this scope.

func (s *Scope) Plan() *Plan
    Plan creates a new Plan pre-bound to this scope's tenant and actor.

func (s *Scope) PlanWithKey(idempotencyKey string) *Plan
    PlanWithKey creates a new Plan with an explicit idempotency key, pre-bound
    to this scope's tenant and actor.

func (s *Scope) RemoveGroupMember(ctx context.Context, groupID string, member Actor) error
    RemoveGroupMember removes “member“ from the ACL group.

func (s *Scope) RevokeAccess(ctx context.Context, nodeID string, grantee Actor) (bool, error)
    RevokeAccess removes a previously-shared grant on a node.

    Returns true if the grant existed and was removed, false if no matching
    grant was found (idempotent — re-revoke is a no-op).

func (s *Scope) Share(ctx context.Context, nodeID string, grantee Actor, perm Permission) error
    Share grants permission on a node to another actor.

    Share stays a regular method because it does not need a type parameter — the
    node_id fully identifies the row.

func (s *Scope) SharedWithMe(ctx context.Context, limit int32) ([]*Node, error)
    SharedWithMe returns nodes other actors have shared with this scope's actor
    — including cross-tenant shares routed through the global shared_index.

    The transport auto-follows the ADR-029 unified keyset cursor across both
    merged sources, so this returns the COMPLETE set by default — never a
    silent 100-row prefix. A positive “limit“ caps the total; “limit <= 0“ (the
    default) returns every shared node. (The deprecated per-source “offset“ is
    superseded by the cursor and no longer exposed here.)

func (s *Scope) TenantID() string
    TenantID returns the tenant this scope is bound to.

func (s *Scope) TransferOwnership(ctx context.Context, nodeID string, newOwner Actor) (bool, error)
    TransferOwnership reassigns “nodeID“'s owner to “newOwner“. Returns false if
    the node was not found.

type StaticMapResolver struct {
	Endpoints map[string]string
}
    StaticMapResolver is the explicit “node_id -> endpoint“ form. It is the
    resolver of choice for tests (DNS isn't available in most test environments)
    and for tiny static deployments where listing endpoints by hand is fine.

func (r *StaticMapResolver) Resolve(nodeID string) (string, error)
    Resolve looks up “nodeID“ in the static map. Returns an error if the node
    id isn't present — callers should treat that as a permanent failure (the SDK
    will surface it as a ConnectionError).

type StorageMode int
    StorageMode selects the physical SQLite file a node lives in.

    Storage mode is chosen at creation time and is IMMUTABLE. It cannot be
    changed by Update. See docs/adr/020-immutable-storage-mode.md for the full
    rationale.

const (
	// StorageModeTenant is the default — node lives in tenant.db and
	// is sharable via ACL within the tenant.
	StorageModeTenant StorageMode = 0

	// StorageModeUserMailbox — node lives in a per-user SQLite file
	// ({tenant_id}/user_{user_id}.db) and is private to one user.
	StorageModeUserMailbox StorageMode = 1

	// StorageModePublic — node lives in the singleton public.db and
	// is readable by any tenant.
	StorageModePublic StorageMode = 2
)
type TenantDetail struct {
	TenantID  string
	Name      string
	Status    string
	Region    string
	CreatedAt int64
}
    TenantDetail mirrors “pb.TenantDetail“ — the identity-layer description of a
    tenant (not its quota, not its data).

    “Region“ is the geographic region the tenant's data is pinned to (e.g.
    “"us-east-1"“, “"eu-west-1"“). It is set when the tenant is created — either
    explicitly via WithRegion on Admin.CreateTenant or, when omitted, defaulted
    server-side to the serving region. Cross-region requests are rejected with
    FAILED_PRECONDITION; clients that need to talk to a tenant in another region
    must connect to a server in that region.

type TenantMember struct {
	TenantID string
	UserID   string
	Role     string
	JoinedAt int64
}
    TenantMember describes one “(tenant_id, user_id, role)“ row from
    “tenant_members“ — used by both “GetTenantMembers“ (rows for one tenant) and
    “GetUserTenants“ (rows for one user).

type TenantQuota struct {
	TenantID string `json:"tenant_id"`
	// Phase 1 — monthly writes.
	MaxWritesPerMonth int64 `json:"max_writes_per_month"`
	WritesUsed        int64 `json:"writes_used"`
	PeriodStartMs     int64 `json:"period_start_ms"`
	PeriodEndMs       int64 `json:"period_end_ms"`
	HardEnforce       bool  `json:"hard_enforce"`
	// Phase 2 — per-tenant token bucket.
	MaxRPSSustained int32 `json:"max_rps_sustained"`
	MaxRPSBurst     int32 `json:"max_rps_burst"`
	// Phase 3 — per-user token bucket.
	MaxRPSPerUserSustained int32 `json:"max_rps_per_user_sustained"`
	MaxRPSPerUserBurst     int32 `json:"max_rps_per_user_burst"`
}
    TenantQuota is the decoded snapshot returned by the GetTenantQuota
    RPC. It mirrors the three-layer rate-limit model (monthly quota,
    per-tenant token bucket, per-user token bucket) defined in
    docs/adr/024-three-layer-rate-limit-model.md so callers can render a full
    quota dashboard from a single round-trip.

    Numeric fields use int64 to match the proto (and to give dashboards headroom
    for high-volume tiers). A zero value for any *Limit field means "unlimited"
    — the server treats 0 as unbounded.

type TenantScope struct {
	// Has unexported fields.
}
    TenantScope captures a (client, tenantID) pair, allowing callers to avoid
    repeating the tenant on every call.

func (s *TenantScope) Actor(actor Actor) *Scope
    Actor binds a typed Actor to this tenant scope, returning a fully-scoped
    Scope ready for reads and writes.

func (s *TenantScope) TenantID() string
    TenantID returns the tenant this scope is bound to.

type TransactionError struct {
	EntDBError
	IdempotencyKey string
}
    TransactionError indicates an atomic transaction failure.

func NewTransactionError(message, idempotencyKey string) *TransactionError
    NewTransactionError creates a new TransactionError.

type Transport interface {
	// Connect establishes the connection to the server.
	Connect(ctx context.Context) error
	// Close tears down the connection.
	Close() error
	// GetNode retrieves a single node.
	GetNode(ctx context.Context, tenantID, actor string, typeID int, nodeID string) (*Node, error)
	// GetNodeByKey resolves a node via a declared unique key.
	//
	// The contract is ``(type_id, field_id, value)``. value is the raw
	// Go scalar; the transport builds both the legacy
	// ``google.protobuf.Value`` and the typed ``EntValue`` (ADR-028 /
	// #572) from it so int64 keys above 2^53 round-trip losslessly while
	// older servers keep reading the legacy field.
	GetNodeByKey(ctx context.Context, tenantID, actor string, typeID, fieldID int32, value any) (*Node, error)
	// QueryNodes retrieves nodes matching a filter.
	// QueryNodes returns nodes matching a filter. The transport follows
	// the ADR-029 keyset cursor across pages so the complete set is
	// returned, never a silent 100-row prefix. limit caps the total when
	// positive; limit <= 0 returns every matching row.
	QueryNodes(ctx context.Context, tenantID, actor string, typeID int, filter map[string]any, limit int) ([]*Node, error)
	// ExecuteAtomic commits a batch of operations atomically.
	ExecuteAtomic(ctx context.Context, tenantID, actor, idempotencyKey string, ops []Operation) (*CommitResult, error)
	// Share grants permission on a node to another actor.
	Share(ctx context.Context, tenantID, actor, nodeID, grantee, permission string) error
	// GetEdgesFrom retrieves outgoing edges from a node.
	GetEdgesFrom(ctx context.Context, tenantID, actor, fromNodeID string, edgeTypeID int) ([]*Edge, error)
	// GetEdgesTo retrieves incoming edges to a node.
	GetEdgesTo(ctx context.Context, tenantID, actor, toNodeID string, edgeTypeID int) ([]*Edge, error)
	// SearchNodes performs full-text search across searchable fields.
	//
	// ADR-029 FTS carve-out: search is relevance-ranked top-N and is
	// OFFSET-paged, NOT cursor-paged — FTS5 `rank` is not a stable keyset
	// column. The transport does NOT auto-follow to completion (unlike
	// QueryNodes / GetEdges / shared-with-me). pageSize bounds one page
	// (alias for the legacy limit); offset advances within the ranked set.
	// Returns the page plus has_more so callers can page deliberately.
	SearchNodes(ctx context.Context, tenantID, actor string, typeID int, query string, pageSize, offset int32) ([]*Node, bool, error)
	// GetTenantQuota retrieves the tenant's quota configuration
	// and current usage (monthly writes + per-tenant / per-user
	// RPS buckets).
	GetTenantQuota(ctx context.Context, actor, tenantID string) (*TenantQuota, error)
	// GetNodes batch-fetches nodes by id. Returns the slice of
	// found nodes plus the slice of ids that were missing — both in
	// input order is NOT guaranteed.
	GetNodes(ctx context.Context, tenantID, actor string, typeID int, nodeIDs []string) ([]*Node, []string, error)
	// GetReceiptStatus polls a transaction by its idempotency key.
	GetReceiptStatus(ctx context.Context, tenantID, actor, idempotencyKey string) (ReceiptStatus, string, error)
	// WaitForOffset blocks (server-side) until the applier has
	// reached ``streamPosition`` for the tenant, or ``timeoutMs``
	// elapses. Returns whether the offset was reached and the
	// current position (so callers can detect "still behind by N").
	WaitForOffset(ctx context.Context, tenantID, actor, streamPosition string, timeoutMs int32) (bool, string, error)
	// GetConnectedNodes returns nodes reachable from ``nodeID`` via
	// edges of ``edgeTypeID`` — server-side ACL filtered.
	GetConnectedNodes(ctx context.Context, tenantID, actor, nodeID string, edgeTypeID int) ([]*Node, error)
	// ListSharedWithMe returns nodes other actors have shared with
	// the calling actor (cross-tenant included). The transport follows
	// the ADR-029 unified keyset cursor across pages so the COMPLETE set
	// is returned, never a silent prefix. limit caps the total when
	// positive; limit <= 0 returns every shared node.
	ListSharedWithMe(ctx context.Context, tenantID, actor string, limit int32) ([]*Node, error)
	// RevokeAccess removes a previously-shared grant from a node.
	RevokeAccess(ctx context.Context, tenantID, actor, nodeID, granteeActor string) (bool, error)
	// TransferOwnership reassigns ``nodeID``'s ``owner_actor`` to
	// ``newOwner``. Returns false if the node was not found.
	TransferOwnership(ctx context.Context, tenantID, actor, nodeID, newOwner string) (bool, error)
	// AddGroupMember / RemoveGroupMember manage the membership of an
	// ACL group node — ``groupID`` is the node id of the group,
	// ``memberActor`` is the principal being added or removed.
	AddGroupMember(ctx context.Context, tenantID, actor, groupID, memberActor, role string) error
	RemoveGroupMember(ctx context.Context, tenantID, actor, groupID, memberActor string) error

	CreateTenant(ctx context.Context, actor, tenantID, name string, opts ...CreateTenantOption) (*TenantDetail, error)
	CreateUser(ctx context.Context, actor, userID, email, name string) (*UserInfo, error)
	AddTenantMember(ctx context.Context, actor, tenantID, userID, role string) error
	RemoveTenantMember(ctx context.Context, actor, tenantID, userID string) error
	ChangeMemberRole(ctx context.Context, actor, tenantID, userID, newRole string) error
	GetTenantMembers(ctx context.Context, actor, tenantID string) ([]TenantMember, error)
	GetUserTenants(ctx context.Context, actor, userID string) ([]TenantMember, error)
	DelegateAccess(ctx context.Context, actor, tenantID, fromUser, toUser, permission string, expiresAtMs int64) (*DelegateResult, error)
	TransferUserContent(ctx context.Context, actor, tenantID, fromUser, toUser string) (int32, error)
	RevokeAllUserAccess(ctx context.Context, actor, tenantID, userID string) (*RevokeAllResult, error)

	// DeleteUser requests deletion of ``userID``. The deletion is
	// scheduled — actually executed by the GDPR worker after a
	// grace period. Pass ``graceDays = 0`` to use the server
	// default (30 days). The user enters the ``"deletion_pending"``
	// status immediately; reads continue to succeed until the grace
	// expires.
	DeleteUser(ctx context.Context, actor, userID string, graceDays int32) (*DeletionScheduled, error)
	// CancelUserDeletion lifts a pending deletion. Idempotent on a
	// user with no pending deletion (returns success but no
	// observable change).
	CancelUserDeletion(ctx context.Context, actor, userID string) error
	// ExportUserData returns a JSON-serialized bundle of every node
	// the user owns across all tenants — the GDPR Article 20
	// portability dump. The shape is documented on the server side.
	ExportUserData(ctx context.Context, actor, userID string) (string, error)
	// FreezeUser flips ``userID`` between ``"active"`` and
	// ``"frozen"``. A frozen user cannot read or write, but their
	// data is preserved (no deletion clock starts). Pass
	// ``enabled = true`` to freeze, ``false`` to unfreeze.
	FreezeUser(ctx context.Context, actor, userID string, enabled bool) (string, error)

	// Health returns a snapshot of server health — useful as a
	// Kubernetes liveness/readiness probe target. Tenant-agnostic.
	Health(ctx context.Context) (*HealthStatus, error)
}
    Transport is the low-level interface between the SDK and the gRPC layer.
    It is intentionally NOT part of the public single- shape API — user code
    goes through the typed Scope / Plan surface, which reaches the transport
    through internal calls.

    Transport methods take bare ints (“typeID“) and raw field ids because they
    sit below the proto-descriptor boundary; every public entry point above this
    layer is typed end-to-end.

type UniqueConstraintError struct {
	EntDBError
	TenantID string
	TypeID   int32
	FieldID  int32
	Value    any
	// ConstraintName is non-empty when the violation came from a
	// composite (multi-field) `(entdb.node).composite_unique`
	// constraint. Single-field violations leave it empty and use
	// `FieldID` / `Value` instead.
	ConstraintName string
	// FieldIDs / Values mirror the composite constraint coordinates.
	// Both are nil for single-field violations.
	FieldIDs []int32
	Values   []any
}
    UniqueConstraintError indicates that a create/update operation violated
    a declared unique field (2026-04-14 SDK v0.3 decision, supersedes
    2026-04-13 unique_keys). The server surfaces this via gRPC ALREADY_EXISTS,
    with a status message identifying the colliding field_id + value.
    parseUniqueConstraintFromStatus turns those responses into a typed error so
    callers can write

        if uce, ok := err.(*entdb.UniqueConstraintError); ok {
            // fetch the existing node via entdb.GetByKey(ctx, scope, ...)
        }

    Fields mirror the server-side payload:

      - TypeID / FieldID identify the (type, unique field) pair.
      - Value holds the colliding value as the Go scalar the caller originally
        passed through the wire.

func NewUniqueConstraintError(tenantID string, typeID, fieldID int32, value any) *UniqueConstraintError
    NewUniqueConstraintError builds a typed UniqueConstraintError from the
    colliding-field coordinates. The message is set from the arguments so
    callers can log it directly.

func (e *UniqueConstraintError) Error() string
    Error implements the error interface. It prefers the server-provided status
    message when one is available, falling back to the typed coordinates so the
    message remains informative.

func (e *UniqueConstraintError) IsComposite() bool
    IsComposite reports whether this is a composite (multi-field)
    unique-constraint violation as opposed to a single-field one.

type UniqueKey[T any] struct {
	TypeID  int32
	FieldID int32
	Name    string
}
    UniqueKey is a typed reference to a unique field on a node type.

    Instances of this struct are produced exclusively by the
    protoc-gen-entdb-keys codegen plugin. The struct fields are exported so
    the generated code can construct instances directly, but user code should
    never construct one by hand — get them from the generated <name>_entdb.go
    alongside your proto package.

    The T type parameter exists so GetByKey can constrain the value argument:
    passing a string where the generated token declares UniqueKey[int64] is a
    compile-time error. Generated code looks like:

        var ProductSKU = entdb.UniqueKey[string]{TypeID: 201, FieldID: 1, Name: "sku"}

    ADR-025 (docs/adr/025-single-shape-sdk-api.md) is the authority on why
    this type exists and why there is deliberately no public constructor for
    "hand-rolled" keys.

type UserInfo struct {
	UserID    string
	Email     string
	Name      string
	Status    string
	CreatedAt int64
	UpdatedAt int64
}
    UserInfo mirrors “pb.UserInfo“ from the global user registry.

type ValidationError struct {
	EntDBError
	FieldName string
	Errors    []string
}
    ValidationError indicates a payload validation failure.

func NewValidationError(message string, fieldName string, errors []string) *ValidationError
    NewValidationError creates a new ValidationError.
```
