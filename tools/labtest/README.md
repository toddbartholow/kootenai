# labtest - Kootenai Platform E2E Testing CLI

A comprehensive testing tool for the Kootenai platform that tests all major functionality end-to-end.

## Installation

```bash
cd tools/labtest
pip install -e .
```

## Quick Start

```bash
# Quick health check (< 10 seconds)
labtest quick

# Full health and connectivity tests
labtest health

# Run diagnostics
labtest diagnose

# List available lab templates
labtest list-labs
```

## Commands

### Quick Tests (< 10 seconds)

```bash
# Basic connectivity check
labtest quick

# Health and connectivity (API, DB, NATS)
labtest health
```

### Full Tests (2-3 minutes)

```bash
# Pod lifecycle tests (create, wait, delete)
labtest pods

# Session management tests
labtest sessions

# Checkpoint detection tests
labtest checkpoints --vm-ip 192.168.1.101

# Full end-to-end test
labtest e2e --lab "Simple Linux Introduction"

# Quick E2E using existing pod
labtest e2e --quick
```

### Combined Tests

```bash
# Run all tests
labtest all

# Only quick tests
labtest all --quick

# Only full tests
labtest all --full
```

### Utility Commands

```bash
# Service diagnostics
labtest diagnose

# List lab templates
labtest list-labs
```

## Configuration

### Environment Variables

```bash
export LABTEST_API_URL=http://<INFRA_IP>:8080
export LABTEST_DB_HOST=<INFRA_IP>
export LABTEST_DB_PORT=5432
export LABTEST_DB_NAME=labctl
export LABTEST_DB_USER=labctl
export LABTEST_DB_PASSWORD=<DB_PASSWORD>
export LABTEST_NATS_URL=nats://<INFRA_IP>:4222
```

### Config File (labtest.yaml)

```yaml
# Connection settings
api_base_url: http://<INFRA_IP>:8080
db_host: <INFRA_IP>
db_port: 5432
db_name: labctl
db_user: labctl
db_password: <DB_PASSWORD>
nats_url: nats://<INFRA_IP>:4222

# Test configuration
test_lab_template: "Simple Linux Introduction"
test_user_id: "7d420402-3706-494e-8dca-92ef98995037"
cleanup_on_success: true

# Scope control
scope:
  quick: true
  full: true
  cleanup: true

# Timeouts (seconds)
timeouts:
  api: 30
  pod_provision: 180
  checkpoint_detect: 60
```

## CLI Options

```bash
# Global options
labtest --verbose          # Enable verbose output
labtest --json             # Output as JSON
labtest --config FILE      # Use custom config file
labtest --api-url URL      # Override API URL

# E2E options
labtest e2e --lab NAME     # Specify lab template
labtest e2e --quick        # Use existing pod
labtest e2e --no-cleanup   # Keep pod after test
```

## Output Formats

### Console (default)

```
┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
┃        Health & Connectivity                       ┃
┣━━━━━━━━━━━━━━━━━━━━┳━━━━━━━━┳━━━━━━━━━┳━━━━━━━━━━━┫
┃ Test               ┃ Status ┃ Time    ┃ Message   ┃
┣━━━━━━━━━━━━━━━━━━━━╋━━━━━━━━╋━━━━━━━━━╋━━━━━━━━━━━┫
┃ API Health Check   ┃  PASS  ┃  52ms   ┃           ┃
┃ Database Ping      ┃  PASS  ┃  12ms   ┃           ┃
┃ NATS Connection    ┃  PASS  ┃  8ms    ┃           ┃
┗━━━━━━━━━━━━━━━━━━━━┻━━━━━━━━┻━━━━━━━━━┻━━━━━━━━━━━┛
  Total: 3 | Passed: 3 | Failed: 0 | Duration: 0.07s
```

### JSON

```bash
labtest --json health > results.json
```

```json
{
  "timestamp": "2025-12-26T12:00:00",
  "duration": 0.07,
  "total": 3,
  "passed": 3,
  "failed": 0,
  "success": true,
  "suites": [...]
}
```

## Exit Codes

- `0` - All tests passed
- `1` - One or more tests failed

## Development

```bash
# Install with dev dependencies
pip install -e ".[dev]"

# Run linter
ruff check labtest

# Run formatter
black labtest

# Type check
mypy labtest
```
