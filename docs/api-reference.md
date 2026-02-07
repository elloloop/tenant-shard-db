# API Reference

EntDB exposes both gRPC and HTTP APIs.

## gRPC API

### Service Definition

```protobuf
service EntDBService {
  // Execute atomic transaction
  rpc ExecuteAtomic(ExecuteAtomicRequest) returns (ExecuteAtomicResponse);

  // Get single node
  rpc GetNode(GetNodeRequest) returns (GetNodeResponse);

  // Query nodes by type
  rpc QueryNodes(QueryNodesRequest) returns (QueryNodesResponse);

  // Get edges from/to node
  rpc GetEdges(GetEdgesRequest) returns (GetEdgesResponse);

  // Search mailbox
  rpc SearchMailbox(SearchMailboxRequest) returns (SearchMailboxResponse);

  // Health check
  rpc Health(HealthRequest) returns (HealthResponse);

  // Get schema
  rpc GetSchema(GetSchemaRequest) returns (GetSchemaResponse);
}
```

### ExecuteAtomic

Execute atomic transaction:

```protobuf
message ExecuteAtomicRequest {
  string tenant_id = 1;
  string actor = 2;
  string idempotency_key = 3;
  repeated Operation operations = 4;
}

message Operation {
  oneof operation {
    CreateNode create_node = 1;
    UpdateNode update_node = 2;
    DeleteNode delete_node = 3;
    CreateEdge create_edge = 4;
    DeleteEdge delete_edge = 5;
    SetVisibility set_visibility = 6;
  }
}

message ExecuteAtomicResponse {
  repeated OperationResult results = 1;
  string wal_position = 2;
}
```

### GetNode

Get single node by ID:

```protobuf
message GetNodeRequest {
  string tenant_id = 1;
  string actor = 2;
  string node_id = 3;
  bool include_deleted = 4;
}

message GetNodeResponse {
  Node node = 1;
}

message Node {
  string id = 1;
  int32 type_id = 2;
  string tenant_id = 3;
  bytes payload_json = 4;
  string owner_actor = 5;
  int64 created_at = 6;
  int64 updated_at = 7;
  bool deleted = 8;
  int64 version = 9;
}
```

### QueryNodes

Query nodes by type with filters:

```protobuf
message QueryNodesRequest {
  string tenant_id = 1;
  string actor = 2;
  int32 type_id = 3;
  bytes filters_json = 4;
  int32 limit = 5;
  int32 offset = 6;
}

message QueryNodesResponse {
  repeated Node nodes = 1;
  int32 total_count = 2;
}
```

## HTTP API

### Base URL

```
http://localhost:8080/v1
```

### Authentication

All requests require tenant and actor headers:

```http
X-Tenant-ID: my_tenant
X-Actor: user:alice
```

Or query parameters:

```
?tenant_id=my_tenant&actor=user:alice
```

### Endpoints

#### POST /v1/atomic

Execute atomic transaction.

**Request:**
```json
{
  "tenant_id": "my_tenant",
  "actor": "user:alice",
  "idempotency_key": "create_user_123",
  "operations": [
    {
      "op": "create_node",
      "alias": "user1",
      "type_id": 1,
      "payload": {
        "email": "alice@example.com",
        "name": "Alice"
      },
      "principals": ["user:alice", "tenant:*"]
    }
  ]
}
```

**Response:**
```json
{
  "results": [
    {
      "node_id": "nd_abc123",
      "alias": "user1"
    }
  ],
  "wal_position": "0:12345"
}
```

#### GET /v1/nodes/:id

Get single node.

**Request:**
```
GET /v1/nodes/nd_abc123?tenant_id=my_tenant&actor=user:alice
```

**Response:**
```json
{
  "id": "nd_abc123",
  "type_id": 1,
  "tenant_id": "my_tenant",
  "payload": {
    "email": "alice@example.com",
    "name": "Alice"
  },
  "owner_actor": "user:alice",
  "created_at": 1705312000000,
  "updated_at": 1705312000000,
  "deleted": false,
  "version": 1
}
```

#### GET /v1/nodes

Query nodes by type.

**Request:**
```
GET /v1/nodes?tenant_id=my_tenant&actor=user:alice&type_id=1&limit=10&offset=0
```

**Response:**
```json
{
  "nodes": [
    {"id": "nd_abc123", "type_id": 1, ...},
    {"id": "nd_def456", "type_id": 1, ...}
  ],
  "total_count": 42
}
```

#### GET /v1/nodes/:id/edges/out

Get outgoing edges from node.

**Request:**
```
GET /v1/nodes/nd_abc123/edges/out?tenant_id=my_tenant&actor=user:alice&edge_type=100
```

**Response:**
```json
{
  "edges": [
    {
      "id": "ed_xyz789",
      "edge_type_id": 100,
      "from_id": "nd_abc123",
      "to_id": "nd_def456"
    }
  ]
}
```

#### GET /v1/nodes/:id/edges/in

Get incoming edges to node.

#### GET /v1/mailbox

Get user mailbox items.

**Request:**
```
GET /v1/mailbox?tenant_id=my_tenant&user_id=alice&limit=20&offset=0
```

**Response:**
```json
{
  "items": [
    {
      "node_id": "nd_msg001",
      "node_type_id": 3,
      "preview_text": "Meeting tomorrow at 3pm",
      "is_read": false,
      "created_at": 1705312000000
    }
  ],
  "unread_count": 5
}
```

#### GET /v1/mailbox/search

Search mailbox with FTS.

**Request:**
```
GET /v1/mailbox/search?tenant_id=my_tenant&user_id=alice&query=meeting
```

**Response:**
```json
{
  "results": [
    {
      "node_id": "nd_msg001",
      "preview_text": "Meeting tomorrow at 3pm",
      "score": 0.95
    }
  ]
}
```

#### POST /v1/mailbox/mark_read

Mark mailbox item as read.

**Request:**
```json
{
  "tenant_id": "my_tenant",
  "user_id": "alice",
  "node_id": "nd_msg001"
}
```

#### GET /v1/schema

Get current schema.

**Response:**
```json
{
  "fingerprint": "sha256:abc123...",
  "node_types": [
    {
      "type_id": 1,
      "name": "User",
      "fields": [
        {"field_id": 1, "name": "email", "kind": "string", "required": true}
      ]
    }
  ],
  "edge_types": [
    {
      "edge_id": 100,
      "name": "AssignedTo",
      "from_type": 2,
      "to_type": 1
    }
  ]
}
```

#### GET /health

Health check.

**Response:**
```json
{
  "status": "healthy",
  "components": {
    "wal": "connected",
    "sqlite": "ok",
    "s3": "ok"
  },
  "applier_lag_ms": 50,
  "version": "1.0.0"
}
```

### Error Responses

All errors follow this format:

```json
{
  "error": {
    "code": "NOT_FOUND",
    "message": "Node not found: nd_abc123",
    "details": {
      "node_id": "nd_abc123"
    }
  }
}
```

Error codes:

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `INVALID_REQUEST` | 400 | Malformed request |
| `VALIDATION_ERROR` | 400 | Payload validation failed |
| `UNAUTHORIZED` | 401 | Missing authentication |
| `FORBIDDEN` | 403 | ACL denied access |
| `NOT_FOUND` | 404 | Resource not found |
| `CONFLICT` | 409 | Optimistic lock conflict |
| `INTERNAL_ERROR` | 500 | Server error |
| `SERVICE_UNAVAILABLE` | 503 | WAL unavailable |

### Rate Limiting

Headers in response:

```http
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 999
X-RateLimit-Reset: 1705312060
```

When rate limited:

```http
HTTP/1.1 429 Too Many Requests
Retry-After: 60
```

### Pagination

List endpoints support pagination:

```
?limit=20&offset=0
```

Response includes total count:

```json
{
  "items": [...],
  "total_count": 150
}
```

### Filtering

Query nodes with filters:

```
GET /v1/nodes?type_id=1&filter.status=active&filter.created_at.gte=1705000000000
```

Filter operators:
- `eq` (default): Exact match
- `neq`: Not equal
- `gt`: Greater than
- `gte`: Greater than or equal
- `lt`: Less than
- `lte`: Less than or equal
- `in`: In list (comma-separated)
- `like`: SQL LIKE pattern
