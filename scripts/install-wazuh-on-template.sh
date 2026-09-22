#!/usr/bin/env bash
#
# Install Wazuh agent on VM template 9002
#
# Strategy: Clone the template to a temporary VM, boot it, install the Wazuh
# agent, shut down, destroy the old template, and promote the clone.
#
# Usage: ./scripts/install-wazuh-on-template.sh
#

set -euo pipefail

PROXMOX_HOST="root@<PROXMOX_IP>"
VMID=9002
TEMP_VMID=9299
WAZUH_MANAGER="<INFRA_VM_IP>"
VM_USER="labadmin"
VM_PASS="labpass123"

echo "=== Install Wazuh Agent on Template $VMID ==="

# Step 1: Clone the template to a temporary VM
echo "[1/7] Cloning template $VMID to temporary VM $TEMP_VMID..."
ssh "$PROXMOX_HOST" "qm clone $VMID $TEMP_VMID --name tmp-wazuh-install --full true"

# Ensure agent flag is set
ssh "$PROXMOX_HOST" "qm set $TEMP_VMID --agent 1"

# Step 2: Start the temporary VM
echo "[2/7] Starting temporary VM $TEMP_VMID..."
ssh "$PROXMOX_HOST" "qm start $TEMP_VMID"

# Step 3: Wait for VM to boot and get IP
echo "[3/7] Waiting for VM to boot (up to 120s)..."
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
        echo "VM IP: $VM_IP"
        break
    fi
    echo "Attempt $i/24..."
done

if [ -z "$VM_IP" ]; then
    echo "ERROR: Could not get VM IP. Guest agent may not be responding."
    echo "The clone VM $TEMP_VMID is still running — clean up manually."
    exit 1
fi

# Step 4: Wait for SSH to be ready (via jump host)
JUMP_HOST="labadmin@<INFRA_VM_IP>"
echo "[4/7] Waiting for SSH to be ready..."
for i in $(seq 1 12); do
    if ssh -o StrictHostKeyChecking=no "$JUMP_HOST" "sshpass -p '$VM_PASS' ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 $VM_USER@$VM_IP 'echo ready'" 2>/dev/null; then
        echo "SSH ready"
        break
    fi
    echo "Attempt $i/12..."
    sleep 5
done

# Step 5: Install Wazuh agent (via jump host)
echo "[5/7] Installing Wazuh agent..."
ssh -o StrictHostKeyChecking=no "$JUMP_HOST" "sshpass -p '$VM_PASS' ssh -o StrictHostKeyChecking=no $VM_USER@$VM_IP" bash <<REMOTE
set -e

echo "Adding Wazuh repository..."
curl -s https://packages.wazuh.com/key/GPG-KEY-WAZUH | sudo gpg --dearmor -o /usr/share/keyrings/wazuh-archive-keyring.gpg

echo "deb [signed-by=/usr/share/keyrings/wazuh-archive-keyring.gpg] https://packages.wazuh.com/4.x/apt/ stable main" | sudo tee /etc/apt/sources.list.d/wazuh.list

echo "Installing wazuh-agent..."
sudo apt-get update -qq
sudo WAZUH_MANAGER="$WAZUH_MANAGER" apt-get install -y wazuh-agent

# Disable repo to prevent auto-updates
sudo sed -i "s/^deb/#deb/" /etc/apt/sources.list.d/wazuh.list

# Configure the agent
echo "Configuring Wazuh agent..."
sudo sed -i "s|<address>.*</address>|<address>$WAZUH_MANAGER</address>|g" /var/ossec/etc/ossec.conf

# Add FIM configuration for lab paths
sudo tee -a /var/ossec/etc/ossec.conf > /dev/null <<'OSSEC_CONFIG'
<!-- Lab file integrity monitoring -->
<ossec_config>
  <syscheck>
    <directories check_all="yes" realtime="yes">/home</directories>
    <directories check_all="yes" realtime="yes">/etc</directories>
  </syscheck>
</ossec_config>
OSSEC_CONFIG

# Enable and start the service
echo "Enabling wazuh-agent service..."
sudo systemctl daemon-reload
sudo systemctl enable wazuh-agent

# Don't start it now - it will start on next boot with proper agent name
echo "Wazuh agent installed successfully!"
REMOTE

# Step 6: Clean up and shut down
echo "[6/7] Cleaning cloud-init and shutting down..."
ssh -o StrictHostKeyChecking=no "$JUMP_HOST" "sshpass -p '$VM_PASS' ssh -o StrictHostKeyChecking=no $VM_USER@$VM_IP" bash <<'REMOTE'
# Clean cloud-init so it runs fresh on clones
sudo cloud-init clean --logs 2>/dev/null || true
sudo rm -rf /var/lib/cloud/instances/* 2>/dev/null || true

# Clean machine-id so clones get unique IDs
sudo truncate -s 0 /etc/machine-id 2>/dev/null || true
sudo rm -f /var/lib/dbus/machine-id 2>/dev/null || true

# Remove SSH host keys (will regenerate on boot)
sudo rm -f /etc/ssh/ssh_host_* 2>/dev/null || true

# Shut down
sudo shutdown -h now
REMOTE

# Wait for shutdown
echo "Waiting for VM to shut down..."
sleep 10
for i in $(seq 1 12); do
    STATUS=$(ssh "$PROXMOX_HOST" "qm status $TEMP_VMID" | awk '{print $2}')
    if [ "$STATUS" = "stopped" ]; then
        echo "VM stopped"
        break
    fi
    sleep 5
done

# Step 7: Replace old template
echo "[7/7] Replacing old template with updated clone..."
ssh "$PROXMOX_HOST" "qm destroy $VMID --purge"
ssh "$PROXMOX_HOST" "qm set $TEMP_VMID --name ubuntu-lab-template"
ssh "$PROXMOX_HOST" "qm template $TEMP_VMID"

echo ""
echo "=== Done! ==="
echo "New template is VMID $TEMP_VMID (was $VMID)."
echo ""
echo "IMPORTANT: Update TEMPLATE_VMIDS environment variable:"
echo "Change ubuntu-22.04-server:$VMID to ubuntu-22.04-server:$TEMP_VMID"
echo ""
echo "Or rename the template back to the original VMID (requires manual steps)."
