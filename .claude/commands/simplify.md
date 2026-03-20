# Simplify — Review Changed Code for Quality

Review all changed code for reuse opportunities, quality issues, and efficiency improvements, then fix any issues found.

## Steps

### 1. Identify changes

```bash
git diff main...HEAD --name-only
```

### 2. Read each changed file

For each file, look for:

**Reuse opportunities:**
- Duplicated logic that could be extracted
- Patterns that match existing utilities in the codebase
- Code that reimplements something already available

**Quality issues:**
- Overly complex functions (can they be simplified?)
- Deep nesting (can early returns flatten it?)
- Unclear naming
- Dead code or commented-out code

**Efficiency:**
- Unnecessary allocations or copies
- N+1 patterns in database queries
- Missing indexes for query patterns
- Redundant API calls

### 3. Fix issues

For each issue found, fix it immediately.

### 4. Verify

```bash
ruff check .
mypy dbaas sdk --exclude '(_generated|api/generated)'
pytest tests/unit -v
```

### 5. Report

Summarize what was found and fixed.
