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
