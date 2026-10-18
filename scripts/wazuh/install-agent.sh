#!/bin/bash
#
# Wazuh Agent Installation Script for Kootenai VMs
#
# This script installs and configures the Wazuh agent on a lab VM.
# It should be run during VM template preparation or pod provisioning.
#
# Usage:
#   ./install-agent.sh --manager <IP> --name <agent-name> --group <group> [--key <key>]
#
# Environment variables (alternative to flags):
#   WAZUH_MANAGER     - Manager IP address
#   WAZUH_AGENT_NAME  - Agent name (hostname if not specified)
#   WAZUH_GROUP       - Agent group (for shared config)
#   WAZUH_KEY         - Registration key (optional, for pre-keyed registration)
#   WAZUH_VERSION     - Wazuh version to install (default: 4.14.1)
#

set -euo pipefail

# Defaults
WAZUH_VERSION="${WAZUH_VERSION:-4.14.1}"
WAZUH_MANAGER="${WAZUH_MANAGER:-}"
WAZUH_AGENT_NAME="${WAZUH_AGENT_NAME:-$(hostname)}"
WAZUH_GROUP="${WAZUH_GROUP:-default}"
WAZUH_KEY="${WAZUH_KEY:-}"

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

usage() {
    cat <<EOF
Usage: $0 [OPTIONS]

Options:
    --manager <IP>      Wazuh Manager IP address (required)
    --name <name>       Agent name (default: hostname)
    --group <group>     Agent group (default: default)
    --key <key>         Pre-shared registration key
    --version <ver>     Wazuh version (default: 4.14.1)
    --help              Show this help message

Environment Variables:
    WAZUH_MANAGER       Manager IP address
    WAZUH_AGENT_NAME    Agent name
    WAZUH_GROUP         Agent group
    WAZUH_KEY           Registration key
    WAZUH_VERSION       Wazuh version
EOF
    exit 1
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --manager)
            WAZUH_MANAGER="$2"
            shift 2
            ;;
        --name)
            WAZUH_AGENT_NAME="$2"
            shift 2
            ;;
        --group)
            WAZUH_GROUP="$2"
            shift 2
            ;;
        --key)
            WAZUH_KEY="$2"
            shift 2
            ;;
        --version)
            WAZUH_VERSION="$2"
            shift 2
            ;;
        --help)
            usage
            ;;
        *)
            log_error "Unknown option: $1"
            usage
            ;;
    esac
done

# Validate required parameters
if [[ -z "$WAZUH_MANAGER" ]]; then
    log_error "Manager IP address is required"
    usage
fi

# Detect OS
detect_os() {
    if [[ -f /etc/os-release ]]; then
        . /etc/os-release
        OS=$ID
        VERSION_ID=${VERSION_ID:-""}
    elif [[ -f /etc/redhat-release ]]; then
        OS="rhel"
    elif [[ -f /etc/debian_version ]]; then
        OS="debian"
    else
        log_error "Unsupported OS"
        exit 1
    fi
    log_info "Detected OS: $OS $VERSION_ID"
}

# Install on Debian/Ubuntu
install_debian() {
    log_info "Installing Wazuh agent on Debian/Ubuntu..."

    # Add Wazuh repository
    curl -s https://packages.wazuh.com/key/GPG-KEY-WAZUH | gpg --dearmor -o /usr/share/keyrings/wazuh-archive-keyring.gpg

    echo "deb [signed-by=/usr/share/keyrings/wazuh-archive-keyring.gpg] https://packages.wazuh.com/4.x/apt/ stable main" > /etc/apt/sources.list.d/wazuh.list

    apt-get update
    WAZUH_MANAGER="$WAZUH_MANAGER" WAZUH_AGENT_NAME="$WAZUH_AGENT_NAME" WAZUH_AGENT_GROUP="$WAZUH_GROUP" apt-get install -y wazuh-agent

    # Prevent automatic updates
    sed -i "s/^deb/#deb/" /etc/apt/sources.list.d/wazuh.list
    apt-get update
}

# Install on RHEL/CentOS/Rocky
install_rhel() {
    log_info "Installing Wazuh agent on RHEL/CentOS..."

    rpm --import https://packages.wazuh.com/key/GPG-KEY-WAZUH

    cat > /etc/yum.repos.d/wazuh.repo <<EOF
[wazuh]
gpgcheck=1
gpgkey=https://packages.wazuh.com/key/GPG-KEY-WAZUH
enabled=1
name=EL-\$releasever - Wazuh
baseurl=https://packages.wazuh.com/4.x/yum/
protect=1
EOF

    WAZUH_MANAGER="$WAZUH_MANAGER" WAZUH_AGENT_NAME="$WAZUH_AGENT_NAME" WAZUH_AGENT_GROUP="$WAZUH_GROUP" yum install -y wazuh-agent

    # Disable repo to prevent automatic updates
    sed -i 's/^enabled=1/enabled=0/' /etc/yum.repos.d/wazuh.repo
}

# Configure agent
configure_agent() {
    log_info "Configuring Wazuh agent..."

    local config_file="/var/ossec/etc/ossec.conf"

    # Update manager address
    if [[ -f "$config_file" ]]; then
        sed -i "s|<address>.*</address>|<address>$WAZUH_MANAGER</address>|g" "$config_file"
    fi

    # Import key if provided
    if [[ -n "$WAZUH_KEY" ]]; then
        log_info "Importing agent key..."
        /var/ossec/bin/manage_agents -i "$WAZUH_KEY"
    fi
}

# Enable and start service
start_agent() {
    log_info "Starting Wazuh agent service..."

    systemctl daemon-reload
    systemctl enable wazuh-agent
    systemctl start wazuh-agent

    # Wait for agent to connect
    sleep 5

    if systemctl is-active --quiet wazuh-agent; then
        log_info "Wazuh agent started successfully"
    else
        log_error "Failed to start Wazuh agent"
        systemctl status wazuh-agent
        exit 1
    fi
}

# Verify connection to manager
verify_connection() {
    log_info "Verifying connection to Wazuh Manager..."

    local status
    status=$(/var/ossec/bin/agent_control -i 000 2>/dev/null | grep "Status" || echo "")

    if [[ "$status" == *"Active"* ]] || [[ "$status" == *"Connected"* ]]; then
        log_info "Agent connected to manager successfully"
    else
        log_warn "Agent may not be connected yet. Check /var/ossec/logs/ossec.log"
    fi
}

# Main
main() {
    log_info "Starting Wazuh agent installation..."
    log_info "Manager: $WAZUH_MANAGER"
    log_info "Agent Name: $WAZUH_AGENT_NAME"
    log_info "Group: $WAZUH_GROUP"
    log_info "Version: $WAZUH_VERSION"

    # Check if running as root
    if [[ $EUID -ne 0 ]]; then
        log_error "This script must be run as root"
        exit 1
    fi

    detect_os

    case $OS in
        ubuntu|debian)
            install_debian
            ;;
        rhel|centos|rocky|almalinux|fedora)
            install_rhel
            ;;
        *)
            log_error "Unsupported OS: $OS"
            exit 1
            ;;
    esac

    configure_agent
    start_agent
    verify_connection

    log_info "Wazuh agent installation complete!"
}

main "$@"
