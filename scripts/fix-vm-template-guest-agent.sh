#!/usr/bin/env bash
# Fix VM template 9100 (ubuntu-lab-wazuh) to have qemu-guest-agent installed
# and the Proxmox agent flag enabled.
#
# Strategy: Clone the template to a temporary VM, boot it, install the guest
# agent, shut down, destroy the old template, and promote the clone to the
# original VMID as a new template.
#
# Usage: ./scripts/fix-vm-template-guest-agent.sh
#
# Prerequisites:
#   - SSH access to Proxmox host (root@<PROXMOX_IP>)
#   - SSH access to the VM once booted (student@<dhcp-ip> or root)

set -euo pipefail

PROXMOX_HOST="${PROXMOX_HOST:-root@192.0.2.20}"
VMID=9100
TEMP_VMID=9199
VM_USER="student"

echo "=== Fix VM Template $VMID Guest Agent ==="

# Step 1: Ensure agent flag is set on the Proxmox side
echo "[1/7] Checking Proxmox agent flag..."
AGENT_CFG=$(ssh "$PROXMOX_HOST" "qm config $VMID" | grep -c '^agent:' || true)
if [ "$AGENT_CFG" -eq 0 ]; then
    echo "  Setting agent: 1 on VM $VMID..."
    ssh "$PROXMOX_HOST" "qm set $VMID --agent 1"
else
    echo "  agent flag already set."
fi

# Step 2: Clone the template to a temporary VM (full clone so disk is writable)
echo "[2/7] Cloning template $VMID to temporary VM $TEMP_VMID..."
ssh "$PROXMOX_HOST" "qm clone $VMID $TEMP_VMID --name tmp-guest-agent-fix --full true"

# Ensure agent flag on the clone too
ssh "$PROXMOX_HOST" "qm set $TEMP_VMID --agent 1"

# Step 3: Start the temporary VM
echo "[3/7] Starting temporary VM $TEMP_VMID..."
ssh "$PROXMOX_HOST" "qm start $TEMP_VMID"

# Step 4: Wait for VM to boot and get IP
echo "[4/7] Waiting for VM to boot (up to 120s)..."
VM_IP=""
for i in $(seq 1 24); do
    sleep 5
    VM_IP=$(ssh "$PROXMOX_HOST" "qm guest cmd $TEMP_VMID network-get-interfaces 2>/dev/null" \
        | python3 -c "
import sys, json
data = json.load(sys.stdin)
for iface in data:
    if iface.get('name') == 'lo':
        continue
    for addr in iface.get('ip-addresses', []):
        if addr.get('ip-address-type') == 'ipv4':
            print(addr['ip-address'])
            sys.exit(0)
" 2>/dev/null || true)
    if [ -n "$VM_IP" ]; then
        echo "  VM IP: $VM_IP"
        break
    fi
    echo "  Attempt $i/24..."
done

if [ -z "$VM_IP" ]; then
    echo "ERROR: Could not get VM IP. Guest agent may not be responding yet."
    echo "The clone VM $TEMP_VMID is still running — clean up manually."
    echo "Try: ssh $PROXMOX_HOST 'qm stop $TEMP_VMID && qm destroy $TEMP_VMID'"
    exit 1
fi

# Step 5: Install and enable qemu-guest-agent inside the VM
echo "[5/7] Installing qemu-guest-agent inside VM..."
ssh -o StrictHostKeyChecking=no "$VM_USER@$VM_IP" bash <<'REMOTE'
sudo apt-get update -qq
sudo apt-get install -y qemu-guest-agent
sudo systemctl enable --now qemu-guest-agent
echo "Guest agent installed and running."
REMOTE

# Step 6: Shutdown the temporary VM
echo "[6/7] Shutting down temporary VM..."
ssh "$PROXMOX_HOST" "qm shutdown $TEMP_VMID --timeout 60 && qm wait $TEMP_VMID --timeout 60"

# Step 7: Destroy old template and promote clone
echo "[7/7] Replacing old template with updated clone..."
ssh "$PROXMOX_HOST" "qm destroy $VMID --purge"
ssh "$PROXMOX_HOST" "qm set $TEMP_VMID --name ubuntu-lab-wazuh"
# Move to original VMID — Proxmox doesn't support renaming VMIDs directly,
# so we convert the clone in place as the new template.
ssh "$PROXMOX_HOST" "qm template $TEMP_VMID"

echo ""
echo "=== Done! ==="
echo "New template is VMID $TEMP_VMID (was $VMID)."
echo "NOTE: The template VMID has changed to $TEMP_VMID."
echo "Update any lab templates or configs that reference VMID $VMID."
echo ""
echo "Verify with: ssh $PROXMOX_HOST 'qm config $TEMP_VMID | grep agent'"
