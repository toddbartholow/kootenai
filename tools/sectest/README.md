# sectest

Security testing CLI for the Kootenai platform. Wraps a set of scanners
(nmap, nuclei, OWASP ZAP, SSL checks, and Docker-free HTTP header/security
checks) and produces console and JSON reports.

Only run sectest against Kootenai deployments you own or are explicitly
authorized to test.

## Install

```bash
cd tools/sectest
pip install -e ".[dev]"
```

Requires Python 3.11+. The `full`, `scan nmap`, `scan nuclei`, `scan zap`,
and `scan ssl` commands run their scanners as Docker containers, so Docker
must be installed and running; `quick` and `scan http` need no Docker.

## Usage

```bash
sectest check                 # Verify Docker and scanner availability
sectest quick                 # HTTP security checks only (no Docker)
sectest full                  # Run all scanners
sectest scan <name>           # One scanner: nmap, nuclei, zap, ssl, or http
```

## Configuration

Settings come from (lowest to highest precedence) a YAML config file, a
`.env` file / `SECTEST_`-prefixed environment variables, and CLI overrides.
There is no default target — set one explicitly:

```bash
export SECTEST_TARGET_HOST=192.0.2.10   # your Kootenai host
export SECTEST_TARGET_PORT=8080
export SECTEST_TARGET_SCHEME=http
```

Reports are written to `./sectest-results` (`SECTEST_OUTPUT_DIR` to change).

## Tests

```bash
python -m pytest
```
