#!/usr/bin/env python3
"""
Fix VM console for OS installation.

Changes VMs from serial console to VGA display so you can see the installer.

Usage:
    python3 fix_vm_console.py                    # Uses VMIDS env var or defaults
    python3 fix_vm_console.py 100 9001 9002      # Explicit VM IDs
    VMIDS=100,9001,9002 python3 fix_vm_console.py
"""

import argparse
import os
import sys

from proxmoxer import ProxmoxAPI

from config_utils import load_config

def fix_vm_console(proxmox, node, vmid):
    """Change VM from serial to VGA console."""
    print(f"\nFixing VM {vmid} console...")

    try:
        # Stop VM if running
        status = proxmox.nodes(node).qemu(vmid).status.current.get()
        if status.get("status") == "running":
            print(f"  Stopping VM {vmid}...")
            proxmox.nodes(node).qemu(vmid).status.stop.post()
            import time
            time.sleep(5)

        # Remove serial console, set standard VGA
        print(f"  Updating display settings...")
        proxmox.nodes(node).qemu(vmid).config.put(
            vga="std",
            delete="serial0"
        )

        print(f"  Starting VM {vmid}...")
        proxmox.nodes(node).qemu(vmid).status.start.post()

        print(f"  VM {vmid} fixed! You can now use the noVNC console.")
        return True
    except Exception as e:
        print(f"  Error: {e}")
        return False

def parse_vmids() -> list[int]:
    """Resolve VM IDs from CLI args, env var, or defaults."""
    parser = argparse.ArgumentParser(
        description="Fix VM console for OS installation (serial -> VGA)",
    )
    parser.add_argument(
        "vmids",
        nargs="*",
        type=int,
        help="VM IDs to fix (default: $VMIDS env var, or 100 9001 9002)",
    )
    args = parser.parse_args()

    if args.vmids:
        return args.vmids

    env_vmids = os.getenv("VMIDS", "")
    if env_vmids:
        return [int(v.strip()) for v in env_vmids.split(",") if v.strip()]

    return [100, 9001, 9002]


def main():
    config = load_config()

    if not config["host"] or not config["password"]:
        print("Error: PROXMOX_HOST and PROXMOX_PASSWORD required")
        sys.exit(1)

    print("Connecting to Proxmox...")
    proxmox = ProxmoxAPI(
        config["host"],
        user=config["user"],
        password=config["password"],
        verify_ssl=False
    )
    print(f"Connected to Proxmox VE {proxmox.version.get()['version']}")

    vmids = parse_vmids()
    print(f"Target VM IDs: {vmids}")

    for vmid in vmids:
        try:
            fix_vm_console(proxmox, config["node"], vmid)
        except Exception as e:
            print(f"VM {vmid} not found or error: {e}")

    print("\n" + "=" * 50)
    print("Done! Refresh Proxmox web UI and use 'Console' button.")
    print("You should now see the OS installer.")
    print("=" * 50)

if __name__ == "__main__":
    main()
