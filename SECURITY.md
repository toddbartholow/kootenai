# Security Policy

> ## This repository is archived. No vulnerability report will be triaged.
>
> Issues, pull requests and GitHub private vulnerability reporting are closed or
> disabled. There is no maintainer on call, no acknowledgment window, no severity
> triage and no patch release. Anything you report goes nowhere. Please do not
> spend your time writing one up for this repository.
>
> If you find something, the useful places to put it are a fork you or someone
> else is actually running, or a public write-up — this code is Apache 2.0 and you
> owe the original project nothing.
>
> **If you are running this code, you own its security outcomes entirely.** The
> rest of this file describes the posture as it stood when the repository was
> archived, so you know what you are inheriting. Read
> [Known Security Considerations](#known-security-considerations) before deploying
> anything; it is the part most likely to bite you.

## Supported Versions

None. No branch or tag is supported, and no further releases will be made.

## Security Posture at Archiving

Everything below describes what was true when the repository was archived. It is
a description, not a commitment — nothing here will be maintained, re-verified or
updated.

### Authentication & Authorization

- JWT-based authentication with configurable providers
- Role-based access control (RBAC)
- Session management with secure token handling
- LTI 1.3 integration for Canvas LMS — implemented, but gated off in this build
  (see "Known gaps" in `README.md`), so it is not part of any running attack surface
  unless a fork opens the gate

### Data Protection

- All sensitive data encrypted at rest and in transit
- TLS 1.2+ required for all external communications
- Database credentials stored securely (environment variables or vault)
- No secrets committed to version control

### Input Validation

- All user inputs validated and sanitized
- SQL injection prevention via parameterized queries
- XSS protection through proper output encoding
- CSRF protection on state-changing operations

### Network Security

- CORS configured with explicit origin allowlist
- Rate limiting on all API endpoints
- Pod network isolation (student environments are isolated)
- WebSocket connections require authentication

### Security Scanning

Our CI/CD pipeline includes:

- **govulncheck**: Go dependency vulnerability scanning
- **gosec**: Go static security analysis
- **npm audit**: JavaScript dependency vulnerability scanning
- **Trivy**: Container and filesystem vulnerability scanning

### Audit Logging

Security-relevant events are logged including:

- Authentication attempts (success/failure)
- Authorization decisions
- Configuration changes
- Administrative actions

## Security Practices for Forks

These were the project's own review conventions. They are reproduced because they
are still the right habits for anyone continuing the code, not because a review
awaits you here.

### Code Review Checklist

- [ ] No hardcoded credentials or secrets
- [ ] Input validation on all user-supplied data
- [ ] Proper error handling (no sensitive info in errors)
- [ ] Authentication/authorization checks in place
- [ ] SQL queries use parameterized statements
- [ ] Output encoding for user-controlled content
- [ ] Secure defaults (deny by default)
- [ ] Dependencies from trusted sources with pinned versions

### Common Vulnerabilities to Avoid

1. **Command Injection**: Never pass user input directly to shell commands
2. **SQL Injection**: Always use parameterized queries
3. **XSS**: Encode output, use Content Security Policy
4. **CSRF**: Use anti-CSRF tokens for state changes
5. **Path Traversal**: Validate and sanitize file paths
6. **Insecure Deserialization**: Validate all deserialized data
7. **SSRF**: Validate and restrict outbound requests
8. **Secrets in Code**: Use environment variables or secret management

### Running Security Scans Locally

```bash
# Go security scans
cd api
go install golang.org/x/vuln/cmd/govulncheck@latest
go install github.com/securego/gosec/v2/cmd/gosec@latest
govulncheck ./...
gosec ./...

# JavaScript security scans
cd web
npm audit

# Full security audit via Mage
mage security:all
```

## Third-Party Dependencies

Dependencies are pinned in `api/go.mod`, `web/package.json` and
`docusaurus/package.json`, and were current when the repository was archived. **They
are frozen there and will drift out of date from the moment you read this.** A fork's
first security task is a full dependency pass; assume every pin is stale and that
advisories have landed since.

## Compliance

The project was written against these references. It has never been formally audited
or certified against any of them, and no such work is planned:

- OWASP Top 10
- CWE/SANS Top 25
- NIST Cybersecurity Framework

## Known Security Considerations

The following are architectural decisions with security implications that operators should understand:

### Checkpoint Script Execution

Lab templates can define bash scripts for checkpoint evaluation (objective detection). These scripts execute on the API server when evaluating student progress. **Only load lab templates from trusted sources.** The scripts are not sandboxed and will not be here, because nothing further ships from this repository. A fork that needs to run untrusted templates has to add that isolation itself.

### TLS Verification for Proxmox

The `PROXMOX_INSECURE` environment variable controls TLS certificate verification for Proxmox API connections. **Always set `PROXMOX_INSECURE=false` in production.** The insecure option exists only for development environments using self-signed certificates.

### Demo Mode

The `AUTH_DEMO_MODE` environment variable bypasses authentication entirely. **Never enable demo mode in production.** Demo mode is intended only for local demonstrations and development. When enabled, all requests are treated as an authenticated demo user.

### Residual Advisories and Why They Are Not Remediated

Some advisories reported by Dependabot and `npm audit` survived the last dependency
pass before archiving. Each had been evaluated and none was reachable from code that
runs in production. This analysis is a snapshot — re-do it against your own dependency
tree rather than trusting it.

| Advisory | Package | Status |
|----------|---------|--------|
| [GO-2026-5932](https://pkg.go.dev/vuln/GO-2026-5932) | `golang.org/x/crypto` | **No fix exists.** The advisory covers `x/crypto/openpgp`, which is unmaintained. The API's own code uses only `x/crypto/bcrypt`, and `openpgp` is absent from `go list -deps`. |
| [GHSA-w3rx-r6r6-pgpr](https://github.com/advisories/GHSA-w3rx-r6r6-pgpr), [GHSA-5p2g-fcmc-qvqq](https://github.com/advisories/GHSA-5p2g-fcmc-qvqq) | `image-size` | **No fix exists** — every published version is affected, including the current 2.0.2. Denial of service via crafted ICNS/JXL/HEIF images. Pulled in by `@docusaurus/mdx-loader`, so `npm audit` reports it against all `@docusaurus/*` packages. Runs only while building the docs site, over images committed to this repository, and is absent from the generated static output. |
| [GHSA-w5hq-g745-h8pq](https://github.com/advisories/GHSA-w5hq-g745-h8pq) | `uuid` (via `sockjs`) | **Fixed upstream in `uuid` 11.1.1, but unreachable here:** `sockjs@0.3.24` is the newest published release and pins `uuid@^8`, so npm cannot resolve the fix. Not exploitable regardless — the advisory concerns the `v3()`/`v5()`/`v6()` generator functions when passed a `buf` argument, and `sockjs` calls `v4()` with no arguments. Reached only through `webpack-dev-server`, which runs during local docs development and is absent from every deployed artifact. |

Run `npm audit` in `docusaurus/` or `govulncheck ./...` in `api/` to re-check
whether upstream fixes have landed.
