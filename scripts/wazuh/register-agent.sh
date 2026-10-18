#!/bin/bash
#
# Wazuh Agent Registration Script for Kootenai
#
# This script registers a Wazuh agent with the manager using the API
# and returns the agent key for import.
#
# Usage:
#   ./register-agent.sh --manager <IP> --name <agent-name> --group <group>
#
# Output: Agent key (can be piped to agent import)
#

set -euo pipefail

# Defaults
WAZUH_MANAGER="${WAZUH_MANAGER:-}"
WAZUH_API_PORT="${WAZUH_API_PORT:-55000}"
WAZUH_API_USER="${WAZUH_API_USER:-wazuh}"
WAZUH_API_PASS="${WAZUH_API_PASS:-}"
WAZUH_AGENT_NAME="${WAZUH_AGENT_NAME:-}"
WAZUH_AGENT_IP="${WAZUH_AGENT_IP:-any}"
WAZUH_GROUP="${WAZUH_GROUP:-default}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1" >&2
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

usage() {
    cat >&2 <<EOF
Usage: $0 [OPTIONS]

Options:
    --manager <IP>      Wazuh Manager IP address (required)
    --api-port <port>   Wazuh API port (default: 55000)
    --api-user <user>   API username (default: wazuh)
    --api-pass <pass>   API password (required)
    --name <name>       Agent name (required)
    --ip <ip>           Agent IP (default: any)
    --group <group>     Agent group (default: default)
    --json              Output full JSON response
    --help              Show this help message

Environment Variables:
    WAZUH_MANAGER       Manager IP address
    WAZUH_API_PORT      API port
    WAZUH_API_USER      API username
    WAZUH_API_PASS      API password
    WAZUH_AGENT_NAME    Agent name
    WAZUH_AGENT_IP      Agent IP
    WAZUH_GROUP         Agent group
EOF
    exit 1
}

OUTPUT_JSON=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --manager)
            WAZUH_MANAGER="$2"
            shift 2
            ;;
        --api-port)
            WAZUH_API_PORT="$2"
            shift 2
            ;;
        --api-user)
            WAZUH_API_USER="$2"
            shift 2
            ;;
        --api-pass)
            WAZUH_API_PASS="$2"
            shift 2
            ;;
        --name)
            WAZUH_AGENT_NAME="$2"
            shift 2
            ;;
        --ip)
            WAZUH_AGENT_IP="$2"
            shift 2
            ;;
        --group)
            WAZUH_GROUP="$2"
            shift 2
            ;;
        --json)
            OUTPUT_JSON=true
            shift
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

if [[ -z "$WAZUH_API_PASS" ]]; then
    log_error "API password is required"
    usage
fi

if [[ -z "$WAZUH_AGENT_NAME" ]]; then
    log_error "Agent name is required"
    usage
fi

# Check for required commands
for cmd in curl jq; do
    if ! command -v $cmd &> /dev/null; then
        log_error "$cmd is required but not installed"
        exit 1
    fi
done

API_BASE="https://${WAZUH_MANAGER}:${WAZUH_API_PORT}"

# Get authentication token
get_token() {
    local response
    response=$(curl -s -k -X POST \
        -u "${WAZUH_API_USER}:${WAZUH_API_PASS}" \
        "${API_BASE}/security/user/authenticate" 2>&1)

    local token
    token=$(echo "$response" | jq -r '.data.token // empty')

    if [[ -z "$token" ]]; then
        log_error "Failed to authenticate with Wazuh API"
        log_error "Response: $response"
        exit 1
    fi

    echo "$token"
}

# Check if agent exists
agent_exists() {
    local token="$1"
    local name="$2"

    local response
    response=$(curl -s -k -X GET \
        -H "Authorization: Bearer $token" \
        "${API_BASE}/agents?name=${name}" 2>&1)

    local count
    count=$(echo "$response" | jq -r '.data.total_affected_items // 0')

    [[ "$count" -gt 0 ]]
}

# Register new agent
register_agent() {
    local token="$1"

    log_info "Registering agent: $WAZUH_AGENT_NAME"

    local response
    response=$(curl -s -k -X POST \
        -H "Authorization: Bearer $token" \
        -H "Content-Type: application/json" \
        -d "{\"name\": \"${WAZUH_AGENT_NAME}\", \"ip\": \"${WAZUH_AGENT_IP}\"}" \
        "${API_BASE}/agents" 2>&1)

    local agent_id
    agent_id=$(echo "$response" | jq -r '.data.id // empty')

    if [[ -z "$agent_id" ]]; then
        log_error "Failed to register agent"
        log_error "Response: $response"
        exit 1
    fi

    log_info "Agent registered with ID: $agent_id"
    echo "$agent_id"
}

# Assign agent to group
assign_group() {
    local token="$1"
    local agent_id="$2"

    if [[ "$WAZUH_GROUP" == "default" ]]; then
        return
    fi

    log_info "Assigning agent to group: $WAZUH_GROUP"

    local response
    response=$(curl -s -k -X PUT \
        -H "Authorization: Bearer $token" \
        "${API_BASE}/agents/${agent_id}/group/${WAZUH_GROUP}" 2>&1)

    local error
    error=$(echo "$response" | jq -r '.error // 0')

    if [[ "$error" != "0" ]]; then
        log_error "Failed to assign agent to group"
        log_error "Response: $response"
        # Don't exit, agent is still registered
    fi
}

# Get agent key
get_agent_key() {
    local token="$1"
    local agent_id="$2"

    local response
    response=$(curl -s -k -X GET \
        -H "Authorization: Bearer $token" \
        "${API_BASE}/agents/${agent_id}/key" 2>&1)

    local key
    key=$(echo "$response" | jq -r '.data.affected_items[0].key // empty')

    if [[ -z "$key" ]]; then
        log_error "Failed to get agent key"
        log_error "Response: $response"
        exit 1
    fi

    echo "$key"
}

# Main
main() {
    log_info "Registering Wazuh agent with manager at $WAZUH_MANAGER"

    # Get API token
    log_info "Authenticating with Wazuh API..."
    local token
    token=$(get_token)

    # Check if agent already exists
    if agent_exists "$token" "$WAZUH_AGENT_NAME"; then
        log_error "Agent with name '$WAZUH_AGENT_NAME' already exists"
        exit 1
    fi

    # Register agent
    local agent_id
    agent_id=$(register_agent "$token")

    # Assign to group
    assign_group "$token" "$agent_id"

    # Get key
    log_info "Retrieving agent key..."
    local key
    key=$(get_agent_key "$token" "$agent_id")

    if $OUTPUT_JSON; then
        echo "{\"id\": \"$agent_id\", \"name\": \"$WAZUH_AGENT_NAME\", \"key\": \"$key\"}"
    else
        # Output just the key (for piping to agent import)
        echo "$key"
    fi

    log_info "Agent registration complete!"
}

main "$@"
