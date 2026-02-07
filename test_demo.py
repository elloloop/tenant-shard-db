#!/usr/bin/env python3
"""
EntDB Demo Script - Tests the HTTP API

Run: python test_demo.py
"""

import json
import urllib.error
import urllib.request

BASE_URL = "http://localhost:8081"
TENANT_ID = "demo_tenant"
ACTOR = "user:demo"


def make_request(method, path, data=None):
    """Make an HTTP request to the EntDB API."""
    url = f"{BASE_URL}{path}"
    headers = {
        "X-Tenant-ID": TENANT_ID,
        "X-Actor": ACTOR,
        "Content-Type": "application/json",
    }

    body = json.dumps(data).encode() if data else None
    req = urllib.request.Request(url, data=body, headers=headers, method=method)

    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            return json.loads(resp.read().decode())
    except urllib.error.HTTPError as e:
        return {"error": str(e), "body": e.read().decode()}
    except urllib.error.URLError as e:
        return {"error": str(e)}


def check_health():
    """Check if the server is healthy."""
    print("=== Checking Health ===")
    try:
        req = urllib.request.Request(f"{BASE_URL}/v1/health")
        with urllib.request.urlopen(req, timeout=5) as resp:
            result = json.loads(resp.read().decode())
            print(json.dumps(result, indent=2))
            return True
    except Exception as e:
        print(f"Error: {e}")
        return False


def create_user():
    """Create a User node."""
    print("\n=== Creating User Node ===")
    result = make_request(
        "POST",
        "/v1/execute",
        {
            "operations": [
                {
                    "create_node": {
                        "type_id": 1,
                        "data_json": json.dumps({"email": "alice@example.com", "name": "Alice"}),
                        "as": "alice",
                    }
                }
            ],
            "idempotency_key": "create_alice_demo_001",
        },
    )
    print(json.dumps(result, indent=2))
    return result


def create_task():
    """Create a Task node."""
    print("\n=== Creating Task Node ===")
    result = make_request(
        "POST",
        "/v1/execute",
        {
            "operations": [
                {
                    "create_node": {
                        "type_id": 2,
                        "data_json": json.dumps({"title": "Review PR #123", "status": "todo"}),
                        "as": "task1",
                    }
                }
            ],
            "idempotency_key": "create_task_demo_001",
        },
    )
    print(json.dumps(result, indent=2))
    return result


def create_task_with_edge():
    """Create a Task with an edge to a user."""
    print("\n=== Creating Task with Edge ===")
    result = make_request(
        "POST",
        "/v1/execute",
        {
            "operations": [
                {
                    "create_node": {
                        "type_id": 2,
                        "data_json": json.dumps(
                            {"title": "Write documentation", "status": "doing"}
                        ),
                        "as": "task2",
                    }
                }
            ],
            "idempotency_key": "create_task_with_edge_demo_001",
        },
    )
    print(json.dumps(result, indent=2))
    return result


def query_users():
    """Query all User nodes."""
    print("\n=== Querying Users ===")
    result = make_request("GET", "/v1/nodes?type_id=1&limit=10")
    print(json.dumps(result, indent=2))
    return result


def query_tasks():
    """Query all Task nodes."""
    print("\n=== Querying Tasks ===")
    result = make_request("GET", "/v1/nodes?type_id=2&limit=10")
    print(json.dumps(result, indent=2))
    return result


def main():
    print("=" * 60)
    print("EntDB Demo")
    print("=" * 60)

    # Check health first
    if not check_health():
        print("\nServer is not healthy. Make sure docker compose is running:")
        print("  docker compose up -d")
        return

    # Run demos
    create_user()
    create_task()
    create_task_with_edge()
    query_users()
    query_tasks()

    print("\n" + "=" * 60)
    print("Demo Complete!")
    print("=" * 60)


if __name__ == "__main__":
    main()
