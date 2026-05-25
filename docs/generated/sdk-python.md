<!-- GENERATED FILE — do not edit by hand.
     Regenerate with: python scripts/generate_api_docs.py
     Source of truth is the proto + SDK code, not this file. -->

# Python SDK Reference (generated)

Extracted from `sdk/python/entdb_sdk` — `__all__` plus the public methods of `DbClient` (connection entrypoint) and `Plan` (chained-write builder). Narrative + Go side-by-side lives in [sdk-reference.md](../sdk-reference.md).

## Public API surface (`from entdb_sdk import ...`)

- `ACLEntry`
- `AclDefaults`
- `Actor`
- `ActorScope`
- `AliasRef`
- `CompositeUniqueDef`
- `ConnectionError`
- `DNSTemplateResolver`
- `DataPolicy`
- `DbClient`
- `EdgeTypeDef`
- `EntDbError`
- `FieldDef`
- `FieldKind`
- `Filter`
- `FilterOp`
- `Mailbox`
- `NodeRef`
- `NodeResolver`
- `NodeTypeDef`
- `Permission`
- `Plan`
- `PreconditionFailedError`
- `Public`
- `RateLimitError`
- `Receipt`
- `SchemaError`
- `SchemaRegistry`
- `ScopedPlan`
- `StaticMapResolver`
- `Storage`
- `SubjectExitPolicy`
- `Tenant`
- `TenantScope`
- `TypedEdge`
- `TypedNode`
- `UniqueConstraintError`
- `UniqueKey`
- `UnknownFieldError`
- `ValidationError`
- `__version__`
- `field`
- `get_registry`
- `register_edge_type`
- `register_node_type`
- `register_proto_schema`

## `DbClient` methods

| Method | Description |
|---|---|
| `async add_tenant_member(tenant_id, user_id, role=..., *, actor=..., timeout=...)` | Add a member to a tenant. |
| `async archive_tenant(tenant_id, *, actor=..., timeout=...)` | Archive a tenant. |
| `atomic(tenant_id, actor, idempotency_key=..., *, trace_id=...)` | Create an atomic transaction plan. |
| `async cancel_user_deletion(user_id, *, actor=..., timeout=...)` | Cancel a pending user deletion during the grace period. |
| `async change_member_role(tenant_id, user_id, new_role, *, actor=..., timeout=...)` | Change a member's role in a tenant. |
| `clear_offsets()` | Clear tracked offsets. Useful for testing. |
| `async close()` | Close the connection. |
| `async connect()` | Connect to the server. |
| `async connected(node_id, edge_type, tenant_id, actor, *, limit=..., offset=..., after_offset=..., trace_id=..., timeout=...)` | Get connected nodes via edge type with ACL filtering. |
| `async create_tenant(tenant_id, name, *, actor=..., region=..., timeout=...)` | Create a new tenant in the global registry. |
| `async create_user(user_id, email, name, *, actor=..., timeout=...)` | Create a new user in the global registry. |
| `async delegate_access(tenant_id, from_user, to_user, permission=..., expires_at=..., *, actor=..., timeout=...)` | Grant to_user temporary access to all of from_user's nodes. |
| `async delete_user(user_id, *, actor=..., grace_days=..., timeout=...)` | Right to erasure (GDPR Article 17). |
| `async edges_in(node_id, tenant_id, actor, edge_type=..., *, after_offset=..., trace_id=..., timeout=...)` | Get incoming edges to a node. |
| `async edges_out(node_id, tenant_id, actor, edge_type=..., *, after_offset=..., trace_id=..., timeout=...)` | Get outgoing edges from a node. |
| `async export_user_data(user_id, *, actor=..., timeout=...)` | Right to access / portability (GDPR Articles 15, 20). |
| `async freeze_user(user_id, *, enabled=..., actor=..., timeout=...)` | Right to restrict processing (GDPR Article 18). |
| `async get(node_type, node_id, tenant_id, actor, *, after_offset=..., trace_id=..., timeout=...)` | Get a node by ID. |
| `async get_by_key(key, value, tenant_id, actor, *, after_offset=..., trace_id=..., timeout=...)` | Resolve a node via a typed unique-key token. |
| `async get_many(node_type, node_ids, tenant_id, actor, *, after_offset=..., trace_id=..., timeout=...)` | Get multiple nodes by IDs. |
| `async get_receipt_status(tenant_id, idempotency_key, *, trace_id=..., timeout=...)` | Get receipt status. |
| `async get_tenant(tenant_id, *, actor=..., timeout=...)` | Get a tenant by ID. |
| `async get_tenant_members(tenant_id, *, actor=..., timeout=...)` | List all members of a tenant. |
| `async get_user(user_id, *, actor=..., timeout=...)` | Get a user by ID. |
| `async get_user_tenants(user_id, *, actor=..., timeout=...)` | List all tenants a user belongs to. |
| `async group_add(group_id, member_actor_id, tenant_id, actor, role=..., *, trace_id=..., timeout=...)` | Add a member to a group. |
| `async group_remove(group_id, member_actor_id, tenant_id, actor, *, trace_id=..., timeout=...)` | Remove a member from a group. |
| `async health(*, timeout=...)` | Check server health. |
| `async list_users(*, status=..., limit=..., offset=..., actor=..., timeout=...)` | List users filtered by status. |
| `async query(node_type, tenant_id, actor, *, filter=..., where=..., limit=..., offset=..., order_by=..., descending=..., after_offset=..., trace_id=..., timeout=...)` | Query nodes by type. |
| `async remove_tenant_member(tenant_id, user_id, *, actor=..., timeout=...)` | Remove a member from a tenant. |
| `async revoke(node_id, actor_id, tenant_id, actor, *, trace_id=..., timeout=...)` | Revoke access from an actor on a node. |
| `async revoke_all_user_access(tenant_id, user_id, *, actor=..., timeout=...)` | Remove all access for a user in a tenant (instant termination). |
| `async search(query, user_id, tenant_id, actor, types=..., limit=..., *, trace_id=..., timeout=...)` | Search user's mailbox. |
| `async search_nodes(node_type, tenant_id, actor, query, *, limit=..., offset=..., page_size=..., trace_id=..., timeout=...)` | Full-text search across searchable fields of a node type. |
| `async search_nodes_page(node_type, tenant_id, actor, query, *, limit=..., offset=..., page_size=..., trace_id=..., timeout=...)` | :meth:`search_nodes` that also reports ``has_more``. |
| `async set_legal_hold(tenant_id, enabled=..., *, actor=..., timeout=...)` | Enable or disable legal hold on a tenant. |
| `async share(node_id, actor_id, tenant_id, actor, permission=..., *, actor_type=..., expires_at=..., trace_id=..., timeout=...)` | Share a node with an actor. |
| `async shared_with_me(tenant_id, actor, *, limit=..., offset=..., after_offset=..., trace_id=..., timeout=...)` | List nodes shared with the calling actor. |
| `tenant(tenant_id)` | Create a tenant-scoped handle. |
| `async transfer_ownership(node_id, new_owner, tenant_id, actor, *, trace_id=..., timeout=...)` | Transfer ownership of a node. |
| `async transfer_user_content(tenant_id, from_user, to_user, *, actor=..., timeout=...)` | Reassign ownership of all nodes from from_user to to_user. |
| `async update_user(user_id, *, name=..., email=..., status=..., actor=..., timeout=...)` | Update user fields. |
| `async wait_for_offset(tenant_id, stream_position, timeout_ms=..., *, trace_id=..., timeout=...)` | Wait for a stream position to be applied. |

## `Plan` methods

| Method | Description |
|---|---|
| `async commit(wait_applied=..., *, timeout=...)` | Commit the transaction. |
| `create(msg, *, acl=..., storage=..., as_=..., fanout_to=..., id_=...)` | Create a node from a proto message. |
| `delete(node_type, node_id)` | Delete a node. |
| `delete_where(node_type, where, *, limit=...)` | Delete every node of a type matching a predicate, in one op. |
| `edge_create(edge_type, from_id, to_id, *, props=...)` | Create an edge. |
| `edge_delete(edge_type, from_id, to_id)` | Delete an edge. |
| `update(node_id, msg, *, fields=..., precondition=...)` | Update a node from a proto message. |
