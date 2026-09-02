# Version Tracking & Smart Rebuild System

This document describes the version tracking infrastructure that enables the build system to know what version of each component is running on the infra VM, allowing it to selectively rebuild only components that have changed.

## Overview

The system tracks versions across multiple layers:

| Component | Version Source | Query Method |
|-----------|---------------|--------------|
| API binary | Git commit + ldflags | `/version` endpoint |
| Web UI | Git commit + build manifest | `version.json` in container |
| DB Schema | Migration version | `schema_migrations` table |
| PostgreSQL | Server version | `SELECT version()` |
| NATS | Binary version | `nats-server --version` |
| Nginx | Binary version | `nginx -v` |
| Redis | Server version | `redis-cli INFO server` |

## Implementation Details

### Go Binary Version Embedding

The API binary embeds version information at build time using Go's linker flags (`ldflags`):

```go
// api/cmd/labctl/main.go
var (
    Version   = "dev"      // Set via: -X main.Version=...
    Commit    = "unknown"  // Set via: -X main.Commit=...
    BuildTime = "unknown"  // Set via: -X main.BuildTime=...
)
```

The Mage build system (in `magefiles/build.go`) extracts this information:

```go
version := sh.Output("git", "describe", "--tags", "--always")
commit := sh.Output("git", "rev-parse", "--short", "HEAD")
buildTime := time.Now().Format(time.RFC3339)

ldflags := fmt.Sprintf("-s -w -X main.Version=%s -X main.Commit=%s -X main.BuildTime=%s",
    version, commit, buildTime)
```

### Version API Endpoint

The API exposes version information via a public endpoint:

```
GET /version
```

Response:
```json
{
  "version": "0.2.1",
  "commit": "abc1234",
  "build_time": "2024-12-24T12:00:00Z",
  "go_version": "go1.22.0"
}
```

This endpoint is unauthenticated (like `/health`) for monitoring tool compatibility.

### Version Manifest File

A `.versions.json` file on the infra VM tracks all deployed component versions:

```json
{
  "api": {
    "version": "0.2.1",
    "commit": "abc1234",
    "build_time": "2024-12-24T12:00:00Z",
    "deployed_at": "2024-12-24T12:05:00Z"
  },
  "web": {
    "version": "0.2.1",
    "commit": "abc1234",
    "build_time": "2024-12-24T12:00:00Z",
    "deployed_at": "2024-12-24T12:05:00Z"
  },
  "db_schema": {
    "version": "20241224120000",
    "migration_count": 15
  },
  "infrastructure": {
    "postgresql": "15.4",
    "nats": "2.10.0",
    "nginx": "1.24.0",
    "redis": "7.2.0"
  }
}
```

Location: `/home/labadmin/kootenai/.versions.json`

### Smart Rebuild Logic

The default `mage deploy:api` behavior:

1. Get local commit hash (`git rev-parse HEAD`)
2. SSH to infra VM, read `.versions.json`
3. Compare API commit hashes
4. If same: skip build, print "API already up to date (commit abc1234)"
5. If different: build and deploy
6. Update `.versions.json` with new version info

To force a rebuild regardless of version:
```bash
mage deploy:force
```

## Mage Targets

| Target | Description |
|--------|-------------|
| `mage deploy:versions` | Display all component versions from infra VM |
| `mage deploy:api` | Smart deploy - skips if version matches |
| `mage deploy:web` | Smart deploy for web container |
| `mage deploy:force` | Force rebuild all components |
| `mage deploy:status` | Show service status including versions |

## CLI Commands

```bash
# Show embedded version info
./bin/labctl version

# Output:
# labctl version 0.2.1
# Commit: abc1234
# Built: 2024-12-24T12:00:00Z
# Go: go1.22.0
```

## Industry References

This implementation follows established patterns:

- **Go ldflags**: [DigitalOcean Guide](https://www.digitalocean.com/community/tutorials/using-ldflags-to-set-version-information-for-go-applications)
- **Health/Version Endpoints**: [Microservices.io Pattern](https://microservices.io/patterns/observability/health-check-api.html)
- **Incremental Deployment**: [DeployHQ](https://www.deployhq.com/blog/what-is-an-incremental-deployment) - 70-95% faster deployments
- **Mage + Hugo**: [Hugo's magefile.go](https://github.com/gohugoio/hugo/blob/master/magefile.go) - Real-world example

## Benefits

1. **Faster deploys** - Skip unchanged components (70-95% faster for small changes)
2. **Visibility** - Know exactly what's running on each environment
3. **Debugging** - Correlate issues with specific commits
4. **Audit trail** - Track when each component was deployed
5. **CI/CD ready** - Foundation for automated deployments

## File Structure

```
api/
├── cmd/labctl/
│   └── main.go           # Version variables
├── internal/server/
│   ├── server.go         # /version route
│   └── version.go        # Version handler
magefiles/
├── build.go              # ldflags injection
├── deploy.go             # Smart deploy logic
└── version.go            # Version utilities
```
