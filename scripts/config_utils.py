"""Shared Proxmox configuration loader for automation scripts.

Loads connection settings from environment variables and .env files
with a consistent search path and host-URL parsing logic.
"""

from __future__ import annotations

import os


def load_config() -> dict[str, str]:
    """Load Proxmox configuration from environment variables and .env files.

    Searches for .env files in a standard set of relative paths and
    parses the ``PROXMOX_HOST`` URL down to a bare hostname.

    Returns:
        Dict with keys: host, user, password, node.
    """
    try:
        from dotenv import load_dotenv

        for env_path in [
            ".env.local",
            ".env",
            "../api/.env.local",
            "../api/.env",
            "api/.env.local",
            "api/.env",
        ]:
            if os.path.exists(env_path):
                load_dotenv(env_path, override=True)
    except ImportError:
        pass

    return {
        "host": _parse_host(os.getenv("PROXMOX_HOST", "")),
        "user": os.getenv("PROXMOX_USER", "root@pam"),
        "password": os.getenv("PROXMOX_PASSWORD", ""),
        "node": os.getenv("PROXMOX_NODE", "pve"),
    }


def _parse_host(host_url: str) -> str:
    """Strip scheme and port from a Proxmox host URL.

    Args:
        host_url: Raw value of PROXMOX_HOST (may include https:// and :8006).

    Returns:
        Bare hostname string, or empty string if input is falsy.
    """
    if not host_url:
        return ""
    host = host_url.replace("https://", "").replace("http://", "")
    if ":" in host:
        host = host.split(":")[0]
    return host
