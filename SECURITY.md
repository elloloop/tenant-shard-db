# Security policy

## Supported versions

Security fixes target the **latest minor** of the currently published
major. Older minors get patches only when a fix is mechanical and
low-risk; everything else is "upgrade to the latest minor".

| Version | Supported |
|---------|-----------|
| v2.1.x  | ✅ Active |
| v2.0.x  | ⚠️ Critical / high-severity fixes only |
| < v2.0  | ❌ End-of-life |

The Go SDK, Python SDK, and server image are versioned together —
the policy above applies to all three.

## Reporting a vulnerability

**Do not open a public GitHub issue for a suspected vulnerability.**

Use GitHub's private vulnerability reporting:

  → https://github.com/elloloop/tenant-shard-db/security/advisories/new

This routes a private advisory to the maintainers and (when accepted)
lets us coordinate the fix + the CVE + the disclosure timeline with
you in the same thread.

If you cannot use that channel, email **arun88m@gmail.com** with the
subject prefix `[entdb-security]`. Include:

  - Affected component (server / Go SDK / Python SDK / `entdb-schema` /
    `protoc-gen-entdb-keys`) and version.
  - A minimal reproduction (config, RPC sequence, payload).
  - Your assessment of impact (data exposure / corruption / DoS / RCE).

## What to expect

  - **Acknowledgement** within 72 hours.
  - **Initial assessment** (accepted / declined / need-more-info)
    within 7 days.
  - **Fix + coordinated disclosure** target: 30 days for
    high/critical, 90 days for medium/low. We'll request more time
    explicitly if a fix needs cross-component coordination (server
    + both SDKs).
  - **Credit**: we publish a security advisory with your name (or a
    handle you choose) once a fixed release ships, unless you'd
    prefer to stay anonymous.

## Out of scope

  - Reports against unsupported versions (see table above) without a
    matching reproduction on a supported version.
  - Findings against the development branch that have already been
    fixed on `main` but not yet released.
  - Self-imposed denial-of-service from misconfiguration (e.g.
    setting `--apply-concurrency=0` on a single-tenant deployment).
  - Issues that only apply to running the server without TLS or
    OAuth — these are documented as `for local/dev use only` and the
    server logs warnings at boot.

## Automated security tooling

The CI pipeline (`.github/workflows/ci.yml`) runs on every PR:

  - **Trivy** filesystem scan — fails the build on
    `CRITICAL` / `HIGH` severity findings (`security-scan` job).
  - **`govulncheck`** — Go's call-graph-aware CVE scanner against
    every Go module (`go-vuln` job).
  - **`pip-audit`** — Python dependency scanner against the SDK +
    test extras (`python-vuln` job).
  - **CodeQL** — semantic SAST for Go + Python (separate workflow,
    weekly + per-PR).
  - **Dependabot** — weekly version updates + always-on security
    updates for pip / docker / github-actions / four Go modules.

Container images published to `ghcr.io/elloloop/tenant-shard-db` are
built multi-arch (amd64 + arm64) from a distroless base.
