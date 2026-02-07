#!/bin/bash
# EntDB Demo Script
# Run: chmod +x demo.sh && ./demo.sh

set -e

echo "=== Cleaning up existing containers ==="
docker compose down -v 2>/dev/null || true

echo ""
echo "=== Starting EntDB services ==="
docker compose up -d

echo ""
echo "=== Waiting for services to be healthy ==="
echo "Waiting for Redpanda..."
until docker compose exec -T redpanda rpk cluster health 2>/dev/null; do
  sleep 2
done

echo "Waiting for EntDB server..."
until curl -sf http://localhost:8081/v1/health >/dev/null 2>&1; do
  sleep 2
done

echo ""
echo "=== Services are ready! ==="
echo ""
echo "Endpoints:"
echo "  - EntDB HTTP API: http://localhost:8081"
echo "  - EntDB gRPC:     localhost:50051"
echo "  - Redpanda Console: http://localhost:8080"
echo "  - MinIO Console:    http://localhost:9001 (minioadmin/minioadmin)"
echo ""

echo "=== Running Demo: Creating nodes and edges ==="
echo ""

# Demo 1: Create a User node
echo "1. Creating a User node..."
curl -s -X POST http://localhost:8081/v1/execute \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: demo_tenant" \
  -H "X-Actor: user:demo" \
  -d '{
    "operations": [
      {
        "create_node": {
          "type_id": 1,
          "data_json": "{\"email\": \"alice@example.com\", \"name\": \"Alice\"}",
          "as": "alice"
        }
      }
    ],
    "idempotency_key": "create_alice_001"
  }' | python3 -m json.tool

echo ""
echo "2. Creating a Task node..."
curl -s -X POST http://localhost:8081/v1/execute \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: demo_tenant" \
  -H "X-Actor: user:demo" \
  -d '{
    "operations": [
      {
        "create_node": {
          "type_id": 2,
          "data_json": "{\"title\": \"Review PR #123\", \"status\": \"todo\"}",
          "as": "task1"
        }
      }
    ],
    "idempotency_key": "create_task_001"
  }' | python3 -m json.tool

echo ""
echo "3. Creating an edge (Task -> AssignedTo -> User)..."
curl -s -X POST http://localhost:8081/v1/execute \
  -H "Content-Type: application/json" \
  -H "X-Tenant-ID: demo_tenant" \
  -H "X-Actor: user:demo" \
  -d '{
    "operations": [
      {
        "create_node": {
          "type_id": 2,
          "data_json": "{\"title\": \"Write docs\", \"status\": \"doing\"}",
          "as": "task2"
        }
      },
      {
        "create_edge": {
          "edge_id": 100,
          "from": {"alias_ref": "$task2.id"},
          "to": {"id": "user_alice_001"}
        }
      }
    ],
    "idempotency_key": "create_task_with_edge_001"
  }' | python3 -m json.tool

echo ""
echo "=== Demo: Querying nodes ==="
echo ""

echo "4. Query all nodes of type 1 (User)..."
curl -s "http://localhost:8081/v1/nodes?type_id=1&limit=10" \
  -H "X-Tenant-ID: demo_tenant" \
  -H "X-Actor: user:demo" | python3 -m json.tool

echo ""
echo "5. Query all nodes of type 2 (Task)..."
curl -s "http://localhost:8081/v1/nodes?type_id=2&limit=10" \
  -H "X-Tenant-ID: demo_tenant" \
  -H "X-Actor: user:demo" | python3 -m json.tool

echo ""
echo "=== Demo Complete ==="
echo ""
echo "Try these commands manually:"
echo ""
echo "  # Check health"
echo "  curl http://localhost:8081/v1/health"
echo ""
echo "  # Create a node"
echo '  curl -X POST http://localhost:8081/v1/execute \'
echo '    -H "Content-Type: application/json" \'
echo '    -H "X-Tenant-ID: my_tenant" \'
echo '    -H "X-Actor: user:me" \'
echo '    -d '\''{"operations": [{"create_node": {"type_id": 1, "data_json": "{\"email\": \"test@example.com\"}"}}], "idempotency_key": "my_unique_key"}'\'''
echo ""
echo "  # Query nodes"
echo '  curl "http://localhost:8081/v1/nodes?type_id=1" -H "X-Tenant-ID: my_tenant" -H "X-Actor: user:me"'
