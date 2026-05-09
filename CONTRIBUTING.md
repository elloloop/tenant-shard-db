# Contributing to EntDB

Thank you for considering contributing to EntDB! This document explains how to contribute effectively.

## Getting Started

1. Fork the repository
2. Clone your fork locally
3. Set up the development environment:
   ```bash
   uv pip install -e ".[dev]"
   make dev  # Start Docker Compose stack
   ```
4. Create a feature branch from `main`

## Development Workflow

### Running Tests

```bash
# Unit tests
pytest tests/python/unit -v

# Integration tests
pytest tests/python/integration -v

# E2E tests (requires Docker)
make e2e

# Playwright tests (requires stack running)
make playwright

# Benchmarks
pytest tests/python/benchmarks -v --benchmark-only
```

### Code Quality

```bash
# Lint
ruff check .
ruff format --check .

# Type check
mypy server/python sdk/python --exclude '(_generated|api/generated)'
```

## Pull Request Process

1. Ensure all tests pass
2. Update documentation if needed
3. Follow existing code style and patterns
4. Include tests for new functionality
5. Reference any related GitHub issues

## Contributor License Agreement (CLA)

Before code contributions are accepted, contributors must sign the [Contributor License Agreement](CLA.md).

When opening your first pull request, you will be prompted to sign via GitHub, powered by [CLA Assistant](https://cla-assistant.io). You only need to sign once; future PRs are verified automatically.

## License

EntDB is **dual-licensed**. Your contribution will be licensed under the
license that applies to the directory you're contributing to:

| Where you're contributing | License | LICENSE file |
|---|---|---|
| Server: `server/python/`, `tests/python/` | **GNU AGPL-3.0** | repo-root [`LICENSE`](LICENSE) |
| Python SDK: `sdk/python/entdb_sdk/` | **MIT** | [`sdk/python/LICENSE`](sdk/python/LICENSE) |
| Go SDK + CLIs: `sdk/go/entdb/` (incl. `cmd/entdbctl/`, `cmd/entdb-console/`) | **MIT** | [`sdk/go/entdb/LICENSE`](sdk/go/entdb/LICENSE) |

By submitting a pull request, you agree your contribution is licensed under the
LICENSE file that lives nearest to the files you've changed. Source files
carry an `SPDX-License-Identifier:` header to make this explicit per file.
See the [README License section](README.md#license) for the rationale behind
the dual licensing.
