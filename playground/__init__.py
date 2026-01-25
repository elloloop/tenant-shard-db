"""
EntDB Playground - Interactive SDK Sandbox.

A write-enabled environment for developers to experiment with EntDB.
All data goes to a 'playground' tenant visible in Console.

Features:
- Define custom node types and edge types
- Create, update, delete data
- See generated Python/Go SDK code
- Bidirectional sync between form and code

Usage:
    uvicorn playground.app:app --port 8081

Or via Docker:
    docker compose up playground
"""
