# Kootenai Infrastructure VM Deployment

This directory contains everything needed to deploy the Kootenai control plane in a portable VM that can run on any hypervisor.

## Upgrading from a Virtual Lab deployment

This project was renamed from **Virtual Lab** to **Kootenai**. The checkout
directory, containers, images, Docker **networks** and Docker **volumes** all
carry the new name. A fresh install needs none of this section.

Upgrading in place is **not** a `git pull` away, for three reasons:

- The old `virtual-lab-*` containers still hold ports 8080, 3000, 9090 and
  3001, so the new stack cannot bind them.
- Volumes were renamed, so the new stack starts against **empty** ones. The old
  data is still on disk under the previous names; nothing is reading it.
- Networks were renamed. In particular, the "Rolling Updates" procedure in the
  production deployment guide uses `docker compose up -d --no-deps`, which
  recreates only that one service. After the rename it lands on
  `kootenai-internal` while postgres, nats and redis are still on
  `virtual-lab-internal`, and can no longer resolve them. **Do not use
  `--no-deps` for this upgrade** — it needs a full recreate.

The Postgres **database name is unchanged** (`virtuallab`); only the volume
holding it was renamed.

### Procedure

Run from the old checkout (`~/virtual-lab`):

```bash
cd ~/virtual-lab/deploy
docker compose down                 # frees the ports and the old networks
```

Move the checkout, since every path in the tooling now says `~/kootenai`:

```bash
mv ~/virtual-lab ~/kootenai
cd ~/kootenai && git pull origin main
```

Copy each volume. The guard matters: if you have already run `docker compose
up -d` since upgrading, Postgres has initialised a *new* cluster in the
destination, and copying over it leaves files from two different clusters in
one directory — including stale WAL segments, which is a recovery hazard.

```bash
for v in postgres nats redis prometheus grafana; do
  src="virtual-lab-${v}-data"
  dst="kootenai-${v}-data"

  docker volume inspect "$src" >/dev/null 2>&1 || { echo "SKIP: no $src"; continue; }

  if [ -n "$(docker run --rm -v "$dst":/v alpine sh -c 'ls -A /v 2>/dev/null')" ]; then
    echo "REFUSING: $dst is not empty. Remove it first (docker volume rm $dst)"
    echo "          -- but only if you are certain it holds no data you want."
    continue
  fi

  docker volume create "$dst" >/dev/null
  docker run --rm -v "$src":/from -v "$dst":/to alpine sh -c 'cd /from && cp -a . /to'
  echo "copied $src -> $dst"
done
```

Note the explicit `docker volume inspect` check: `docker run -v <name>:/from`
silently *creates* an empty volume when the name does not exist, so a typo in
the source name would otherwise copy nothing and exit 0.

Then start the new stack and verify before deleting anything:

```bash
docker compose up -d
docker compose exec postgres psql -U labadmin -d virtuallab -c '\dt'
```

That should list the expected tables. The copy leaves the old `virtual-lab-*`
volumes untouched, so this is reversible until you remove them yourself.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  Infrastructure VM (this deployment)                        │
│  ┌───────────────────────────────────────────────────────┐  │
│  │  Docker Compose                                       │  │
│  │  ├── PostgreSQL (database)                            │  │
│  │  ├── NATS (message queue + JetStream)                 │  │
│  │  ├── API Server (Go)                                  │  │
│  │  └── Web Frontend (Vue.js)                            │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
            │
            │ API calls (HTTPS)
            ▼
┌─────────────────────────────────────────────────────────────┐
│  Lab VM Cluster (Proxmox, CloudStack)                       │
│  └── Lab Pod VMs (cloned from templates)                    │
└─────────────────────────────────────────────────────────────┘
```

## Quick Start

### Option 1: Bootstrap Script (Recommended)

On a fresh Rocky Linux 9 or Ubuntu 22.04+ VM:

```bash
curl -fsSL https://raw.githubusercontent.com/toddbartholow/kootenai/main/deploy/bootstrap.sh | bash
```

### Option 2: Manual Setup

1. Install Docker and Docker Compose
2. Clone this repository
3. Configure environment:
   ```bash
   cd deploy
   cp .env.example .env
   # Edit .env with your settings
   ```
4. Start services:
   ```bash
   docker compose up -d
   ```

## Configuration

Copy `.env.example` to `.env` and configure:

| Variable | Required | Description |
|----------|----------|-------------|
| `DATABASE_PASSWORD` | Yes | PostgreSQL password |
| `NATS_PASSWORD` | Yes | NATS password |
| `REDIS_PASSWORD` | Yes | Redis password |
| `PROXMOX_HOST` | Yes | Proxmox API URL (e.g., `https://<PROXMOX_IP>:8006`) |
| `PROXMOX_TOKEN_ID` | Yes | API token ID (e.g., `<PROXMOX_TOKEN_ID>`) |
| `PROXMOX_TOKEN` | Yes | API token secret |
| `JWT_SECRET` | Yes | Auth secret (generate: `openssl rand -base64 32`) |
| `GRAFANA_ADMIN_PASSWORD` | Yes | Grafana admin password |

Every row above is declared `${VAR:?...}` in `docker-compose.yml`, so a missing
value fails `docker compose up` rather than starting a broken stack.

Being *required* is not the same as being *delivered*, though, and the two came
apart for Redis: `REDIS_PASSWORD` gated startup via the `redis` service while
the `api` service was never passed it, so the stack started, authenticated
nothing against Redis, and silently fell back to per-process in-memory rate
limiting. The API service now receives it under the same `${VAR:?...}` guard.
When adding a secret, check both that it is required *and* that every service
which reads it appears in that service's `environment:` block.

Optional but worth setting:

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_SSL_MODE` | `disable` | TLS mode for Postgres: `disable`, `require`, `verify-ca`, `verify-full`. Defaults to `disable` because the bundled `postgres:16-alpine` is built with `ssl=off` and is not port-published. **Set `require` or `verify-full` whenever `DATABASE_HOST` points outside this compose file.** |

See `.env.example` for all available options.

## Creating a Portable OVA

To create an OVA appliance that can be imported into any hypervisor:

### From Proxmox

1. Create the base VM (Rocky 9 or Ubuntu 22.04):
   ```bash
   # On Proxmox host
   qm create 9001 --name kootenai-infra --memory 4096 --cores 2
   qm set 9001 --scsi0 local-lvm:32
   qm set 9001 --ide2 local:iso/rocky-9-minimal.iso,media=cdrom
   qm set 9001 --boot order=ide2
   qm set 9001 --net0 virtio,bridge=vmbr0
   ```

2. Install OS and run bootstrap script

3. Clean up for template:
   ```bash
   # Inside the VM
   sudo cloud-init clean
   sudo rm -rf /var/lib/cloud/*
   sudo truncate -s 0 /etc/machine-id
   sudo rm -f /etc/ssh/ssh_host_*
   sudo rm -f ~/.bash_history
   history -c
   sudo shutdown -h now
   ```

4. Export as OVA:
   ```bash
   # On Proxmox host (requires ovftool or manual process)
   # Option A: Use vzdump and convert
   vzdump 9001 --dumpdir /tmp --mode stop

   # Option B: Direct disk export
   qm stop 9001
   # Export disk and create OVF manually
   ```

### From VMware Workstation/ESXi

1. Create VM with Rocky 9 or Ubuntu 22.04
2. Run bootstrap script
3. Clean up (same steps as above)
4. File → Export to OVF/OVA

### From Hyper-V

1. Create VM with Rocky 9 or Ubuntu 22.04
2. Run bootstrap script
3. Clean up and sysprep
4. Export VM, then convert to OVA using `qemu-img` and `ovftool`

## VM Requirements

| Resource | Minimum | Recommended |
|----------|---------|-------------|
| CPU | 2 cores | 4 cores |
| RAM | 4 GB | 8 GB |
| Disk | 32 GB | 64 GB |
| Network | 1 NIC | 1 NIC |

## Service Management

```bash
cd ~/kootenai/deploy

# View status
docker compose ps

# View logs
docker compose logs -f
docker compose logs -f api  # API only

# Restart services
docker compose restart

# Stop all services
docker compose down

# Start services
docker compose up -d

# Rebuild after code changes
docker compose build
docker compose up -d
```

## Updating

```bash
cd ~/kootenai
git pull
cd deploy
docker compose build
docker compose down
docker compose up -d
```

## Troubleshooting

### Services won't start

Check logs:
```bash
docker compose logs
```

Verify `.env` configuration:
```bash
docker compose config
```

### Can't connect to Proxmox

1. Verify `PROXMOX_HOST` URL is correct
2. Check API token permissions
3. If using self-signed cert, ensure `PROXMOX_INSECURE=true`

### Database connection errors

```bash
# Check PostgreSQL is running
docker compose ps postgres

# Connect to database
docker compose exec postgres psql -U labadmin -d virtuallab
```

### API health check fails

```bash
# Check API logs
docker compose logs api

# Test health endpoint
curl http://localhost:8080/health
```

## Backup and Restore

### Backup

```bash
# Backup database
docker compose exec postgres pg_dump -U labadmin virtuallab > backup.sql

# Backup volumes
docker run --rm -v kootenai-postgres-data:/data -v $(pwd):/backup alpine tar czf /backup/postgres-data.tar.gz /data
docker run --rm -v kootenai-nats-data:/data -v $(pwd):/backup alpine tar czf /backup/nats-data.tar.gz /data
```

### Restore

```bash
# Restore database
cat backup.sql | docker compose exec -T postgres psql -U labadmin virtuallab

# Restore volumes
docker run --rm -v kootenai-postgres-data:/data -v $(pwd):/backup alpine tar xzf /backup/postgres-data.tar.gz -C /
```

## Security Considerations

- Change all default passwords in `.env`
- Generate a strong `JWT_SECRET`
- Consider placing behind a reverse proxy with TLS
- Restrict network access to the infra VM
- Regularly update containers: `docker compose pull && docker compose up -d`
