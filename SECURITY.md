# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| main    | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

We take security seriously and appreciate your help in disclosing vulnerabilities responsibly.

### How to Report

1. **DO NOT** create a public GitHub issue for security vulnerabilities
2. Email security concerns to the repository maintainers
3. Include as much information as possible:
   - Type of vulnerability (e.g., SQL injection, XSS, authentication bypass)
   - Location of the affected code (file path, line numbers if known)
   - Steps to reproduce the vulnerability
   - Potential impact
   - Any suggested fixes (optional)

### What to Expect

- **Acknowledgment**: We will acknowledge receipt within 48 hours
- **Assessment**: We will assess the severity and impact within 7 days
- **Updates**: We will provide status updates at least weekly
- **Resolution**: We aim to resolve critical vulnerabilities within 30 days

## Safe Harbor

We consider security research conducted under this policy to be:

- **Authorized** concerning any applicable anti-hacking laws
- **Authorized** concerning any relevant anti-circumvention laws
- **Exempt** from restrictions in our Terms of Service that would interfere with conducting security research
- **Lawful, helpful to the overall security of the Internet, and conducted in good faith**

We will not pursue civil action or initiate a complaint to law enforcement for accidental, good-faith violations of this policy. We consider security research conducted consistent with this policy to be "authorized" conduct.

You are expected, as always, to comply with all applicable laws. If legal action is initiated by a third party against you and you have complied with this policy, we will take steps to make it known that your actions were conducted in compliance with this policy.

If at any time you have concerns or are uncertain whether your security research is consistent with this policy, please submit a report through one of our Official Channels before going any further.

### Severity Levels

| Severity | Description | Response Time |
|----------|-------------|---------------|
| Critical | Remote code execution, authentication bypass, data breach | 24-48 hours |
| High | Privilege escalation, significant data exposure | 7 days |
| Medium | Limited data exposure, denial of service | 30 days |
| Low | Minor information disclosure, hardening issues | 90 days |

## Security Measures

### Authentication & Authorization

- JWT-based authentication with configurable providers
- Role-based access control (RBAC)
- Session management with secure token handling
- LTI 1.3 integration for Canvas LMS

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

## Security Best Practices for Contributors

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

We regularly update dependencies to address security vulnerabilities:

- Go dependencies managed via `go.mod`
- JavaScript dependencies managed via `package.json`
- Container base images updated quarterly
- Security advisories monitored via GitHub Dependabot

## Compliance

This project aims to follow security best practices aligned with:

- OWASP Top 10
- CWE/SANS Top 25
- NIST Cybersecurity Framework

## Known Security Considerations

The following are architectural decisions with security implications that operators should understand:

### Checkpoint Script Execution

Lab templates can define bash scripts for checkpoint evaluation (objective detection). These scripts execute on the API server when evaluating student progress. **Only load lab templates from trusted sources.** Future versions may implement script sandboxing.

### TLS Verification for Proxmox

The `PROXMOX_INSECURE` environment variable controls TLS certificate verification for Proxmox API connections. **Always set `PROXMOX_INSECURE=false` in production.** The insecure option exists only for development environments using self-signed certificates.

### Demo Mode

The `AUTH_DEMO_MODE` environment variable bypasses authentication entirely. **Never enable demo mode in production.** Demo mode is intended only for local demonstrations and development. When enabled, all requests are treated as an authenticated demo user.

### Residual Advisories and Why They Are Not Remediated

Some advisories reported by Dependabot and `npm audit` survive the latest
dependency pass. Each is re-evaluated whenever dependencies are updated; none
is reachable from code that runs in production.

| Advisory | Package | Status |
|----------|---------|--------|
| [GO-2026-5932](https://pkg.go.dev/vuln/GO-2026-5932) | `golang.org/x/crypto` | **No fix exists.** The advisory covers `x/crypto/openpgp`, which is unmaintained. The API's own code uses only `x/crypto/bcrypt`, and `openpgp` is absent from `go list -deps`. |
| [GHSA-w3rx-r6r6-pgpr](https://github.com/advisories/GHSA-w3rx-r6r6-pgpr), [GHSA-5p2g-fcmc-qvqq](https://github.com/advisories/GHSA-5p2g-fcmc-qvqq) | `image-size` | **No fix exists** — every published version is affected, including the current 2.0.2. Denial of service via crafted ICNS/JXL/HEIF images. Pulled in by `@docusaurus/mdx-loader`, so `npm audit` reports it against all `@docusaurus/*` packages. Runs only while building the docs site, over images committed to this repository, and is absent from the generated static output. |
| [GHSA-w5hq-g745-h8pq](https://github.com/advisories/GHSA-w5hq-g745-h8pq) | `uuid` (via `sockjs`) | **Fixed upstream in `uuid` 11.1.1, but unreachable here:** `sockjs@0.3.24` is the newest published release and pins `uuid@^8`, so npm cannot resolve the fix. Not exploitable regardless — the advisory concerns the `v3()`/`v5()`/`v6()` generator functions when passed a `buf` argument, and `sockjs` calls `v4()` with no arguments. Reached only through `webpack-dev-server`, which runs during local docs development and is absent from every deployed artifact. |

Run `npm audit` in `docusaurus/` or `govulncheck ./...` in `api/` to re-check
whether upstream fixes have landed.

## Contact

For security-related questions or to report vulnerabilities:

- **Preferred**: Use GitHub's [private vulnerability reporting](https://github.com/toddbartholow/kootenai/security/advisories/new)
- **Alternative**: Contact the repository maintainers directly through GitHub

## Acknowledgments

We appreciate the security community's efforts in helping keep Kootenai secure. Researchers who responsibly disclose vulnerabilities will be acknowledged in our release notes (unless they prefer to remain anonymous).
