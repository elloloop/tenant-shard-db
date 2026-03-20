# Review — Principal Engineer Self-Review

Review all code written in this session as if you are a principal software engineer. This is a blocking gate.

Run this automatically after writing code, or manually via `/review`.

## Steps

### 1. Identify all changes

```bash
git diff --name-only HEAD
git diff --cached --name-only
git diff main...HEAD --name-only
```

### 2. Test existence check (BLOCKING)

```bash
SOURCE_FILES=$(git diff main...HEAD --name-only | grep -v '_test\.\|test_\|\.test\.\|\.spec\.\|__tests__\|/tests/\|/test/\|_generated\|generated\|\.config\.\|\.json$\|\.yaml$\|\.yml$\|\.md$\|\.lock$\|\.toml$' | wc -l)
TEST_FILES=$(git diff main...HEAD --name-only | grep -E '_test\.|test_|\.test\.|\.spec\.|/tests/|/test/' | wc -l)
```

**BLOCKING rules:**
- Source files > 0 and test files == 0 → **REJECT**. Write missing tests first.
- For Python changes: must have pytest tests
- For frontend changes: must have Playwright tests
- Verify test files actually import and test the changed modules

### 3. Read every changed file

For each file, evaluate against:

**Code Quality:**
- Single Responsibility — each function does one thing
- No duplicated logic (but don't over-abstract)
- Code reads like prose — clear naming, shallow nesting
- Comments explain WHY, not WHAT

**Security:**
- No SQL injection (parameterized queries — EntDB uses raw sqlite3)
- No hardcoded secrets
- Auth/ACL checks present where needed
- Input validation at gRPC/REST boundaries

**Error Handling:**
- Errors handled, not swallowed
- Error messages include context
- Resources cleaned up (connections, files)
- External system errors wrapped with context

**Performance:**
- No N+1 query patterns in SQLite stores
- No unbounded loops
- Lists paginated
- No loading entire tables into memory

**Correctness:**
- No race conditions (asyncio coordination)
- Null/None cases handled
- Edge cases: empty lists, zero values, missing tenants
- Timeouts on external calls (gRPC, Kafka, S3)
- Idempotency maintained (EntDB's core guarantee)

**Test Coverage:**
- Unit tests: every public function, edge cases, fast
- Integration tests: real SQLite, real gRPC calls
- Playwright tests: for any console/playground UI changes
- E2E tests: critical user journeys through the full stack

### 4. Fix issues found

Fix immediately. Note what was found and fixed.

### 5. Report

```
## Principal Engineer Review

**Files reviewed**: <count>

### Issues found and fixed
- <file:line> — <what was wrong> → <how fixed>

### Checklist
- Code quality: Passed
- Security: Passed
- Error handling: Passed
- Performance: Passed
- Correctness: Passed
- Test coverage: Passed

**Verdict**: Approved / Issues remain
```

## Important
- This is NOT optional. Every piece of code must pass before commit.
- Be genuinely critical — actually read every line.
- Missing test coverage is a rejection.
- The test existence check is a HARD GATE.
