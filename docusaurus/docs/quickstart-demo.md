# Quick Start: Demo Mode (5 Minutes)

This guide gets you running Kootenai **without any infrastructure** - no Proxmox, no CloudStack needed. Perfect for exploring the platform, development, or evaluation.

## Prerequisites

Ensure you have these installed:

| Tool | Version | Check Command |
|------|---------|---------------|
| Docker | 24+ | `docker --version` |
| Docker Compose | 2.0+ | `docker compose version` |
| Go | 1.24+ | `go version` |
| Node.js | 20 LTS | `node --version` |
| Git | Any | `git --version` |

### Quick Install (macOS)

```bash
brew install go node docker
```

### Quick Install (Ubuntu/Debian)

```bash
# Docker
curl -fsSL https://get.docker.com | sh

# Go 1.24+
wget https://go.dev/dl/go1.24.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.24.0.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# Node.js 20 LTS
curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
sudo apt-get install -y nodejs
```

## Step 1: Clone the Repository

```bash
git clone https://github.com/toddbartholow/kootenai.git
cd kootenai
```

## Step 2: Configure Environment

```bash
# Copy the example environment file
cp deploy/.env.example .env
```

Edit `.env` and ensure these settings for demo mode:

```bash
# Demo/Development Settings
AUTH_DEMO_MODE=true
JWT_SECRET=demo-secret-change-in-production

# Database (Docker will create this)
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_NAME=virtuallab
DATABASE_USER=labadmin
DATABASE_PASSWORD=labpass

# Optional: Mock Proxmox responses (no real VMs)
# MOCK_PROXMOX=true
```

## Step 3: Start Database and NATS

```bash
# Start PostgreSQL and NATS in Docker
docker compose -f docker-compose.dev.yml up -d

# Verify they're running
docker compose -f docker-compose.dev.yml ps
```

You should see:
```
NAME                    STATUS
kootenai-nats        running
kootenai-postgres    running
```

## Step 4: Start the API Server

```bash
cd api

# Download dependencies (first time only)
go mod download

# Run the API server
go run ./cmd/labctl serve
```

You should see:
```
INFO Starting HTTP server addr=0.0.0.0:8080
INFO Database connected
INFO NATS connected
```

**Keep this terminal running** and open a new one for the next step.

## Step 5: Start the Web Frontend

```bash
cd web

# Install dependencies (first time only)
npm install

# Start development server
npm run dev
```

You should see:
```
VITE v5.x.x ready in xxx ms

➜  Local:   http://localhost:3000/
```

## Step 6: Access the Platform

Open your browser to: **http://localhost:3000**

### Demo Login

In demo mode, you can log in with any credentials. The system will create a demo user automatically.

Try:
- **Username**: `demo`
- **Password**: `demo`

Or use any email/password combination - demo mode accepts all logins.

## What Works in Demo Mode

| Feature | Status | Notes |
|---------|--------|-------|
| Login/Authentication | ✅ | Any credentials work |
| Dashboard | ✅ | Shows sample data |
| Lab Catalog | ✅ | Browse available labs |
| Achievements | ✅ | View achievement system |
| Pathways | ✅ | Learning path navigation |
| Admin Panel | ✅ | User management, analytics |
| Pod Creation | ⚠️ | Creates records but no real VMs |
| VM Console | ❌ | Requires real Proxmox |
| Checkpoint Evaluation | ⚠️ | Simulated without Wazuh |

## Verify Everything Works

### Check API Health

```bash
curl http://localhost:8080/health
# Returns: {"status":"ok"}

curl http://localhost:8080/ready
# Returns: {"status":"ready","checks":{...}}
```

### Check Available Labs

```bash
curl http://localhost:8080/api/v1/labs
# Returns list of lab templates
```

### Check Achievements

```bash
curl http://localhost:8080/api/v1/achievements
# Returns 16 default achievements
```

## Common Issues

### Port Already in Use

```bash
# Check what's using port 8080
lsof -i :8080

# Or use a different port
API_PORT=8081 go run ./cmd/labctl serve
```

### Database Connection Failed

```bash
# Check if PostgreSQL is running
docker compose -f docker-compose.dev.yml ps

# View PostgreSQL logs
docker compose -f docker-compose.dev.yml logs postgres

# Restart if needed
docker compose -f docker-compose.dev.yml restart postgres
```

### NATS Connection Failed

The API runs fine without NATS, but real-time updates won't work.

```bash
# Check NATS status
docker compose -f docker-compose.dev.yml logs nats

# Restart if needed
docker compose -f docker-compose.dev.yml restart nats
```

### npm install Fails

```bash
# Clear npm cache
npm cache clean --force

# Delete node_modules and retry
rm -rf node_modules package-lock.json
npm install
```

## Next Steps

Once you've explored demo mode:

1. **[Set Up Proxmox](./admin/proxmox-setup.md)** - Connect to real VMs
2. **[Create Your First Lab](./tutorials/first-lab.md)** - Build a lab template
3. **[API Reference](./api/reference.md)** - Explore the API
4. **[Production Deployment](./admin/production-deployment.md)** - Deploy for real use

## Stopping Everything

```bash
# Stop the web frontend (Ctrl+C in that terminal)
# Stop the API server (Ctrl+C in that terminal)

# Stop Docker services
docker compose -f docker-compose.dev.yml down

# To also remove data volumes
docker compose -f docker-compose.dev.yml down -v
```

## Getting Help

- **GitHub Issues**: https://github.com/toddbartholow/kootenai/issues
- **Documentation**: https://github.com/toddbartholow/kootenai/tree/main/docs
