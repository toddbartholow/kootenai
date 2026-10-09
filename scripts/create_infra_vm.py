#!/usr/bin/env python3
"""
Create Infrastructure VM on Proxmox

This script creates a Rocky Linux or Ubuntu VM on Proxmox to host the
Kootenai control plane (PostgreSQL, NATS, API, Web UI).

Usage:
    python3 create_infra_vm.py [--os rocky|ubuntu]

Requirements:
    pip install proxmoxer requests python-dotenv
"""

import sys
import time
import argparse

from proxmoxer import ProxmoxAPI

from config_utils import load_config


def wait_for_task(proxmox, node: str, upid: str, timeout: int = 600) -> bool:
    """Wait for a Proxmox task to complete."""
    start_time = time.time()
    while time.time() - start_time < timeout:
        task_status = proxmox.nodes(node).tasks(upid).status.get()
        if task_status.get("status") == "stopped":
            exit_status = task_status.get("exitstatus", "")
            if exit_status == "OK":
                return True
            else:
                print(f"Task failed with status: {exit_status}")
                return False
        time.sleep(2)
        print(".", end="", flush=True)
    print("\nTask timed out!")
    return False


def list_isos(proxmox, node: str, storage: str = "local") -> list:
    """List available ISO images."""
    try:
        content = proxmox.nodes(node).storage(storage).content.get()
        isos = [item for item in content if item.get("content") == "iso"]
        return isos
    except Exception as e:
        print(f"Error listing ISOs: {e}")
        return []


def find_iso(isos: list, pattern: str) -> str:
    """Find an ISO matching a pattern."""
    pattern_lower = pattern.lower()
    for iso in isos:
        volid = iso.get("volid", "")
        if pattern_lower in volid.lower():
            return volid
    return ""


def get_next_vmid(proxmox) -> int:
    """Get the next available VMID."""
    return proxmox.cluster.nextid.get()


def create_vm(proxmox, node: str, vmid: int, name: str, iso: str, config: dict) -> bool:
    """Create a new VM."""
    print(f"\nCreating VM {vmid}: {name}")

    try:
        upid = proxmox.nodes(node).qemu.create(
            vmid=vmid,
            name=name,
            memory=config.get("memory", 4096),
            cores=config.get("cores", 2),
            sockets=1,
            ostype="l26",  # Linux 2.6+ kernel
            scsihw="virtio-scsi-single",
            scsi0=f"{config.get('storage', 'local-lvm')}:{config.get('disk_size', 32)},iothread=1",
            ide2=f"{iso},media=cdrom",
            net0=f"virtio,bridge={config.get('bridge', 'vmbr0')}",
            boot="order=ide2;scsi0",
            agent="enabled=1",
            serial0="socket",
            vga="serial0",
            cpu="host",
        )
        print("Waiting for VM creation", end="")
        if wait_for_task(proxmox, node, upid):
            print(f"\nVM {vmid} created successfully")
            return True
        else:
            print(f"\nFailed to create VM {vmid}")
            return False
    except Exception as e:
        print(f"Error creating VM: {e}")
        return False


def start_vm(proxmox, node: str, vmid: int) -> bool:
    """Start a VM."""
    print(f"Starting VM {vmid}...")
    try:
        upid = proxmox.nodes(node).qemu(vmid).status.start.post()
        if wait_for_task(proxmox, node, upid, timeout=60):
            print(f"VM {vmid} started")
            return True
        return False
    except Exception as e:
        print(f"Error starting VM: {e}")
        return False


def main():
    parser = argparse.ArgumentParser(description="Create Infrastructure VM on Proxmox")
    parser.add_argument("--os", choices=["rocky", "ubuntu"], default="rocky",
                        help="Operating system to use (default: rocky)")
    parser.add_argument("--name", default="kootenai-infra",
                        help="VM name (default: kootenai-infra)")
    parser.add_argument("--memory", type=int, default=4096,
                        help="Memory in MB (default: 4096)")
    parser.add_argument("--cores", type=int, default=2,
                        help="CPU cores (default: 2)")
    parser.add_argument("--disk", type=int, default=32,
                        help="Disk size in GB (default: 32)")
    parser.add_argument("--storage", default="local-lvm",
                        help="Storage for VM disk (default: local-lvm)")
    parser.add_argument("--bridge", default="vmbr0",
                        help="Network bridge (default: vmbr0)")
    parser.add_argument("--start", action="store_true",
                        help="Start VM after creation")
    parser.add_argument("--list-isos", action="store_true",
                        help="List available ISOs and exit")
    args = parser.parse_args()

    config = load_config()

    if not config["host"]:
        print("Error: PROXMOX_HOST not set")
        sys.exit(1)
    if not config["password"]:
        print("Error: PROXMOX_PASSWORD not set")
        sys.exit(1)

    print("=" * 60)
    print("Kootenai Infrastructure VM Creator")
    print("=" * 60)
    print(f"Proxmox Host: {config['host']}")
    print(f"Node: {config['node']}")
    print("=" * 60)

    # Connect to Proxmox
    print("\nConnecting to Proxmox...")
    try:
        proxmox = ProxmoxAPI(
            config["host"],
            user=config["user"],
            password=config["password"],
            verify_ssl=False,
            timeout=120
        )
        version = proxmox.version.get()
        print(f"Connected to Proxmox VE {version['version']}")
    except Exception as e:
        print(f"Failed to connect: {e}")
        sys.exit(1)

    # List ISOs
    print("\nScanning for ISO images...")
    isos = list_isos(proxmox, config["node"])

    if args.list_isos:
        print("\nAvailable ISOs:")
        for iso in isos:
            size_gb = iso.get("size", 0) / (1024**3)
            print(f"  {iso.get('volid')} ({size_gb:.1f} GB)")
        sys.exit(0)

    if not isos:
        print("No ISO images found. Please upload an ISO to Proxmox storage.")
        sys.exit(1)

    print("\nAvailable ISOs:")
    for iso in isos:
        print(f"  - {iso.get('volid')}")

    # Find matching ISO
    if args.os == "rocky":
        iso_patterns = ["rocky", "rockylinux", "rocky-9", "rocky9"]
    else:
        iso_patterns = ["ubuntu", "ubuntu-22", "ubuntu-24", "ubuntu22", "ubuntu24"]

    selected_iso = ""
    for pattern in iso_patterns:
        selected_iso = find_iso(isos, pattern)
        if selected_iso:
            break

    if not selected_iso:
        print(f"\nNo {args.os} ISO found. Available ISOs:")
        for iso in isos:
            print(f"  {iso.get('volid')}")
        print(f"\nPlease upload a {args.os.title()} Linux ISO to Proxmox.")
        sys.exit(1)

    print(f"\nSelected ISO: {selected_iso}")

    # Get next VMID
    vmid = get_next_vmid(proxmox)
    print(f"Using VMID: {vmid}")

    # Create VM
    vm_config = {
        "memory": args.memory,
        "cores": args.cores,
        "disk_size": args.disk,
        "storage": args.storage,
        "bridge": args.bridge,
    }

    if not create_vm(proxmox, config["node"], vmid, args.name, selected_iso, vm_config):
        sys.exit(1)

    # Start VM if requested
    if args.start:
        start_vm(proxmox, config["node"], vmid)

    print("\n" + "=" * 60)
    print("Infrastructure VM Created!")
    print("=" * 60)
    print(f"  VMID: {vmid}")
    print(f"  Name: {args.name}")
    print(f"  Memory: {args.memory} MB")
    print(f"  Cores: {args.cores}")
    print(f"  Disk: {args.disk} GB")
    print(f"  ISO: {selected_iso}")
    print("")
    print("Next steps:")
    print(f"  1. Open Proxmox console for VM {vmid}")
    print(f"  2. Install {args.os.title()} Linux")
    print("  3. After install, run the bootstrap script:")
    print("     curl -fsSL https://raw.githubusercontent.com/toddbartholow/kootenai/main/deploy/bootstrap.sh | bash")
    print("=" * 60)


if __name__ == "__main__":
    main()
