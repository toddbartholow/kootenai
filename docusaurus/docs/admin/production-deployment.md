# Production Deployment Guide

This guide covers deploying the Kootenai platform to a production environment with monitoring, security, and high availability considerations.

## Prerequisites

### Infrastructure Requirements

| Component | Minimum | Recommended |
|-----------|---------|-------------|
| CPU | 4 cores | 8 cores |
| RAM | 8 GB | 16 GB |
| Storage | 50 GB SSD | 100 GB SSD |
| Network | 1 Gbps | 10 Gbps |

### Software Requirements

- Docker 24.0+ and Docker Compose v2
- Git
- OpenSSL (for certificate generation)
- curl and jq (for verification)

### Network Requirements

- Static IP address
- Ports: 80, 443, 3000 (Web), 8080 (API), 9090 (Prometheus), 3001 (Grafana)
- Outbound access to Docker Hub, GitHub

## Architecture Overview

```
                    ┌─────────────────────────────────────────────┐
                    │              Docker Host (Infra VM)          │
                    │                                              │
   Internet ───────►│  ┌─────────┐  ┌─────────┐  ┌─────────────┐  │
                    │  │  nginx  │──│   web   │──│     api     │  │
                    │  │ (proxy) │  │ (Vue.js)│  │   (labctl)  │  │
                    │  └─────────┘  └─────────┘  └──────┬──────┘  │
                    │                                    │         │
                    │  ┌──────────┐  ┌───────┐  ┌───────┴───────┐ │
                    │  │prometheus│  │ nats  │  │   postgres    │ │
                    │  │          │  │       │  │   database    │ │
                    │  └────┬─────┘  └───────┘  └───────────────┘ │
                    │       │                                      │
                    │  ┌────┴─────┐                                │
                    │  │ grafana  │                                │
                    │  └──────────┘                                │
                    └─────────────────────────────────────────────┘
                                         │
                                         │ API calls
                                         ▼
                    ┌─────────────────────────────────────────────┐
                    │            Proxmox VE Cluster               │
                    │   (Lab VMs, templates, snapshots)           │
                    └─────────────────────────────────────────────┘
```

## Deployment Steps

### 1. Clone the Repository

```bash
ssh your-user@your-server
git clone https://github.com/toddbartholow/kootenai.git
cd kootenai/deploy
```

### 2. Configure Environment

```bash
# Copy example environment file
cp .env.example .env

# Edit with your settings
nano .env
```

**Required Environment Variables:**

```bash
# Database — these are the names docker-compose.yml reads from .env.
# It maps them onto the container's POSTGRES_* variables itself.
DATABASE_USER=labadmin
DATABASE_PASSWORD=<strong-random-password>   # required; compose fails without it
DATABASE_NAME=virtuallab

# Messaging and cache — compose fails to start without these
NATS_PASSWORD=<strong-random-password>
REDIS_PASSWORD=<strong-random-password>

# Authentication
JWT_SECRET=<32+-character-random-string>

# Proxmox Connection
PROXMOX_HOST=https://your-proxmox:8006
PROXMOX_TOKEN_ID=root@pam!kootenai
PROXMOX_TOKEN=<your-proxmox-api-token>
PROXMOX_NODE=pve
PROXMOX_INSECURE=false        # true skips TLS verification (self-signed certs)

# Grafana — required; the container refuses to start without it
GRAFANA_ADMIN_PASSWORD=<admin-password>
```

These eight are the complete set that `deploy/docker-compose.yml` declares as
required (`${VAR:?...}`): `DATABASE_PASSWORD`, `NATS_PASSWORD`,
`REDIS_PASSWORD`, `JWT_SECRET`, `PROXMOX_HOST`, `PROXMOX_TOKEN_ID`,
`PROXMOX_TOKEN` and `GRAFANA_ADMIN_PASSWORD`. Everything else has a default.
`deploy/.env.example` lists the full set with inline notes.

**Generate secure secrets:**

```bash
# JWT Secret
openssl rand -base64 32

# Database password
openssl rand -base64 24 | tr -dc 'a-zA-Z0-9'
```

### 3. Generate SSL Certificates

For development/testing (self-signed):

```bash
mkdir -p ssl
docker run --rm -v $(pwd)/ssl:/ssl alpine/openssl \
  req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout /ssl/server.key -out /ssl/server.crt \
  -subj "/CN=localhost/O=Kootenai/C=US"
chmod 644 ssl/server.crt ssl/server.key
```

For production, use Let's Encrypt or your organization's CA.

### 4. Deploy Services

```bash
# Pull and start all services
docker compose up -d

# Verify services are healthy
docker compose ps

# Expected output: all containers should show "healthy"
```

### 5. Initialize Database

```bash
# Run migrations (automatic on API startup)
docker compose logs api | grep -i migration

# Seed initial data (if needed)
docker compose exec api ./labctl db seed
```

### 6. Verify Deployment

```bash
# API health check
curl -sf http://localhost:8080/health | jq .

# API readiness (database connected)
curl -sf http://localhost:8080/ready | jq .

# Web UI
curl -sf http://localhost:3000 -o /dev/null && echo "Web OK"

# Prometheus targets
curl -sf http://localhost:9090/api/v1/targets | jq '.data.activeTargets[].health'

# Grafana
curl -sf http://localhost:3001/api/health | jq .
```

---

## Monitoring Setup

### Prometheus

Prometheus is pre-configured to scrape metrics from the API.

**Access:** `http://your-server:9090`

**Key Metrics:**

| Metric | Description |
|--------|-------------|
| `labctl_http_requests_total` | Total HTTP requests, counted on entry |
| `labctl_http_responses_by_code_total` | Responses split by status code |
| `labctl_http_request_duration_seconds` | Request latency histogram |
| `labctl_pods_active` | Pods holding hypervisor resources — every status except destroyed |
| `labctl_sessions_active` | Lab sessions that have not ended |
| `labctl_uptime_seconds` | Seconds since the API started |
| `labctl_metrics_reconcile_success_timestamp_seconds` | Unix time of the last successful gauge reconcile |
| `go_goroutines` | Number of goroutines |

The two `*_active` gauges are not maintained by the lifecycle counters. They are
reset from the database every 60 seconds, so they survive an API restart and
cannot drift. `MetricsReconcileStale` alerts if that reconcile stops succeeding.

Every application metric is namespaced `labctl_` (`api/internal/metrics/metrics.go`).
`go_*` and `process_*` come from the standard Go and process collectors, which are
registered on the same registry. `GET /metrics` serves the Prometheus text format, or
a JSON snapshot with `?format=json`.

Set `METRICS_TOKEN` to require `Authorization: Bearer <token>` on that endpoint; leave
it unset and the endpoint is open to anyone who can reach the port.

If you set it, you must also paste the same value into the commented
`authorization:` block on the `kootenai-api` job in
`deploy/prometheus/prometheus.yml`. Prometheus does not read environment
variables from its config file, so `.env` alone will not reach it — and setting
the token in only one place makes every scrape return 401, which takes the
target down and fires `APIDown`.

**Useful PromQL Queries:**

```promql
# Request rate (last 5 minutes)
rate(labctl_http_requests_total[5m])

# Error rate
sum(rate(labctl_http_responses_by_code_total{code=~"5.."}[5m]))
  / sum(rate(labctl_http_responses_total[5m]))

# P95 latency
histogram_quantile(0.95, rate(labctl_http_request_duration_seconds_bucket[5m]))

# Active pods
labctl_pods_active
```

### Grafana

**Access:** `http://your-server:3001`
**Credentials:** `admin` plus the `GRAFANA_ADMIN_PASSWORD` you set in `.env`.
The container will not start without it, so there is no default password.

**Pre-configured dashboard:** "Kootenai Platform", provisioned from
`deploy/grafana/dashboards/kootenai.json`. Its panels are:

- API Status
- Active Pods
- Active Sessions
- Request Rate
- Error Rate
- p95 Latency
- Response Rate by Status Code
- Request Latency Percentiles
- Active HTTP Connections
- Pod Lifecycle
- Session Lifecycle
- Memory Usage
- Goroutines

There is no per-endpoint or per-method breakdown: the HTTP counters carry no
route or method label, and the only labelled series is response code. There is
no database connection-pool panel either — no pool metric is registered.

### Alert Rules

Alert rules are defined in `deploy/prometheus/alerts.yml`:

| Alert | Severity | Condition |
|-------|----------|-----------|
| `APIHighErrorRate` | Critical | >5% of responses are 5xx, 5m |
| `APIHighLatency` | Warning | p99 latency > 2s, 5m |
| `APIDown` | Critical | scrape target down, 2m |
| `HighPodProvisioningTime` | Warning | p90 provision time > 5m, sustained 15m |
| `TooManyActivePods` | Warning | > 50 pods holding resources, 10m |
| `HighCheckpointVerificationRejectionRate` | Info | >10% of checkpoint outcomes are verification rejections, 30m |
| `MetricsReconcileStale` | Warning | gauge reconcile has not succeeded in 5m |
| `HighMemoryUsage` | Warning | RSS > 1GiB, 10m |
| `HighGoroutineCount` | Warning | > 1000 goroutines, 10m |

Four further rules ship commented out, each with a note explaining what it
would take to restore: `DatabaseUnhealthy`, `NATSDisconnected` and
`PodProvisioningFailure` reference metrics the API does not register, and
`NATSDown` depends on a NATS exporter that is not in the compose file.

To add alerting notifications (Slack, email, PagerDuty), configure Alertmanager:

```bash
# Add alertmanager to docker-compose.yml and configure
# See: https://prometheus.io/docs/alerting/latest/alertmanager/
```

---

## Security Hardening

### Network Security

1. **Firewall Rules:** Only expose necessary ports
   ```bash
   # Example UFW rules
   ufw allow 22/tcp    # SSH
   ufw allow 80/tcp    # HTTP redirect
   ufw allow 443/tcp   # HTTPS
   ufw deny 8080/tcp   # Block direct API access (use reverse proxy)
   ufw deny 9090/tcp   # Block Prometheus (internal only)
   ```

2. **Reverse Proxy:** Use nginx for TLS termination
   ```nginx
   server {
       listen 443 ssl;
       server_name your-domain.com;

       ssl_certificate /path/to/fullchain.pem;
       ssl_certificate_key /path/to/privkey.pem;

       location /api/ {
           proxy_pass http://localhost:8080/api/;
       }

       location / {
           proxy_pass http://localhost:3000/;
       }
   }
   ```

### Secret Management

- Rotate secrets regularly (see [Secret Rotation Procedures](secret-rotation.md))
- Never commit secrets to version control
- Use `.env` files with restricted permissions (`chmod 600 .env`)
- Consider HashiCorp Vault for production secrets management

### Security Scanning

The CI pipeline includes:
- **gosec:** Static analysis for Go security issues
- **OWASP ZAP:** Dynamic application security testing (weekly)
- **Trivy:** Container vulnerability scanning
- **npm audit:** JavaScript dependency vulnerabilities

---

## Backup and Recovery

### Database Backup

```bash
# Create backup
docker compose exec postgres pg_dump -U labadmin virtuallab > backup-$(date +%Y%m%d).sql

# Automated daily backups (add to cron)
0 2 * * * cd /home/labadmin/kootenai/deploy && docker compose exec -T postgres pg_dump -U labadmin virtuallab | gzip > /backups/kootenai-$(date +\%Y\%m\%d).sql.gz
```

### Restore from Backup

```bash
# Stop API to prevent writes
docker compose stop api

# Restore database
gunzip -c backup.sql.gz | docker compose exec -T postgres psql -U labadmin virtuallab

# Restart API
docker compose start api
```

### Volume Backups

```bash
# List volumes
docker volume ls | grep kootenai

# Backup Prometheus data
docker run --rm -v kootenai-prometheus-data:/data -v $(pwd):/backup alpine \
  tar czf /backup/prometheus-data.tar.gz -C /data .

# Backup Grafana data
docker run --rm -v kootenai-grafana-data:/data -v $(pwd):/backup alpine \
  tar czf /backup/grafana-data.tar.gz -C /data .
```

---

## Maintenance

### Rolling Updates

:::warning Upgrading from a Virtual Lab deployment

This procedure does **not** work for the first update after the Virtual Lab →
Kootenai rename. `--no-deps` recreates only the named service, so `api` lands on
the renamed `kootenai-internal` network while postgres, nats and redis are still
on `virtual-lab-internal`, and can no longer resolve them. The renamed volumes
also start empty.

Follow the upgrade procedure in `deploy/README.md` once; afterwards this section
applies normally.

:::

```bash
# Pull latest code
cd ~/kootenai
git pull origin main

# Rebuild and restart with zero downtime
cd deploy
docker compose build api web
docker compose up -d --no-deps api
# Wait for API health check
sleep 10 && curl -sf http://localhost:8080/health
docker compose up -d --no-deps web
```

### Log Management

```bash
# View logs
docker compose logs -f api
docker compose logs -f --tail=100 web

# Log rotation (Docker handles this, but configure limits)
# In docker-compose.yml:
#   logging:
#     driver: "json-file"
#     options:
#       max-size: "10m"
#       max-file: "3"
```

### Health Checks

```bash
# Quick health check script
#!/bin/bash
echo "API Health: $(curl -sf http://localhost:8080/health | jq -r .status)"
echo "API Ready: $(curl -sf http://localhost:8080/ready | jq -r .status)"
echo "Web UI: $(curl -sf -o /dev/null -w '%{http_code}' http://localhost:3000)"
echo "Prometheus: $(curl -sf http://localhost:9090/-/healthy && echo OK)"
echo "Grafana: $(curl -sf http://localhost:3001/api/health | jq -r .database)"
```

---

## Troubleshooting

### Common Issues

**API won't start:**
```bash
docker compose logs api | tail -50
# Check for database connection errors, missing env vars
```

**Web UI shows errors:**
```bash
docker compose logs web | tail -50
# Check for nginx configuration issues, SSL certificate problems
```

**Prometheus not scraping:**
```bash
# Verify the API metrics endpoint is reachable from the Prometheus container.
# If METRICS_TOKEN is set, this returns 401 without the header — that is the
# endpoint working, not failing. Add the bearer token to check it properly.
docker exec kootenai-prometheus wget -q -O- http://api:8080/metrics | head -20
docker exec kootenai-prometheus wget -q -O- \
  --header="Authorization: Bearer $METRICS_TOKEN" http://api:8080/metrics | head -20
```

**Grafana dashboards empty:**
```bash
curl -u "admin:${GRAFANA_ADMIN_PASSWORD}" http://localhost:3001/api/datasources | jq .
# Verify Prometheus datasource is configured correctly
```

### If something does not work

This repository is archived and takes no issues, so a production deployment is entirely
yours to debug.

- Check logs: `docker compose logs <service>`
- [Common issues](../troubleshooting/common-issues.md)
- The other guides under `docs/admin/`

---

## Related Documentation

- [Secret Rotation Procedures](secret-rotation.md)
- [Proxmox Setup Guide](proxmox-setup.md)
- [Canvas LTI Integration](canvas-lms-integration.md)
- [Wazuh Integration](wazuh-integration.md)
