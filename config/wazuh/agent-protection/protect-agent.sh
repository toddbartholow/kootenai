#!/bin/bash
#
# Kootenai Wazuh Agent Protection Script
#
# This script hardens the Wazuh agent installation to make it
# more difficult for students to tamper with during lab sessions.
#
# Run this script on lab VM templates before creating snapshots.
#
# Usage: sudo ./protect-agent.sh
#

set -e

echo "=== Kootenai Wazuh Agent Protection ==="
echo ""

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo "ERROR: This script must be run as root"
    exit 1
fi

# Check if Wazuh agent is installed
if [ ! -d /var/ossec ]; then
    echo "ERROR: Wazuh agent not found at /var/ossec"
    exit 1
fi

echo "[1/7] Protecting Wazuh configuration files..."

# Make configuration files immutable
chattr +i /var/ossec/etc/ossec.conf 2>/dev/null || true
chattr +i /var/ossec/etc/client.keys 2>/dev/null || true
chattr +i /var/ossec/etc/internal_options.conf 2>/dev/null || true
chattr +i /var/ossec/etc/local_internal_options.conf 2>/dev/null || true

echo "[2/7] Protecting Wazuh binaries..."

# Make binaries immutable
for bin in /var/ossec/bin/*; do
    chattr +i "$bin" 2>/dev/null || true
done

echo "[3/7] Setting up systemd service protection..."

# Create systemd override to prevent stopping
mkdir -p /etc/systemd/system/wazuh-agent.service.d/
cat > /etc/systemd/system/wazuh-agent.service.d/kootenai-protect.conf << 'EOF'
[Unit]
# Prevent manual stopping
RefuseManualStop=true

[Service]
# Restart always if it stops
Restart=always
RestartSec=5

# Prevent OOM killer
OOMScoreAdjust=-1000

# Make service critical
FailureAction=reboot-force
EOF

systemctl daemon-reload

echo "[4/7] Configuring audit rules for Wazuh monitoring..."

# Create audit rules to monitor tampering attempts
cat > /etc/audit/rules.d/99-kootenai-wazuh.rules << 'EOF'
# Monitor Wazuh configuration directory
-w /var/ossec/etc/ -p wa -k wazuh_config

# Monitor Wazuh binaries
-w /var/ossec/bin/ -p wa -k wazuh_binary

# Monitor attempts to kill Wazuh processes
-a always,exit -F arch=b64 -S kill -S tkill -S tgkill -F a1=9 -k process_kill
-a always,exit -F arch=b64 -S kill -S tkill -S tgkill -F a1=15 -k process_kill

# Monitor systemctl commands
-w /usr/bin/systemctl -p x -k systemctl_usage

# Monitor service commands
-w /usr/sbin/service -p x -k service_usage

# Monitor iptables for network blocking
-w /usr/sbin/iptables -p x -k firewall_change
-w /usr/sbin/ip6tables -p x -k firewall_change
-w /usr/sbin/nft -p x -k firewall_change
EOF

# Reload audit rules
auditctl -R /etc/audit/rules.d/99-kootenai-wazuh.rules 2>/dev/null || true

echo "[5/7] Setting up process monitoring..."

# Create a watchdog script
cat > /usr/local/bin/wazuh-watchdog.sh << 'WATCHDOG'
#!/bin/bash
# Wazuh Agent Watchdog for Kootenai
# Monitors agent status and restarts if stopped

while true; do
    if ! pgrep -x "wazuh-agentd" > /dev/null; then
        logger -t wazuh-watchdog "Wazuh agent not running - attempting restart"
        systemctl start wazuh-agent 2>/dev/null || /var/ossec/bin/wazuh-control start
    fi
    sleep 30
done
WATCHDOG

chmod +x /usr/local/bin/wazuh-watchdog.sh

# Create systemd service for watchdog
cat > /etc/systemd/system/wazuh-watchdog.service << 'WDSERVICE'
[Unit]
Description=Wazuh Agent Watchdog for Kootenai
After=wazuh-agent.service

[Service]
Type=simple
ExecStart=/usr/local/bin/wazuh-watchdog.sh
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
WDSERVICE

systemctl daemon-reload
systemctl enable wazuh-watchdog.service
systemctl start wazuh-watchdog.service

echo "[6/7] Restricting sudo access to Wazuh..."

# Add sudoers rule to prevent stopping Wazuh
cat > /etc/sudoers.d/kootenai-wazuh << 'SUDOERS'
# Kootenai: Prevent students from tampering with Wazuh
# Deny specific commands that could disable monitoring
Cmnd_Alias WAZUH_TAMPERING = /usr/bin/systemctl *wazuh*, \
                              /usr/bin/systemctl *ossec*, \
                              /usr/sbin/service wazuh*, \
                              /usr/sbin/service ossec*, \
                              /bin/kill -9 *, \
                              /bin/kill -KILL *, \
                              /usr/bin/killall *, \
                              /usr/bin/pkill *wazuh*, \
                              /usr/bin/pkill *ossec*, \
                              /usr/bin/chattr *ossec*, \
                              /usr/bin/chattr */var/ossec*

# Apply to lab users (adjust group as needed)
%labusers ALL = ALL, !WAZUH_TAMPERING
SUDOERS

chmod 440 /etc/sudoers.d/kootenai-wazuh
visudo -c

echo "[7/7] Creating health check endpoint..."

# Create a simple health check that active verification can query
cat > /usr/local/bin/wazuh-health.sh << 'HEALTH'
#!/bin/bash
# Health check for Wazuh agent
# Returns 0 if healthy, 1 if not

# Check if agent process is running
if ! pgrep -x "wazuh-agentd" > /dev/null; then
    echo "ERROR: wazuh-agentd not running"
    exit 1
fi

# Check if agent is connected (via status file)
if [ -f /var/ossec/var/run/wazuh-agentd.state ]; then
    status=$(cat /var/ossec/var/run/wazuh-agentd.state 2>/dev/null | grep -i status | head -1)
    if [[ "$status" == *"connected"* ]]; then
        echo "OK: Agent connected"
        exit 0
    fi
fi

# Fallback: check if control script reports running
if /var/ossec/bin/wazuh-control status 2>/dev/null | grep -q "is running"; then
    echo "OK: Agent running"
    exit 0
fi

echo "ERROR: Agent status unknown"
exit 1
HEALTH

chmod +x /usr/local/bin/wazuh-health.sh

echo ""
echo "=== Protection Complete ==="
echo ""
echo "Summary of protections applied:"
echo "  - Configuration files made immutable (chattr +i)"
echo "  - Binaries made immutable"
echo "  - systemd service protected (RefuseManualStop)"
echo "  - Audit rules installed for tampering detection"
echo "  - Watchdog service enabled"
echo "  - Sudo restrictions applied"
echo "  - Health check endpoint created"
echo ""
echo "IMPORTANT: Create a VM snapshot after running this script!"
echo "Students will revert to this protected state when they reset."
echo ""
