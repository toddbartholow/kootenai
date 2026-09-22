# CI/CD Pipeline

This document describes the Continuous Integration and Continuous Deployment (CI/CD) pipeline for Kootenai.

## Overview

The pipeline consists of two GitHub Actions workflows:

1. **CI (`ci.yml`)** - Runs on all pushes and pull requests to `main`
2. **Deploy (`deploy.yml`)** - Runs on merge to `main` or manual trigger

## CI Workflow

The CI workflow runs the following jobs:

### Jobs

| Job | Description | Trigger |
|-----|-------------|---------|
| `api-test` | Go tests, vet, fmt check | Push/PR to main |
| `api-build` | Build API binary | Push/PR to main |
| `web-test` | TypeScript check, tests | Push/PR to main |
| `web-build` | Build Vue.js app | Push/PR to main |
| `security` | Trivy vulnerability scan | Push/PR to main |
| `docker-build` | Verify Docker builds | Push/PR to main |

### Triggers

```yaml
on:
  push:
    branches: [main]
  pull_request:
    branches: [main]
```

## Deploy Workflow

The deploy workflow runs after CI passes and deploys to the infra VM.

### Jobs

| Job | Description | Trigger |
|-----|-------------|---------|
| `test` | Pre-deploy tests | Merge to main |
| `deploy` | SSH deploy to infra VM | Merge to main |
| `notify-failure` | Create issue on failure | Deploy failure |

### Triggers

```yaml
on:
  push:
    branches: [main]
  workflow_dispatch:
    inputs:
      environment:
        description: 'Deployment environment'
        required: true
        default: 'production'
```

### Manual Deployment

You can manually trigger a deployment from the GitHub Actions tab:

1. Go to **Actions** → **Deploy**
2. Click **Run workflow**
3. Select the environment (production/staging)
4. Click **Run workflow**

## Required GitHub Secrets

Configure these secrets in your GitHub repository settings:

| Secret | Description | Example |
|--------|-------------|---------|
| `INFRA_HOST` | IP address of infra VM | `<INFRA_VM_IP>` |
| `INFRA_USER` | SSH username | `labadmin` |
| `SSH_PRIVATE_KEY` | SSH private key for deployment | `-----BEGIN OPENSSH PRIVATE KEY-----...` |

### Setting Up Secrets

1. Go to repository **Settings** → **Secrets and variables** → **Actions**
2. Click **New repository secret**
3. Add each secret with its name and value

### Generating SSH Key

```bash
# Generate a new SSH key pair for deployment
ssh-keygen -t ed25519 -f ~/.ssh/kootenai-deploy -C "github-actions-deploy"

# Add public key to infra VM
ssh-copy-id -i ~/.ssh/kootenai-deploy.pub labadmin@<INFRA_VM_IP>

# Copy private key content for GitHub secret
cat ~/.ssh/kootenai-deploy
```

## Deployment Process

The deploy workflow performs these steps:

1. **Checkout** - Clone the repository
2. **Setup SSH** - Configure SSH key from secrets
3. **Sync code** - rsync files to infra VM (excluding node_modules, .git, etc.)
4. **Build containers** - Build API and Web Docker images
5. **Deploy API** - Recreate API container, wait for health check
6. **Deploy Web** - Recreate Web container
7. **Smoke tests** - Verify API and Web are responding
8. **Cleanup** - Prune old Docker images

### Zero-Downtime Deployment

The workflow deploys services one at a time:
1. Deploy API first, wait for health check
2. Deploy Web after API is healthy

### Rollback

If deployment fails, you can rollback using Mage:

```bash
# SSH to infra VM
ssh labadmin@<INFRA_VM_IP>

# Rollback to previous version
cd ~/kootenai
mage deploy:rollback

# Or rollback to specific commit
mage deploy:rollbackTo abc1234
```

## Environment Configuration

### CI Environment Variables

Set in `ci.yml`:
```yaml
env:
  GO_VERSION: '1.24'
  NODE_VERSION: '20'
```

### Deploy Environment Variables

The deploy workflow uses environment-specific variables set in GitHub Environments:
- `production` - Production deployment
- `staging` - Staging deployment (if configured)

## Codecov Integration

Test coverage is uploaded to Codecov:
- API coverage: `api/coverage.out`
- Web coverage: `web/coverage/coverage-final.json`

Configure Codecov by adding the repository to [codecov.io](https://codecov.io).

## Security Scanning

The CI workflow includes Trivy vulnerability scanning:
- Scans filesystem for vulnerabilities
- Reports CRITICAL and HIGH severity issues
- Does not fail the build (informational only)

## Troubleshooting

### CI Failures

**API tests failing:**
```bash
# Run locally
cd api && go test -v ./...
```

**Web tests failing:**
```bash
# Run locally
cd web && npm run typecheck && npm run test:run
```

### Deploy Failures

**SSH connection issues:**
- Verify `SSH_PRIVATE_KEY` secret is correct
- Check infra VM is accessible
- Ensure SSH key is added to `~/.ssh/authorized_keys` on infra VM

**Docker build failures:**
- Check Docker is running on infra VM
- Verify disk space: `df -h`
- Check Docker logs: `docker compose logs`

**Health check failures:**
- Check API logs: `docker compose logs api`
- Verify database is running: `docker compose ps`
- Test manually: `curl http://localhost:8080/health`

### Viewing Logs

```bash
# On infra VM
cd ~/kootenai/deploy
docker compose logs -f api
docker compose logs -f web
```

## Local Testing

You can test the CI checks locally before pushing:

```bash
# API checks
cd api
go fmt ./...
go vet ./...
go test -race ./...

# Web checks
cd web
npm run typecheck
npm run test:run
npm run build

# Docker builds
docker build -t kootenai-api:test ./api
docker build -t kootenai-web:test ./web
```

## Mage Integration

The CI/CD pipeline complements the Mage build system:

```bash
# Local development
mage dev:up           # Start local services
mage test:api         # Run API tests
mage test:web         # Run web tests
mage build:all        # Build all components

# Manual deployment
mage deploy:all       # Full deployment
mage deploy:status    # Check deployment status
mage deploy:logs      # View logs
```
