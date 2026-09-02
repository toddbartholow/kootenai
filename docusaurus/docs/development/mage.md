---
title: Build System (Mage)
description: Guide to the Mage build system for Kootenai development
tags:
  - development
  - build
  - mage
---

# Build System (Mage)

Kootenai uses [Mage](https://magefile.org/) as its build system. Mage provides a modern, Go-native alternative to Make with better cross-platform support and type safety.

## Overview

Mage targets are defined in Go files in the `magefiles/` directory:

```
magefiles/
├── magefile.go      # Main entry point
├── build.go         # Build targets
├── dev.go           # Development environment
├── test.go          # Test and lint targets
├── db.go            # Database operations
├── deploy.go        # Deployment targets
├── proxmox.go       # Proxmox integration
├── config.go        # Configuration loading
├── ssh.go           # SSH utilities
└── version.go       # Version tracking
```

## Installation

```bash
# Install Mage
go install github.com/magefile/mage@latest

# Verify installation
mage --version
```

## Quick Reference

### List All Targets

```bash
mage -l
```

Output:

```
Targets:
  build:all        Build all components
  build:api        Build API binary
  build:apiLinux   Build API for Linux
  build:web        Build web frontend
  build:docker     Build Docker images
  build:clean      Clean build artifacts
  dev:setup        Set up development environment
  dev:up           Start development services
  dev:down         Stop development services
  dev:api          Run API in development mode
  dev:web          Run web frontend in dev mode
  dev:status       Show service status
  dev:logs         Show service logs
  dev:psql         Open PostgreSQL shell
  test:all         Run all tests
  test:api         Run API tests
  test:apiCoverage Run API tests with coverage
  test:apiRace     Run API tests with race detection
  test:web         Run web tests
  test:lint        Run linters
  db:migrate       Run database migrations
  db:migrateRemote Run migrations on remote
  db:reset         Reset database
  db:seed          Seed database with test data
  db:status        Show migration status
  db:psql          Open database shell
  deploy:all       Full deployment
  deploy:api       Deploy API only
  deploy:web       Deploy web only
  deploy:quick     Quick deploy (skip build if unchanged)
  deploy:restart   Restart services
  deploy:status    Show deployment status
  deploy:logs      Show deployment logs
  proxmox:status   Check Proxmox status
  proxmox:test     Run Proxmox tests
  proxmox:testConn Test Proxmox connection
  proxmox:listVMs  List Proxmox VMs
```

### Run a Target

```bash
# Simple target
mage build:api

# Target with namespace
mage dev:up

# Multiple targets
mage build:api deploy:api
```

## Target Namespaces

### Build Targets

Build binaries and containers:

```bash
# Build everything
mage build:all

# Build API binary (current platform)
mage build:api

# Build API for Linux (for deployment)
mage build:apiLinux

# Build web frontend
mage build:web

# Build Docker images
mage build:docker

# Clean build artifacts
mage build:clean
```

### Development Targets

Local development environment:

```bash
# Initial setup
mage dev:setup

# Start services (PostgreSQL, NATS)
mage dev:up

# Stop services
mage dev:down

# Run API in dev mode (with hot reload)
mage dev:api

# Run web frontend
mage dev:web

# Check service status
mage dev:status

# View logs
mage dev:logs

# Open PostgreSQL shell
mage dev:psql
```

### Test Targets

Run tests and linters:

```bash
# Run all tests
mage test:all

# Run API tests only
mage test:api

# Run with coverage report
mage test:apiCoverage

# Run with race detector
mage test:apiRace

# Run web tests
mage test:web

# Run linters (golangci-lint, eslint)
mage test:lint
```

### Database Targets

Database management:

```bash
# Run pending migrations
mage db:migrate

# Run migrations on infra VM
mage db:migrateRemote

# Reset database (drop and recreate)
mage db:reset

# Seed with test data
mage db:seed

# Check migration status
mage db:status

# Open database shell
mage db:psql
```

### Deploy Targets

Deployment to infra VM:

```bash
# Full deployment (build + deploy all)
mage deploy:all

# Deploy API only
mage deploy:api

# Deploy web only
mage deploy:web

# Quick deploy (skip if unchanged)
mage deploy:quick

# Restart services
mage deploy:restart

# Check deployment status
mage deploy:status

# View deployment logs
mage deploy:logs
```

### Proxmox Targets

Proxmox integration testing:

```bash
# Check Proxmox cluster status
mage proxmox:status

# Test connection
mage proxmox:testConn

# List VMs
mage proxmox:listVMs

# List templates
mage proxmox:listTemplates

# Run full Proxmox test suite
mage proxmox:test
```

## Configuration

Mage reads configuration from environment variables only. It has no dotenv
support (`magefiles/config.go` uses `os.Getenv` throughout), so values placed in
a `.env` file are ignored and the defaults apply. Export them first, or prefix
the individual command — for example
`INFRA_HOST=10.0.0.5 mage deploy:api`:

```bash
# Infrastructure — export these, they are not read from a file
export
INFRA_HOST=<INFRA_VM_IP>
INFRA_USER=labadmin

# Database
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_USER=labadmin
DATABASE_PASSWORD=<your-database-password>
DATABASE_NAME=virtuallab

# Proxmox
PROXMOX_HOST=https://<PROXMOX_IP>:8006
PROXMOX_TOKEN_ID=root@pam!kootenai
PROXMOX_TOKEN=your-token-here
PROXMOX_NODE=pve
```

## Common Workflows

### Start Development

```bash
# One-time setup
mage dev:setup

# Start services and run API
mage dev:up
mage dev:api

# In another terminal, run web
mage dev:web
```

### Run Tests Before Commit

```bash
# Run all checks
mage test:lint test:api test:web
```

### Deploy Changes

```bash
# Full deployment
mage deploy:all

# Or step by step
mage build:apiLinux build:web
mage deploy:api deploy:web

# Verify
mage deploy:status
```

### Debug Database

```bash
# Check migrations
mage db:status

# Open shell
mage db:psql

# Reset if needed
mage db:reset
mage db:migrate
mage db:seed
```

## Writing Custom Targets

### Basic Target

```go
// magefiles/custom.go
package main

import (
    "fmt"
    "github.com/magefile/mage/mg"
)

type Custom mg.Namespace

// MyTask does something useful
func (Custom) MyTask() error {
    fmt.Println("Running my task...")
    return nil
}
```

Run with: `mage custom:myTask`

### Target with Dependencies

```go
// Depends on build:api
func (Deploy) Api() error {
    mg.Deps(Build.Api)
    // Deploy logic...
    return nil
}
```

### Target with Parameters

Mage targets can't have parameters directly, but you can use environment variables:

```go
func (Deploy) To() error {
    target := os.Getenv("DEPLOY_TARGET")
    if target == "" {
        target = "production"
    }
    // Deploy to target...
    return nil
}
```

Run with: `DEPLOY_TARGET=staging mage deploy:to`

## Version Tracking

The build system embeds version information:

```bash
# View embedded version
./bin/labctl version

# Output:
# Version: 1.0.0
# Commit: abc1234
# Build Time: 2025-01-15T10:00:00Z
```

### Smart Rebuilds

Deploy targets check version to skip unnecessary rebuilds:

```bash
# Skips build if version matches
mage deploy:quick

# Forces rebuild
mage deploy:all
```

## Troubleshooting

### Mage Not Found

```bash
# Ensure GOPATH/bin is in PATH
export PATH=$PATH:$(go env GOPATH)/bin
```

### Target Not Found

```bash
# List available targets
mage -l

# Check for typos in namespace
mage build:api  # Correct
mage Build:Api  # Wrong (case sensitive)
```

### Compilation Errors

```bash
# Clean and rebuild mage cache
mage -clean
mage -compile ./mage_output
```

### SSH Connection Issues

For deploy targets:

```bash
# Test SSH connection
ssh labadmin@<INFRA_VM_IP> echo "Connected"

# Check SSH key
ssh-add -l
```

## Related Documentation

- [CI/CD](ci-cd.md) - Continuous integration
- [Testing](testing.md) - Testing guide
- [Version Tracking](version-tracking.md) - Version management
