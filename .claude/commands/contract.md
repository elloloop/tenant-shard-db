# Define Proto Contract

Update the EntDB protobuf contract. This is always done FIRST before any server, SDK, or frontend work.

EntDB uses a single proto file at `dbaas/entdb_server/api/proto/entdb.proto` with generation via `./scripts/generate_proto.sh`.

## Arguments

$ARGUMENTS: optional description of what to add.
- `/contract` → infer from issue
- `/contract Add ListTenants RPC`

## Steps

### 1. Read context

- Read issue from `.claude/issue`
- `gh issue view <number>` to understand what needs to be built
- Read the current proto:

```bash
cat dbaas/entdb_server/api/proto/entdb.proto
```

### 2. Update the proto

Edit `dbaas/entdb_server/api/proto/entdb.proto`:

- Add new RPCs to the `EntDBService` service definition
- Add new message types as needed
- Follow existing patterns in the file
- Use clear, descriptive names
- Include comments explaining each new RPC and message
- ONLY add — never modify or remove existing RPCs (backward compatibility)

### 3. Generate code

```bash
./scripts/generate_proto.sh
```

This generates:
- `dbaas/entdb_server/api/generated/entdb_pb2.py`
- `dbaas/entdb_server/api/generated/entdb_pb2_grpc.py`
- `sdk/entdb_sdk/_generated/entdb_pb2.py`
- `sdk/entdb_sdk/_generated/entdb_pb2_grpc.py`

### 4. Verify generation

```bash
python -c "from dbaas.entdb_server.api.generated import entdb_pb2; print('Server proto OK')"
python -c "from entdb_sdk._generated import entdb_pb2; print('SDK proto OK')"
```

### 5. Build the contract spec

Document what was added:

```
### Contract Update

**Proto file**: `dbaas/entdb_server/api/proto/entdb.proto`

#### New RPCs
| RPC | Request | Response | Description |
|---|---|---|---|
| <Name> | <Request> | <Response> | <Description> |

#### New Messages
<list new message types with their fields>

#### Generated code locations
| Component | Path |
|---|---|
| Server | `dbaas/entdb_server/api/generated/` |
| SDK | `sdk/entdb_sdk/_generated/` |
```

### 6. Commit

```bash
git add dbaas/entdb_server/api/proto/ dbaas/entdb_server/api/generated/ sdk/entdb_sdk/_generated/
git commit -m "Add <description> to EntDB proto contract"
```

### 7. Propagate to component sub-issues

Find component sub-issues from the parent issue and comment with the full contract spec so each agent knows what to implement.

### 8. Report

Show the proto changes. Suggest `/pr` to create a PR, then start component work after merge.

## Important
- The proto IS the API design — take time to get it right
- ONLY add, never modify or remove existing RPCs
- Always regenerate after proto changes
- Schema evolution rules in `dbaas/entdb_server/schema/compat.py` apply
- Generated code in `_generated/` directories is never hand-edited
