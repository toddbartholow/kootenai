#!/bin/bash

# Kootenai Prerequisites Check Script
# Run this to verify your system is ready for Kootenai

set -e

echo "========================================"
echo "Kootenai Prerequisites Check"
echo "========================================"
echo ""

ERRORS=0
WARNINGS=0

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

check_command() {
    local cmd=$1
    local name=$2
    local min_version=$3

    if command -v "$cmd" &> /dev/null; then
        version=$($cmd --version 2>/dev/null | head -1 || echo "unknown")
        echo -e "${GREEN}OK${NC} $name: $version"
        return 0
    else
        echo -e "${RED}FAIL${NC} $name: NOT FOUND"
        return 1
    fi
}

check_version() {
    local cmd=$1
    local name=$2
    local min_version=$3
    local version_cmd=$4

    if command -v "$cmd" &> /dev/null; then
        version=$(eval "$version_cmd" 2>/dev/null || echo "0")
        if [ "$(printf '%s\n' "$min_version" "$version" | sort -V | head -n1)" = "$min_version" ]; then
            echo -e "${GREEN}OK${NC} $name: v$version (required: >=$min_version)"
            return 0
        else
            echo -e "${YELLOW}WARN${NC} $name: v$version (required: >=$min_version) - VERSION TOO OLD"
            return 2
        fi
    else
        echo -e "${RED}FAIL${NC} $name: NOT FOUND (required: >=$min_version)"
        return 1
    fi
}

echo "Checking required tools..."
echo "-------------------------------------------"

# Check Go
if ! check_version "go" "Go" "1.26" "go version | grep -oE '[0-9]+\.[0-9]+' | head -1"; then
    ((ERRORS++))
    echo "Install: https://go.dev/doc/install"
fi

# Check Node.js
if ! check_version "node" "Node.js" "20" "node -v | grep -oE '[0-9]+' | head -1"; then
    ((ERRORS++))
    echo "Install: https://nodejs.org/ (LTS version)"
fi

# Check npm
if ! check_command "npm" "npm"; then
    ((ERRORS++))
    echo "Install: Comes with Node.js"
fi

# Check Docker
if ! check_version "docker" "Docker" "24" "docker version --format '{{.Server.Version}}' 2>/dev/null | grep -oE '^[0-9]+' || echo 0"; then
    ((ERRORS++))
    echo "Install: https://docs.docker.com/get-docker/"
fi

# Check Docker Compose
if docker compose version &> /dev/null; then
    version=$(docker compose version | grep -oE '[0-9]+\.[0-9]+' | head -1)
    echo -e "${GREEN}OK${NC} Docker Compose: v$version"
else
    echo -e "${RED}FAIL${NC} Docker Compose: NOT FOUND"
    ((ERRORS++))
    echo "Install: Included with Docker Desktop, or: https://docs.docker.com/compose/install/"
fi

# Check Git
if ! check_command "git" "Git"; then
    ((ERRORS++))
    echo "Install: https://git-scm.com/downloads"
fi

echo ""
echo "Checking optional tools..."
echo "-------------------------------------------"

# Check Mage (optional, for development)
if command -v mage &> /dev/null; then
    echo -e "${GREEN}OK${NC} Mage: $(mage -version 2>/dev/null || echo 'installed')"
else
    echo -e "${YELLOW}○${NC} Mage: Not installed (optional, for development)"
    echo "Install: go install github.com/magefile/mage@latest"
    ((WARNINGS++))
fi

# Check Python (optional, for scripts)
if command -v python3 &> /dev/null; then
    version=$(python3 --version | grep -oE '[0-9]+\.[0-9]+')
    echo -e "${GREEN}OK${NC} Python: v$version"
else
    echo -e "${YELLOW}○${NC} Python 3: Not installed (optional, for automation scripts)"
    ((WARNINGS++))
fi

# Check curl
if ! check_command "curl" "curl"; then
    ((WARNINGS++))
fi

echo ""
echo "Checking Docker status..."
echo "-------------------------------------------"

# Check if Docker daemon is running
if docker info &> /dev/null; then
    echo -e "${GREEN}OK${NC} Docker daemon: Running"
else
    echo -e "${RED}FAIL${NC} Docker daemon: Not running"
    ((ERRORS++))
    echo "Start Docker Desktop or run: sudo systemctl start docker"
fi

echo ""
echo "Checking port availability..."
echo "-------------------------------------------"

check_port() {
    local port=$1
    local service=$2

    if lsof -Pi :$port -sTCP:LISTEN -t &> /dev/null 2>&1 || netstat -tuln 2>/dev/null | grep -q ":$port "; then
        echo -e "${YELLOW}WARN${NC} Port $port ($service): IN USE"
        ((WARNINGS++))
    else
        echo -e "${GREEN}OK${NC} Port $port ($service): Available"
    fi
}

check_port 3000 "Web Frontend"
check_port 8080 "API Server"
check_port 5432 "PostgreSQL"
check_port 4222 "NATS"

echo ""
echo "Checking system resources..."
echo "-------------------------------------------"

# Check available memory
if command -v free &> /dev/null; then
    mem_gb=$(free -g | awk '/^Mem:/{print $2}')
    if [ "$mem_gb" -ge 8 ]; then
        echo -e "${GREEN}OK${NC} Memory: ${mem_gb}GB available (recommended: 8GB+)"
    elif [ "$mem_gb" -ge 4 ]; then
        echo -e "${YELLOW}WARN${NC} Memory: ${mem_gb}GB available (recommended: 8GB+)"
        ((WARNINGS++))
    else
        echo -e "${RED}FAIL${NC} Memory: ${mem_gb}GB available (minimum: 4GB, recommended: 8GB+)"
        ((ERRORS++))
    fi
elif command -v sysctl &> /dev/null; then
    # macOS
    mem_bytes=$(sysctl -n hw.memsize 2>/dev/null || echo 0)
    mem_gb=$((mem_bytes / 1024 / 1024 / 1024))
    if [ "$mem_gb" -ge 8 ]; then
        echo -e "${GREEN}OK${NC} Memory: ${mem_gb}GB available"
    else
        echo -e "${YELLOW}WARN${NC} Memory: ${mem_gb}GB available (recommended: 8GB+)"
        ((WARNINGS++))
    fi
fi

# Check disk space
if command -v df &> /dev/null; then
    disk_gb=$(df -BG . 2>/dev/null | awk 'NR==2 {gsub("G",""); print $4}' || echo 0)
    if [ "$disk_gb" -ge 20 ]; then
        echo -e "${GREEN}OK${NC} Disk Space: ${disk_gb}GB free"
    elif [ "$disk_gb" -ge 10 ]; then
        echo -e "${YELLOW}WARN${NC} Disk Space: ${disk_gb}GB free (recommended: 20GB+)"
        ((WARNINGS++))
    else
        echo -e "${RED}FAIL${NC} Disk Space: ${disk_gb}GB free (minimum: 10GB)"
        ((ERRORS++))
    fi
fi

echo ""
echo "========================================"
echo "Summary"
echo "========================================"

if [ $ERRORS -eq 0 ] && [ $WARNINGS -eq 0 ]; then
    echo -e "${GREEN}All checks passed! You're ready to run Kootenai.${NC}"
    echo ""
    echo "Next steps:"
    echo "  1. cp deploy/.env.example deploy/.env"
    echo "  2. docker compose up -d"
    echo "  3. cd api && go run ./cmd/labctl serve"
    echo "  4. cd web && npm install && npm run dev"
    echo "  5. Open http://localhost:3000"
    exit 0
elif [ $ERRORS -eq 0 ]; then
    echo -e "${YELLOW}$WARNINGS warning(s) found, but you can proceed.${NC}"
    echo ""
    echo "Review warnings above, then:"
    echo "  1. cp deploy/.env.example deploy/.env"
    echo "  2. docker compose up -d"
    echo "  3. cd api && go run ./cmd/labctl serve"
    echo "  4. cd web && npm install && npm run dev"
    exit 0
else
    echo -e "${RED}$ERRORS error(s) found. Please fix before continuing.${NC}"
    if [ $WARNINGS -gt 0 ]; then
        echo -e "${YELLOW}Also found $WARNINGS warning(s).${NC}"
    fi
    echo ""
    echo "After fixing errors, run this script again:"
    echo "  ./scripts/check-prereqs.sh"
    exit 1
fi
