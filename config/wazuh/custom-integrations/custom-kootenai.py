#!/usr/bin/env python3
"""
Kootenai Custom Integration for Wazuh Manager

This script is called by the Wazuh integrator module to forward
alerts to the Kootenai API.

Installation:
  1. Copy to /var/ossec/integrations/custom-kootenai
  2. chmod 750 /var/ossec/integrations/custom-kootenai
  3. chown root:wazuh /var/ossec/integrations/custom-kootenai
  4. Add integration block to ossec.conf
  5. Restart Wazuh Manager
"""

import hashlib
import hmac
import json
import sys
import time
import os
import urllib.request
import urllib.error
import ssl

# Read configuration
ALERT_FILE = sys.argv[1]
API_KEY = sys.argv[2] if len(sys.argv) > 2 else ""
HOOK_URL = sys.argv[3] if len(sys.argv) > 3 else ""

# Debug logging (set to True for troubleshooting)
DEBUG = os.getenv("KOOTENAI_DEBUG", "false").lower() == "true"
LOG_FILE = "/var/ossec/logs/integrations.log"


def log(message: str) -> None:
    """Log message to integrations log file."""
    if DEBUG:
        with open(LOG_FILE, "a") as f:
            f.write(f"{time.strftime('%Y-%m-%d %H:%M:%S')} kootenai: {message}\n")


def read_alert() -> dict:
    """Read the alert JSON from the file passed by Wazuh."""
    try:
        with open(ALERT_FILE, "r") as f:
            return json.load(f)
    except Exception as e:
        log(f"Error reading alert file: {e}")
        raise


def send_to_kootenai(alert: dict) -> bool:
    """Send alert to Kootenai API."""
    if not HOOK_URL:
        log("No hook URL configured")
        return False

    payload = json.dumps({"alert": alert}).encode("utf-8")

    headers = {
        "Content-Type": "application/json",
        "User-Agent": "Wazuh-Kootenai-Integration/1.0",
    }

    # SECURITY: Send webhook authentication
    # API_KEY is used as the webhook secret (passed as 2nd argument from ossec.conf)
    if API_KEY:
        # Compute HMAC-SHA256 signature of the payload for tamper-proof verification
        signature = hmac.new(
            API_KEY.encode("utf-8"),
            payload,
            hashlib.sha256,
        ).hexdigest()
        headers["X-Wazuh-Signature"] = signature
        # Keep shared secret header for backward compatibility during rollout
        headers["X-Wazuh-Webhook-Secret"] = API_KEY

    try:
        # Create request
        req = urllib.request.Request(
            HOOK_URL,
            data=payload,
            headers=headers,
            method="POST"
        )

        # Allow self-signed certs in dev (remove in production)
        ctx = ssl.create_default_context()
        if os.getenv("KOOTENAI_INSECURE", "false").lower() == "true":
            ctx.check_hostname = False
            ctx.verify_mode = ssl.CERT_NONE

        # Send request with timeout
        with urllib.request.urlopen(req, timeout=10, context=ctx) as response:
            status = response.getcode()
            log(f"Alert sent successfully, status: {status}")
            return status in (200, 202)

    except urllib.error.HTTPError as e:
        log(f"HTTP error sending alert: {e.code} {e.reason}")
        return False
    except urllib.error.URLError as e:
        log(f"URL error sending alert: {e.reason}")
        return False
    except Exception as e:
        log(f"Error sending alert: {e}")
        return False


def should_forward_alert(alert: dict) -> bool:
    """
    Determine if an alert should be forwarded to Kootenai.

    Filter rules:
    - Always forward FIM (syscheck) alerts
    - Forward authentication events
    - Forward sudo/audit events (for command execution checkpoints)
    - Forward high-level alerts (level >= 7)
    - Forward specific rule groups
    """
    rule = alert.get("rule", {})
    groups = rule.get("groups", [])
    level = rule.get("level", 0)

    # Always forward syscheck (FIM) events
    if "syscheck" in groups:
        return True

    # Forward authentication events
    auth_groups = {"authentication_success", "authentication_failed", "sshd", "pam", "adduser"}
    if auth_groups & set(groups):
        return True

    # Forward sudo events (for command_executed checkpoints)
    if "sudo" in groups:
        return True

    # Forward audit events (for command_executed checkpoints)
    # This includes auditd execve events that track command execution
    audit_groups = {"audit", "audit_command", "audit_watch"}
    if audit_groups & set(groups):
        return True

    # Forward if alert has command data (sudo commands)
    if alert.get("data", {}).get("command"):
        return True

    # Forward package management events
    pkg_groups = {"dpkg", "yum", "rpm", "apt"}
    if pkg_groups & set(groups):
        return True

    # Forward service/systemd events
    if "systemd" in groups:
        return True

    # Forward high-level alerts
    if level >= 7:
        return True

    # Forward if alert contains syscheck data
    if alert.get("syscheck"):
        return True

    return False


def main():
    """Main entry point."""
    try:
        alert = read_alert()

        log(f"Processing alert: rule_id={alert.get('rule', {}).get('id')}, "
            f"level={alert.get('rule', {}).get('level')}, "
            f"agent={alert.get('agent', {}).get('id')}")

        if should_forward_alert(alert):
            success = send_to_kootenai(alert)
            if not success:
                log("Failed to send alert to Kootenai")
                sys.exit(1)
        else:
            log("Alert filtered, not forwarding")

        sys.exit(0)

    except Exception as e:
        log(f"Integration error: {e}")
        sys.exit(1)


if __name__ == "__main__":
    main()
