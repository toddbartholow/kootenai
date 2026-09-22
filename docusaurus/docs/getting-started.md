# Getting Started

This guide walks you through setting up the Kootenai Platform for local development.

## Prerequisites

Before you begin, ensure you have the following installed:

| Software | Version | Purpose |
|----------|---------|---------|
| [Docker](https://docker.com) | 24+ | Run PostgreSQL and NATS |
| [Go](https://go.dev) | 1.25+ | Build and run the API |
| [Node.js](https://nodejs.org) | 20 LTS | Build and run the web UI |
| [Git](https://git-scm.com) | 2.x | Clone the repository |

## Quick Start

### 1. Clone the Repository

```bash
git clone https://github.com/toddbartholow/kootenai.git
cd kootenai
```

### 2. Configure Environment

Copy the example environment file:

```bash
cp deploy/.env.example .env
```

The defaults work out of the box for local development. Review `.env` if you need to customize ports or credentials.

### 3. Start Infrastructure Services

Start PostgreSQL and NATS using Docker Compose:

```bash
docker compose -f docker-compose.dev.yml up -d
```

Verify the services are running:

```bash
docker compose -f docker-compose.dev.yml ps
```

You should see:

```
NAME              STATUS              PORTS
labctl-postgres   Up (healthy)        0.0.0.0:5432->5432/tcp
labctl-nats       Up (healthy)        0.0.0.0:4222->4222/tcp, 0.0.0.0:8222->8222/tcp
```

### 4. Build and Run the API

```bash
cd api
go build -o bin/labctl ./cmd/labctl
./bin/labctl serve
```

Or run directly without building:

```bash
cd api
go run ./cmd/labctl serve
```

The API will:

- Connect to PostgreSQL and run migrations automatically
- Connect to NATS for event streaming
- Start listening on `http://localhost:8080`

!!! tip "Development Mode"
    Without Proxmox/CloudStack credentials, the API runs in demo mode with mock data.

### 5. Run the Web UI

In a new terminal:

```bash
cd web
npm install
npm run dev
```

The web UI will start at `http://localhost:3000` and proxy API requests to the backend.

### 6. Verify Everything Works

Open your browser to [http://localhost:3000](http://localhost:3000). You should see the Kootenai dashboard.

Test the API health endpoint:

```bash
curl http://localhost:8080/health
# {"status":"ok"}
```

## Configuration

### Database Configuration

The API reads database configuration from `config/config.yaml` or environment variables:

```yaml
database:
  host: "localhost"
  port: 5432
  user: "labctl"
  password: "<DB_PASSWORD>"
  database: "labctl"
  # Note the underscore: the key is ssl_mode. A misspelled key is silently
  # ignored by the loader, which then falls back to the built-in default.
  ssl_mode: "disable"
```

`ssl_mode` accepts only the values the PostgreSQL driver (`lib/pq`) implements:
`disable`, `require`, `verify-ca`, `verify-full`. libpq's `allow` and `prefer`
are **not** supported and are rejected at connect time. The bundled Postgres
(`postgres:16-alpine`) is built without TLS enabled, so local development uses
`disable`; raise it to `require` or `verify-full` for any database reached over
a network you do not control.

Environment variable overrides:

| Variable | Description |
|----------|-------------|
| `DATABASE_HOST` | PostgreSQL hostname |
| `DATABASE_PORT` | PostgreSQL port |
| `DATABASE_USER` | Database username |
| `DATABASE_PASSWORD` | Database password |
| `DATABASE_NAME` | Database name |
| `DATABASE_SSL_MODE` | TLS mode: `disable`, `require`, `verify-ca`, `verify-full` (note: `DATABASE_SSL_MODE`, not `DATABASE_SSLMODE`) |

### Logging Configuration

Configure logging output for easy debugging:

```yaml
logging:
  level: "debug"           # debug, info, warn, error
  output: "both"           # stdout, file, or both
  file_path: "/var/log/labctl/labctl.log"
  format: "text"           # text (for tail -f) or json
```

To tail logs in real-time:

```bash
# Create log directory (first time only)
sudo mkdir -p /var/log/labctl
sudo chown $USER /var/log/labctl

# Tail logs
tail -f /var/log/labctl/labctl.log
```

### NATS Configuration

NATS provides event streaming for real-time updates:

```yaml
nats:
  url: "nats://localhost:4222"
```

Monitor NATS at [http://localhost:8222](http://localhost:8222).

## Common Commands

### Docker Compose

```bash
# Start services
docker compose -f docker-compose.dev.yml up -d

# View logs
docker compose -f docker-compose.dev.yml logs -f

# Stop services
docker compose -f docker-compose.dev.yml down

# Stop and delete all data
docker compose -f docker-compose.dev.yml down -v

# Check service health
docker compose -f docker-compose.dev.yml ps
```

### Database

```bash
# Run migrations manually
./api/bin/labctl migrate

# Check migration status
./api/bin/labctl migrate --status

# Connect to PostgreSQL directly
docker exec -it labctl-postgres psql -U labctl -d labctl
```

### API

```bash
# Build
cd api && go build -o bin/labctl ./cmd/labctl

# Run with live reload (requires air)
air

# Run tests
go test ./...

# Format code
go fmt ./...
```

### Web UI

```bash
# Install dependencies
npm install

# Development server
npm run dev

# Build for production
npm run build

# Type checking
npm run typecheck

# Linting
npm run lint
```

## Troubleshooting

### Database Connection Failed

If you see `Failed to connect to database`:

1. Check Docker is running: `docker compose ps`
2. Verify PostgreSQL is healthy: `docker compose logs postgres`
3. Check port 5432 isn't in use: `lsof -i :5432`

### NATS Connection Failed

The API runs fine without NATS (events won't be persisted):

```
WARN Failed to connect to NATS
INFO Running without NATS - events will not be persisted
```

To fix, ensure NATS is running: `docker compose up -d nats`

### Port Already in Use

If ports 5432, 4222, 8080, or 3000 are in use:

1. Stop conflicting services, or
2. Change ports in `.env`:

```bash
DATABASE_PORT=5433
NATS_PORT=4223
API_PORT=8081
```

### Reset Everything

To start fresh:

```bash
# Stop services and delete volumes
docker compose -f docker-compose.dev.yml down -v

# Remove local config
rm config/config.yaml

# Start fresh
docker compose -f docker-compose.dev.yml up -d
```

## Next Steps

- [Architecture Overview](architecture/overview.md) - Understand the system design
- [Lab Templates](lab-templates/template-creation-guide.md) - Create your first lab template
- [API Reference](api/reference.md) - Explore the REST API
