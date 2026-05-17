<!-- GENERATED FILE — do not edit by hand.
     Regenerate with: python scripts/generate_api_docs.py
     Source of truth is the proto + SDK code, not this file. -->

# API Reference — gRPC contract (generated)

> Internal transport. Application code MUST use an SDK (`pip install entdb-sdk` / `go get .../sdk/go/entdb`). This page is the machine-extracted RPC inventory; the editorial narrative lives in [api-reference.md](../api-reference.md).

Source: [`proto/entdb/v1/entdb.proto`](../../proto/entdb/v1/entdb.proto). **44 RPCs.**

| RPC | Request | Response | Description |
|---|---|---|---|
| `AddGroupMember` | `GroupMemberRequest` | `GroupMemberResponse` | ACL v2 - Add a member to a group |
| `AddTenantMember` | `TenantMemberRequest` | `TenantMemberResponse` | Tenant membership management |
| `ArchiveTenant` | `ArchiveTenantRequest` | `ArchiveTenantResponse` | — |
| `CancelUserDeletion` | `CancelUserDeletionRequest` | `CancelUserDeletionResponse` | — |
| `ChangeMemberRole` | `ChangeMemberRoleRequest` | `ChangeMemberRoleResponse` | — |
| `CreateTenant` | `CreateTenantRequest` | `CreateTenantResponse` | Tenant registry CRUD |
| `CreateUser` | `CreateUserRequest` | `CreateUserResponse` | User registry CRUD |
| `DelegateAccess` | `DelegateAccessRequest` | `DelegateAccessResponse` | — |
| `DeleteUser` | `DeleteUserRequest` | `DeleteUserResponse` | GDPR operations (Issue #103, ADR-004) |
| `ExecuteAtomic` | `ExecuteAtomicRequest` | `ExecuteAtomicResponse` | Execute an atomic transaction containing one or more operations Returns a receipt that can be used to check application status |
| `ExportUserData` | `ExportUserDataRequest` | `ExportUserDataResponse` | — |
| `FreezeUser` | `FreezeUserRequest` | `FreezeUserResponse` | — |
| `GetConnectedNodes` | `GetConnectedNodesRequest` | `GetConnectedNodesResponse` | ACL v2 - Get connected nodes with ACL filtering |
| `GetEdgesFrom` | `GetEdgesRequest` | `GetEdgesResponse` | Get outgoing edges from a node |
| `GetEdgesTo` | `GetEdgesRequest` | `GetEdgesResponse` | Get incoming edges to a node |
| `GetMailbox` | `GetMailboxRequest` | `GetMailboxResponse` | Get mailbox items for a user |
| `GetNode` | `GetNodeRequest` | `GetNodeResponse` | Get a node by ID |
| `GetNodeByKey` | `GetNodeByKeyRequest` | `GetNodeByKeyResponse` | Unique-key secondary lookup (2026-04-13 unique_keys decision) |
| `GetNodes` | `GetNodesRequest` | `GetNodesResponse` | Get multiple nodes by IDs |
| `GetReceiptStatus` | `GetReceiptStatusRequest` | `GetReceiptStatusResponse` | Get the status of a previously submitted transaction |
| `GetSchema` | `GetSchemaRequest` | `GetSchemaResponse` | Get schema information |
| `GetTenant` | `GetTenantRequest` | `GetTenantResponse` | — |
| `GetTenantMembers` | `GetTenantMembersRequest` | `GetTenantMembersResponse` | — |
| `GetTenantQuota` | `GetTenantQuotaRequest` | `GetTenantQuotaResponse` | Quota dashboard (ADR: docs/decisions/quotas.md) |
| `GetUser` | `GetUserRequest` | `GetUserResponse` | — |
| `GetUserTenants` | `GetUserTenantsRequest` | `GetUserTenantsResponse` | — |
| `Health` | `HealthRequest` | `HealthResponse` | Get server health and status |
| `ListMailboxUsers` | `ListMailboxUsersRequest` | `ListMailboxUsersResponse` | List mailbox users for a tenant |
| `ListSharedWithMe` | `ListSharedWithMeRequest` | `ListSharedWithMeResponse` | ACL v2 - List nodes shared with the caller |
| `ListTenants` | `ListTenantsRequest` | `ListTenantsResponse` | List all tenants that have data |
| `ListUsers` | `ListUsersRequest` | `ListUsersResponse` | — |
| `QueryNodes` | `QueryNodesRequest` | `QueryNodesResponse` | Query nodes by type with optional filters |
| `RemoveGroupMember` | `GroupMemberRequest` | `GroupMemberResponse` | ACL v2 - Remove a member from a group |
| `RemoveTenantMember` | `TenantMemberRequest` | `TenantMemberResponse` | — |
| `RevokeAccess` | `RevokeAccessRequest` | `RevokeAccessResponse` | ACL v2 - Revoke access from an actor |
| `RevokeAllUserAccess` | `RevokeAllUserAccessRequest` | `RevokeAllUserAccessResponse` | — |
| `SearchMailbox` | `SearchMailboxRequest` | `SearchMailboxResponse` | Search mailbox with full-text search |
| `SearchNodes` | `SearchNodesRequest` | `SearchNodesResponse` | Full-text search across searchable fields (2026-04-19 FTS decision) |
| `SetLegalHold` | `LegalHoldRequest` | `LegalHoldResponse` | — |
| `ShareNode` | `ShareNodeRequest` | `ShareNodeResponse` | ACL v2 - Share a node with an actor |
| `TransferOwnership` | `TransferOwnershipRequest` | `TransferOwnershipResponse` | ACL v2 - Transfer ownership of a node |
| `TransferUserContent` | `TransferUserContentRequest` | `TransferUserContentResponse` | Admin operations (Issue #90, ADR-003) |
| `UpdateUser` | `UpdateUserRequest` | `UpdateUserResponse` | — |
| `WaitForOffset` | `WaitForOffsetRequest` | `WaitForOffsetResponse` | Wait for a specific stream position to be applied |

## `ExecuteAtomic` operations

The `ExecuteAtomic` RPC carries a list of `Operation`s (the `oneof op`). Each op below is a member of that union — they are wire contract, not standalone RPCs. `delete_where` is the single-RPC predicate sweeper (issues #504, #545).

| Op | Message | Description |
|---|---|---|
| `create_edge` | `CreateEdgeOp` | — |
| `create_node` | `CreateNodeOp` | — |
| `delete_edge` | `DeleteEdgeOp` | — |
| `delete_node` | `DeleteNodeOp` | — |
| `delete_where` | `DeleteWhereOp` | DeleteWhereOp is a single-RPC predicate-based sweeper: it deletes every node of ``type_id`` whose payload matches ALL of the ``where`` predicates, inside one ``ExecuteAtomic`` op. It collapses the standard "QueryNodes to find ids, then ExecuteAtomic to delete them" loop into a single round trip — the TTL-sweeper pattern. See GitHub issue #504. The ``where`` predicates reuse the exact ``FieldFilter`` / ``FilterOp`` types shipped for ``QueryNodes`` (issue #501), so no new wire concept is introduced — Eq/Ne/Lt/Le/Gt/Ge are supported and AND-ed together. Limit semantics are BEST-EFFORT (Postgres ``DELETE … LIMIT`` style): at most ``limit`` matching nodes are deleted in this op; the rest remain for a subsequent sweep. This matches sweeper semantics — a caller polls until a sweep deletes zero rows. The server caps ``limit`` to a server-side maximum so a runaway predicate cannot pin the single applier goroutine for one tenant (CLAUDE.md single-applier invariant). ``limit <= 0`` means "use the server default cap". Out of scope for v1 (issue #504): the deleted ids are NOT returned in the response — callers that need the ids keep using the QueryNodes + DeleteNodeOp loop. |
| `update_node` | `UpdateNodeOp` | — |
