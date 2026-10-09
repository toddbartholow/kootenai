---
title: Prerequisites
description: System requirements and prerequisites for using Kootenai
tags:
  - prerequisites
  - requirements
  - setup
---

# Prerequisites

This page lists everything you need to use Kootenai effectively.

## For Students

### Browser Requirements

Kootenai works best with modern browsers:

| Browser | Minimum Version | Notes |
|---------|-----------------|-------|
| Chrome | 90+ | Recommended |
| Firefox | 90+ | Full support |
| Safari | 15+ | Full support |
| Edge | 90+ | Full support |

!!! warning "Internet Explorer"
    Internet Explorer is not supported. Please use a modern browser.

### Network Requirements

- **Stable internet connection** - Required for VM console access
- **WebSocket support** - Most networks allow this
- **Ports** - Only standard HTTPS (443) needed

### If You're Behind a Corporate Firewall

You may need to request access to:

- Your institution's Kootenai URL
- WebSocket connections (same URL)

## For Local Development

If you're setting up Kootenai locally for development:

### Required Software

| Software | Version | Purpose |
|----------|---------|---------|
| Docker | 24.0+ | Container runtime |
| Docker Compose | v2+ | Service orchestration |
| Go | 1.26+ | API development |
| Node.js | 20 LTS | Web frontend |
| Git | 2.30+ | Version control |

### Verify Installation

Run the prerequisites check script:

```bash
./scripts/check-prereqs.sh
```

Expected output:

```
========================================
  Kootenai Prerequisites Check
========================================

Checking required tools...
-------------------------------------------
OK Go: v1.26 (required: >=1.26)
OK Node.js: v20 (required: >=20)
OK npm: 10.2.3
OK Docker: v24 (required: >=24)
OK Docker Compose: v2.21
OK Git: git version 2.42.0
```

Each line is prefixed `OK`, `WARN` (present but below the required version) or
`FAIL` (not found), and coloured green, yellow or red respectively.

### Optional Tools

These aren't required but help with development:

| Tool | Purpose |
|------|---------|
| [Mage](https://magefile.org/) | Build system |
| [Air](https://github.com/cosmtrek/air) | Hot reload for Go |
| [jq](https://stedolan.github.io/jq/) | JSON processing |
| [httpie](https://httpie.io/) | API testing |

Install Mage (recommended):

```bash
go install github.com/magefile/mage@latest
```

## For Administrators

### Infrastructure Requirements

To deploy Kootenai in production:

#### Infra VM (Control Plane)

| Resource | Minimum | Recommended |
|----------|---------|-------------|
| CPU | 4 cores | 8 cores |
| RAM | 8 GB | 16 GB |
| Storage | 50 GB SSD | 100 GB SSD |
| Network | 1 Gbps | 10 Gbps |

#### Proxmox VE Cluster

| Resource | Minimum | Per 50 Students |
|----------|---------|-----------------|
| CPU | 8 cores | +4 cores |
| RAM | 32 GB | +16 GB |
| Storage | 500 GB | +250 GB |

### Software Requirements

- **Docker** 24.0+ with Compose v2
- **PostgreSQL** 16+ (included in Docker stack)
- **NATS** 2.10+ (included in Docker stack)
- **Proxmox VE** 8.0+ (for virtualization)

### Network Requirements

| Port | Service | Required |
|------|---------|----------|
| 80 | HTTP redirect | Optional |
| 443 | HTTPS | Yes |
| 3000 | Web UI | Yes |
| 8080 | API | Yes |
| 9090 | Prometheus | Optional |
| 3001 | Grafana | Optional |

### Proxmox Requirements

- API access enabled
- API token with appropriate permissions
- Network bridges configured for pod isolation
- Template VMs created

See [Proxmox Setup](../admin/proxmox-setup.md) for detailed configuration.

## Canvas LMS Integration

If integrating with Canvas — note that LTI is [gated off in this build](../enterprise.md)
and returns 403 until a fork opens the gate:

### Canvas Requirements

- Canvas LMS with LTI 1.3 support
- Admin access to create Developer Keys
- Ability to install external apps

### Configuration Needed

1. Create a Developer Key in Canvas
2. Configure LTI 1.3 settings
3. Install Kootenai as an external app

See [Canvas LMS Integration](../admin/canvas-lms-integration.md) for setup instructions.

## Troubleshooting Prerequisites

### Docker Issues

**Docker daemon not running:**

```bash
# Linux
sudo systemctl start docker

# macOS
open -a Docker
```

**Permission denied:**

```bash
# Add yourself to docker group
sudo usermod -aG docker $USER
# Log out and back in
```

### Node.js Issues

**Wrong Node version:**

Use [nvm](https://github.com/nvm-sh/nvm) to manage versions:

```bash
nvm install 20
nvm use 20
```

### Go Issues

**GOPATH not set:**

Add to your shell profile:

```bash
export GOPATH=$HOME/go
export PATH=$PATH:$GOPATH/bin
```

## Next Steps

Once prerequisites are met:

- [Quick Start](../getting-started.md) - Development setup
- [Demo Mode](../quickstart-demo.md) - Try without infrastructure
- [Production Deployment](../admin/production-deployment.md) - Full deployment
