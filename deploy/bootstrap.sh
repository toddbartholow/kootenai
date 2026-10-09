#!/bin/bash
# Kootenai Infrastructure VM Bootstrap Script
#
# This script prepares a fresh Rocky Linux 9 or Ubuntu 22.04+ VM to run
# the Kootenai platform using Docker Compose.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/toddbartholow/kootenai/main/deploy/bootstrap.sh | bash
#
# Or download and run:
#   chmod +x bootstrap.sh
#   ./bootstrap.sh
#
# Tested on:
#   - Rocky Linux 9
#   - Ubuntu 22.04 LTS
#   - Ubuntu 24.04 LTS
#   - Debian 12

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Detect OS
detect_os() {
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        OS=$ID
        VERSION=$VERSION_ID
    else
        log_error "Cannot detect OS. /etc/os-release not found."
        exit 1
    fi
    log_info "Detected OS: $OS $VERSION"
}

# Install Docker on Rocky Linux
install_docker_rocky() {
    log_info "Installing Docker on Rocky Linux..."

    # Remove old versions
    sudo dnf remove -y docker docker-client docker-client-latest \
        docker-common docker-latest docker-latest-logrotate \
        docker-logrotate docker-engine podman runc 2>/dev/null || true

    # Add Docker repository
    sudo dnf config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo

    # Install Docker
    sudo dnf install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin

    # Start and enable Docker
    sudo systemctl start docker
    sudo systemctl enable docker
}

# Install Docker on Ubuntu/Debian
install_docker_debian() {
    log_info "Installing Docker on Ubuntu/Debian..."

    # Remove old versions
    sudo apt-get remove -y docker docker-engine docker.io containerd runc 2>/dev/null || true

    # Update and install prerequisites
    sudo apt-get update
    sudo apt-get install -y ca-certificates curl gnupg lsb-release

    # Add Docker GPG key
    sudo install -m 0755 -d /etc/apt/keyrings
    curl -fsSL https://download.docker.com/linux/$OS/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
    sudo chmod a+r /etc/apt/keyrings/docker.gpg

    # Add Docker repository
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/$OS $(lsb_release -cs) stable" | \
        sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

    # Install Docker
    sudo apt-get update
    sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
}

# Add current user to docker group
setup_docker_user() {
    log_info "Adding current user to docker group..."
    sudo usermod -aG docker $USER
    log_warn "You may need to log out and back in for group changes to take effect."
}

# Install additional tools
install_tools() {
    log_info "Installing additional tools..."

    case $OS in
        rocky|centos|rhel|fedora)
            sudo dnf install -y git curl wget vim
            ;;
        ubuntu|debian)
            sudo apt-get install -y git curl wget vim
            ;;
    esac
}

# Clone Kootenai repository
clone_repo() {
    INSTALL_DIR="${HOME}/kootenai"

    if [ -d "$INSTALL_DIR" ]; then
        log_warn "Directory $INSTALL_DIR already exists. Pulling latest changes..."
        cd "$INSTALL_DIR"
        git pull
    else
        log_info "Cloning Kootenai repository..."
        git clone https://github.com/toddbartholow/kootenai.git "$INSTALL_DIR"
        cd "$INSTALL_DIR"
    fi
}

# Setup environment file
setup_env() {
    log_info "Setting up environment configuration..."

    if [ ! -f deploy/.env ]; then
        # 0600 from the moment it exists. This file gets real secrets a few
        # lines down, and harden_permissions does not run until after the
        # build-and-start cycle — minutes during which cp's default 0644 would
        # leave them readable by every local user.
        ( umask 077 && cp deploy/.env.example deploy/.env )

        # Fill in every required secret we can generate ourselves. The
        # template writes these as `VAR=` followed by an inline comment, so
        # anchor on the name and replace only up to the comment.
        for var in JWT_SECRET DATABASE_PASSWORD NATS_PASSWORD REDIS_PASSWORD GRAFANA_ADMIN_PASSWORD; do
            value=$(openssl rand -base64 32 | tr -d '\n/+=' | cut -c1-32)
            sed -i.bak "s|^${var}=[[:space:]]*|${var}=${value}   |" deploy/.env
            rm -f deploy/.env.bak
        done

        # Verify rather than assert: if the template's format ever changes,
        # say so instead of reporting a secret that was never written.
        missing=""
        for var in JWT_SECRET DATABASE_PASSWORD NATS_PASSWORD REDIS_PASSWORD GRAFANA_ADMIN_PASSWORD; do
            if ! grep -qE "^${var}=[^[:space:]#]" deploy/.env; then
                missing="${missing} ${var}"
            fi
        done

        if [ -n "$missing" ]; then
            log_warn "Could not auto-generate:${missing}"
            log_warn "Set them by hand in deploy/.env before starting services."
        else
            log_info "Auto-generated: JWT_SECRET, DATABASE_PASSWORD, NATS_PASSWORD, REDIS_PASSWORD, GRAFANA_ADMIN_PASSWORD."
        fi

        log_warn "Created deploy/.env from template. Still needs your Proxmox details:"
        log_warn "  - PROXMOX_HOST, PROXMOX_TOKEN_ID, PROXMOX_TOKEN"
    else
        log_info "deploy/.env already exists, skipping..."
    fi
}

# Build and start services
start_services() {
    log_info "Building and starting services..."
    cd deploy

    # Build images
    docker compose build

    # Start services
    docker compose up -d

    log_info "Waiting for services to be healthy..."
    sleep 10

    # Check status
    docker compose ps
}

# Harden file permissions
harden_permissions() {
    log_info "Setting secure file permissions..."
    chmod 600 ~/kootenai/deploy/.env 2>/dev/null || true
    chmod 600 ~/kootenai/deploy/ssl/server.key 2>/dev/null || true
    log_info "File permissions secured"
}

# Print summary
# print_summary [started]  — pass "no" when start_services was skipped, so the
# summary does not advertise URLs for services that were never started.
print_summary() {
    started="${1:-yes}"
    IP=$(hostname -I | awk '{print $1}')

    echo ""
    echo "============================================================"
    if [ "$started" = "yes" ]; then
        echo -e "${GREEN}Kootenai Infrastructure VM Setup Complete!${NC}"
    else
        echo -e "${YELLOW}Kootenai setup staged — services NOT started${NC}"
    fi
    echo "============================================================"
    echo ""

    if [ "$started" != "yes" ]; then
        echo "Finish configuring deploy/.env, then run:"
        echo "  cd deploy && docker compose up -d"
        echo ""
        return
    fi

    echo "Services:"
    echo "  Web UI:     http://${IP}:3000"
    echo "  API:        http://${IP}:8080"
    echo "  API Health: http://${IP}:8080/health"
    echo ""
    echo "Management:"
    echo "  cd ~/kootenai/deploy"
    echo "  docker compose ps          # Check status"
    echo "  docker compose logs -f     # View logs"
    echo "  docker compose down        # Stop services"
    echo "  docker compose up -d       # Start services"
    echo ""
    echo "Next steps:"
    echo "  1. Edit deploy/.env with your Proxmox credentials"
    echo "  2. Restart: docker compose down && docker compose up -d"
    echo "  3. Create VM templates on Proxmox"
    echo "============================================================"
}

# Main
main() {
    echo "============================================================"
    echo "Kootenai Infrastructure VM Bootstrap"
    echo "============================================================"
    echo ""

    detect_os

    case $OS in
        rocky|centos|rhel|fedora)
            install_docker_rocky
            ;;
        ubuntu|debian)
            install_docker_debian
            ;;
        *)
            log_error "Unsupported OS: $OS"
            log_error "Supported: Rocky Linux 9, Ubuntu 22.04+, Debian 12"
            exit 1
            ;;
    esac

    setup_docker_user
    install_tools
    clone_repo
    setup_env

    # Only start services once the values the script cannot generate are set.
    # Anchor on "name= followed by nothing but whitespace or a comment", since
    # the template writes `VAR=` with a trailing inline comment.
    unset_required=""
    for var in PROXMOX_HOST PROXMOX_TOKEN_ID PROXMOX_TOKEN; do
        if ! grep -qE "^${var}=[^[:space:]#]" deploy/.env 2>/dev/null; then
            unset_required="${unset_required} ${var}"
        fi
    done

    # Before start_services, not after: the .env written above holds real
    # secrets, and the build-and-start cycle is exactly the window during which
    # they should not be sitting at default permissions.
    harden_permissions

    started=yes
    if [ -n "$unset_required" ]; then
        log_warn "Skipping service start — still unset in deploy/.env:${unset_required}"
        started=no
    else
        start_services
    fi
    print_summary "$started"
}

main "$@"
