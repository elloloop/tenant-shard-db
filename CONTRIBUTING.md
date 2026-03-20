# Contributing to EntDB

Thank you for considering contributing to EntDB! This document explains how to contribute effectively.

## Getting Started

1. Fork the repository
2. Clone your fork locally
3. Set up the development environment:
   ```bash
   pip install -e ".[dev]"
   make dev  # Start Docker Compose stack
   ```
4. Create a feature branch from `main`

## Development Workflow

### Running Tests

```bash
# Unit tests
pytest tests/unit -v

# Integration tests
pytest tests/integration -v

# E2E tests (requires Docker)
make e2e

# Playwright tests (requires stack running)
make playwright

# Benchmarks
pytest tests/benchmarks -v --benchmark-only
```

### Code Quality

```bash
# Lint
ruff check .
ruff format --check .

# Type check
mypy dbaas sdk --exclude '(_generated|api/generated)'
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

By contributing to EntDB, you agree that your contributions will be licensed under the [AGPL-3.0 License](LICENSE).
