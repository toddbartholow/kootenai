# Wazuh Security Hardening Guide

This guide covers the security measures implemented to prevent students from tampering with Wazuh agents during lab sessions, ensuring checkpoint verification integrity.

## Overview

The Kootenai platform uses Wazuh for passive checkpoint detection by monitoring file changes, command execution, and service states on lab VMs. Students might attempt to bypass checkpoint verification by:

1. Disabling the Wazuh agent
2. Modifying agent configuration
3. Blocking network traffic to the Wazuh manager
4. Spoofing events to fake checkpoint completion

This guide covers the multi-layered security approach to detect and prevent such tampering.

## Security Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Security Layers                               │
├─────────────────────────────────────────────────────────────────────┤
│  Layer 1: Webhook Authentication                                     │
│  - Secret validation (X-Wazuh-Webhook-Secret header)                │
│  - IP whitelisting                                                   │
│  - Rate limiting                                                     │
│  - Timestamp validation (replay attack prevention)                   │
├─────────────────────────────────────────────────────────────────────┤
│  Layer 2: Agent Health Monitoring                                    │
│  - Continuous heartbeat tracking                                     │
│  - Disconnect detection and alerting                                 │
│  - Automatic penalty application                                     │
├─────────────────────────────────────────────────────────────────────┤
│  Layer 3: Active Checkpoint Verification                             │
│  - Direct VM queries via QEMU guest agent                            │
│  - Cross-verification with passive events                            │
│  - Mismatch detection for spoofed events                            │
├─────────────────────────────────────────────────────────────────────┤
│  Layer 4: VM-Level Protection                                        │
│  - Immutable configuration files                                     │
│  - Protected systemd services                                        │
│  - Audit rules for tampering detection                               │
│  - Watchdog process                                                  │
├─────────────────────────────────────────────────────────────────────┤
│  Layer 5: Tampering Detection Rules                                  │
│  - Custom Wazuh rules for specific tampering patterns                │
│  - Automatic scoring penalties                                       │
│  - Session flagging for instructor review                           │
└─────────────────────────────────────────────────────────────────────┘
```

## Configuration

### 1. Webhook Security

Configure webhook security in your API configuration:

```yaml
wazuh:
  security:
    # Shared secret between Wazuh manager and API
    webhook_secret: "your-secure-32-char-secret-here"

    # Allowed IP addresses/networks for webhook requests
    allowed_ips:
      - "10.0.0.0/8"
      - "192.168.1.50"  # Wazuh manager IP

    # Rate limiting
    rate_limit_per_minute: 1000

    # Timestamp validation (prevent replay attacks)
    max_timestamp_drift: "5m"
    require_signature: true
```

### 2. Wazuh Integration Script

Update the integration script on the Wazuh manager to send the webhook secret:

```bash
# Copy the integration script
cp config/wazuh/custom-integrations/custom-kootenai.py \
   /var/ossec/integrations/custom-kootenai

# Set permissions
chmod 750 /var/ossec/integrations/custom-kootenai
chown root:wazuh /var/ossec/integrations/custom-kootenai
```

Configure in `/var/ossec/etc/ossec.conf`:

```xml
<integration>
  <name>custom-kootenai</name>
  <hook_url>https://your-api-server:8080/api/v1/events/wazuh</hook_url>
  <api_key>your-secure-32-char-secret-here</api_key>
  <level>3</level>
  <alert_format>json</alert_format>
</integration>
```

### 3. Tampering Detection Rules

Install the tampering detection rules on the Wazuh manager:

```bash
# Copy rules
cp config/wazuh/rules/kootenai_tampering.xml \
   /var/ossec/etc/rules/

# Add to ossec.conf
# <include>rules/kootenai_tampering.xml</include>

# Restart manager
systemctl restart wazuh-manager
```

### 4. Agent Protection

Run the protection script on each lab VM template:

```bash
# On the lab VM template (as root)
./config/wazuh/agent-protection/protect-agent.sh

# Create a snapshot after protection is applied
# Students will revert to this protected state
```

### 5. Tampering Penalties

Configure tampering penalties in your application:

```yaml
checkpoint:
  tampering:
    enabled: true

    # Points deducted per tampering incident (percentage)
    penalty_per_incident: 10

    # Maximum total penalty that can be applied
    max_penalty: 50

    # How long agent can be offline before penalty
    offline_threshold: "2m"

    # Grace period after pod starts
    grace_period_on_start: "5m"

    # Restarts that trigger tampering alert
    restart_threshold: 3

    # Auto-flag sessions for instructor review
    auto_flag_for_review: true
```

## Tampering Detection Events

The following Wazuh rules detect tampering attempts:

| Rule ID | Level | Description |
|---------|-------|-------------|
| 100200 | 15 | Attempt to stop Wazuh agent service |
| 100201-02 | 14 | systemctl/service commands targeting Wazuh |
| 100210-11 | 15/14 | Kill commands targeting Wazuh processes |
| 100220-23 | 14/15/12 | Configuration file modifications |
| 100230-32 | 13/12 | Network tampering (firewall, hosts file) |
| 100240-41 | 15 | Binary modifications |
| 100250-51 | 13/12 | Privilege escalation for tampering |
| 100260-61 | 14/13 | Process hiding attempts |
| 100290 | 15 | Multiple tampering attempts (correlation) |

## Active Verification

The active verification system provides a secondary check that directly queries VMs:

```go
// Verification is performed by executing commands on the VM
// via QEMU guest agent, which is harder to spoof than events

verifier := checkpoint.NewActiveVerifier(orchestrator, logger, config)

// Verify a checkpoint
result, err := verifier.VerifyCheckpoint(ctx, podID, checkpoint)

// Cross-verify passive (event) and active (direct query) results
result, passed, err := verifier.CrossVerify(ctx, podID, checkpoint, passedByEvent)
```

Supported verification methods:
- File existence checks
- File content verification (contains/regex)
- Service state verification
- Package installation checks
- User existence verification
- File permission checks

## Monitoring Dashboard

Administrators can monitor tampering incidents through the API:

```bash
# Get flagged sessions
curl -H "Authorization: Bearer $TOKEN" \
  https://api-server/api/v1/admin/tampering/flagged

# Get session tampering state
curl -H "Authorization: Bearer $TOKEN" \
  https://api-server/api/v1/sessions/{sessionId}/tampering

# Acknowledge an incident
curl -X POST -H "Authorization: Bearer $TOKEN" \
  https://api-server/api/v1/admin/tampering/{sessionId}/acknowledge/{incidentId}
```

## Instructor Review

When tampering is detected:

1. The session is automatically flagged for review
2. Scoring penalties are applied based on severity
3. Instructors receive notifications (if configured)
4. Full incident details are available in the admin dashboard

Instructors can:
- Review incident details
- Override penalties if tampering was unintentional
- Submit final grades after review

## Best Practices

1. **VM Templates**: Always run the protection script on templates before creating snapshots

2. **Secrets Management**: Store webhook secrets securely (environment variables or secrets manager)

3. **Network Isolation**: Ensure students cannot block traffic between VMs and Wazuh manager

4. **Regular Audits**: Review flagged sessions regularly

5. **Student Communication**: Inform students that monitoring is in place and tampering will affect grades

6. **Testing**: Test tampering detection in a staging environment before production

## Troubleshooting

### Agent Not Reporting

1. Check agent status: `/var/ossec/bin/wazuh-control status`
2. Verify manager connectivity: `telnet manager-ip 1514`
3. Check agent logs: `/var/ossec/logs/ossec.log`

### False Positive Tampering Alerts

1. Review the specific rule that triggered
2. Check if it's during the grace period
3. Adjust thresholds if needed

### Webhook Validation Failures

1. Verify secret matches on both ends
2. Check IP is in allowlist
3. Verify timestamps are synchronized (NTP)

## Security Considerations

- The webhook secret should be at least 32 characters
- Use HTTPS for webhook endpoints
- Rotate secrets periodically
- Monitor for unusual patterns in tampering attempts
- Keep Wazuh rules updated for new evasion techniques
