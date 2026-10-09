#!/usr/bin/env python3
"""
Create Lab VM Templates on Proxmox

This script creates VMs from ISOs that can be installed and converted to templates
for use as lab pod base images.

For cloud-init templates (recommended), use cloud images instead of ISOs.

Usage:
    python3 create_lab_templates.py --list-isos
    python3 create_lab_templates.py --create rocky --vmid 9001
    python3 create_lab_templates.py --create ubuntu --vmid 9002
    python3 create_lab_templates.py --convert-to-template --vmid 9001

Requirements:
    pip install proxmoxer requests python-dotenv
"""

import os
import sys
import time
import argparse

from proxmoxer import ProxmoxAPI


def load_config():
    """Load configuration from environment variables and .env files."""
    try:
        from dotenv import load_dotenv
        for env_path in [".env.local", ".env", "../api/.env.local", "../api/.env", "api/.env.local", "api/.env"]:
            if os.path.exists(env_path):
                load_dotenv(env_path, override=True)
    except ImportError:
        pass

    def parse_host(host_url: str) -> str:
        if not host_url:
            return ""
        host = host_url.replace("https://", "").replace("http://", "")
        if ":" in host:
            host = host.split(":")[0]
        return host

    return {
        "host": parse_host(os.getenv("PROXMOX_HOST", "")),
        "user": os.getenv("PROXMOX_USER", "root@pam"),
        "password": os.getenv("PROXMOX_PASSWORD", ""),
        "node": os.getenv("PROXMOX_NODE", "pve"),
    }


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


def delete_vm_if_exists(proxmox, node: str, vmid: int) -> bool:
    """Delete a VM if it exists."""
    try:
        proxmox.nodes(node).qemu(vmid).status.current.get()
        print(f"VM {vmid} exists, deleting...")
        upid = proxmox.nodes(node).qemu(vmid).delete(purge=1)
        if wait_for_task(proxmox, node, upid):
            print(f"\nVM {vmid} deleted successfully")
            return True
        return False
    except Exception:
        return True  # VM doesn't exist


def create_template_vm(proxmox, node: str, vmid: int, name: str, iso: str, config: dict) -> bool:
    """Create a VM optimized for use as a template."""
    print(f"\nCreating template VM {vmid}: {name}")

    try:
        # Create VM with cloud-init optimized settings
        upid = proxmox.nodes(node).qemu.create(
            vmid=vmid,
            name=name,
            memory=config.get("memory", 2048),
            cores=config.get("cores", 2),
            sockets=1,
            ostype="l26",
            scsihw="virtio-scsi-single",
            scsi0=f"{config.get('storage', 'local-lvm')}:{config.get('disk_size', 20)},iothread=1,discard=on",
            ide2=f"{iso},media=cdrom",
            net0=f"virtio,bridge={config.get('bridge', 'vmbr0')}",
            boot="order=ide2;scsi0",
            agent="enabled=1",
            serial0="socket",
            vga="serial0",
            cpu="host",
            balloon=0,  # Disable ballooning for templates
            hotplug="disk,network,usb",
        )
        print("Waiting for VM creation", end="")
        if wait_for_task(proxmox, node, upid):
            print(f"\nVM {vmid} created successfully")
        else:
            print(f"\nFailed to create VM {vmid}")
            return False

        # Add cloud-init drive
        print("Adding cloud-init drive...")
        proxmox.nodes(node).qemu(vmid).config.put(
            ide0=f"{config.get('storage', 'local-lvm')}:cloudinit"
        )

        # Set cloud-init defaults
        print("Setting cloud-init defaults...")
        proxmox.nodes(node).qemu(vmid).config.put(
            ciuser="labadmin",
            cipassword=os.environ["CLOUDINIT_PASSWORD"],  # Required: no default password
            ipconfig0="ip=dhcp",
            sshkeys=""  # Can be populated with SSH public keys
        )

        # Add description
        proxmox.nodes(node).qemu(vmid).config.put(
            description=f"Lab template: {name}\nOS: {iso}\nCloud-init enabled\n\nAfter OS installation:\n1. Install qemu-guest-agent and cloud-init\n2. Clean up: cloud-init clean\n3. Convert to template via Proxmox UI or API"
        )

        return True

    except Exception as e:
        print(f"Error creating VM: {e}")
        return False


def convert_to_template(proxmox, node: str, vmid: int) -> bool:
    """Convert a VM to a template."""
    print(f"\nConverting VM {vmid} to template...")
    try:
        # Make sure VM is stopped
        status = proxmox.nodes(node).qemu(vmid).status.current.get()
        if status.get("status") != "stopped":
            print("Stopping VM first...")
            upid = proxmox.nodes(node).qemu(vmid).status.stop.post()
            wait_for_task(proxmox, node, upid, timeout=60)

        # Convert to template
        proxmox.nodes(node).qemu(vmid).template.post()
        print(f"VM {vmid} is now a template")
        return True
    except Exception as e:
        print(f"Error converting to template: {e}")
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
    parser = argparse.ArgumentParser(description="Create Lab VM Templates on Proxmox")
    parser.add_argument("--list-isos", action="store_true", help="List available ISOs")
    parser.add_argument("--create", choices=["rocky", "ubuntu", "kali"], help="Create template VM from ISO")
    parser.add_argument("--vmid", type=int, help="VMID for the template (default: 9001 for rocky, 9002 for ubuntu, 9003 for kali)")
    parser.add_argument("--name", help="VM name (default: based on OS)")
    parser.add_argument("--memory", type=int, default=2048, help="Memory in MB (default: 2048)")
    parser.add_argument("--cores", type=int, default=2, help="CPU cores (default: 2)")
    parser.add_argument("--disk", type=int, default=20, help="Disk size in GB (default: 20)")
    parser.add_argument("--storage", default="local-lvm", help="Storage for VM disk (default: local-lvm)")
    parser.add_argument("--force", action="store_true", help="Delete existing VM with same VMID")
    parser.add_argument("--start", action="store_true", help="Start VM after creation (for OS installation)")
    parser.add_argument("--convert-to-template", action="store_true", help="Convert VM to template")
    args = parser.parse_args()

    config = load_config()

    if not config["host"]:
        print("Error: PROXMOX_HOST not set")
        sys.exit(1)
    if not config["password"]:
        print("Error: PROXMOX_PASSWORD not set")
        sys.exit(1)

    print("=" * 60)
    print("Kootenai Template Creator")
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
    if args.list_isos:
        isos = list_isos(proxmox, config["node"])
        print("\nAvailable ISOs:")
        for iso in isos:
            size_gb = iso.get("size", 0) / (1024**3)
            print(f"  {iso.get('volid')} ({size_gb:.1f} GB)")
        sys.exit(0)

    # Convert existing VM to template
    if args.convert_to_template:
        if not args.vmid:
            print("Error: --vmid required for --convert-to-template")
            sys.exit(1)
        if convert_to_template(proxmox, config["node"], args.vmid):
            print("\nTemplate conversion complete!")
        else:
            sys.exit(1)
        sys.exit(0)

    # Create template VM
    if args.create:
        isos = list_isos(proxmox, config["node"])

        # Find matching ISO
        iso_patterns = {
            "rocky": ["rocky", "rockylinux", "rocky-9", "rocky-10"],
            "ubuntu": ["ubuntu", "ubuntu-22", "ubuntu-24"],
            "kali": ["kali", "kali-linux"],
        }

        selected_iso = ""
        for pattern in iso_patterns.get(args.create, [args.create]):
            selected_iso = find_iso(isos, pattern)
            if selected_iso:
                break

        if not selected_iso:
            print(f"\nNo {args.create} ISO found. Available ISOs:")
            for iso in isos:
                print(f"  {iso.get('volid')}")
            sys.exit(1)

        # Default VMIDs
        default_vmids = {"rocky": 9001, "ubuntu": 9002, "kali": 9003}
        vmid = args.vmid or default_vmids.get(args.create, 9000)

        # Default names
        default_names = {
            "rocky": "rocky-lab-template",
            "ubuntu": "ubuntu-lab-template",
            "kali": "kali-lab-template",
        }
        name = args.name or default_names.get(args.create, f"{args.create}-template")

        print(f"\nSelected ISO: {selected_iso}")
        print(f"VMID: {vmid}")
        print(f"Name: {name}")

        # Delete existing VM if force
        if args.force:
            delete_vm_if_exists(proxmox, config["node"], vmid)

        # Create VM
        vm_config = {
            "memory": args.memory,
            "cores": args.cores,
            "disk_size": args.disk,
            "storage": args.storage,
        }

        if not create_template_vm(proxmox, config["node"], vmid, name, selected_iso, vm_config):
            sys.exit(1)

        # Start VM if requested
        if args.start:
            start_vm(proxmox, config["node"], vmid)

        print("\n" + "=" * 60)
        print("Template VM Created!")
        print("=" * 60)
        print(f"  VMID: {vmid}")
        print(f"  Name: {name}")
        print(f"  ISO: {selected_iso}")
        print("")
        print("Next steps:")
        print(f"  1. Open Proxmox console for VM {vmid}")
        print(f"  2. Install the OS (minimal server install recommended)")
        print("  3. Install cloud-init and qemu-guest-agent:")
        if args.create == "rocky":
            print("     dnf install -y cloud-init qemu-guest-agent")
            print("     systemctl enable qemu-guest-agent cloud-init")
        elif args.create == "ubuntu":
            print("     apt install -y cloud-init qemu-guest-agent")
            print("     systemctl enable qemu-guest-agent cloud-init")
        elif args.create == "kali":
            print("     apt install -y cloud-init qemu-guest-agent")
            print("     systemctl enable qemu-guest-agent cloud-init")
        print("  4. Clean up for cloning:")
        print("     cloud-init clean")
        print("     truncate -s 0 /etc/machine-id")
        print("     rm -f /etc/ssh/ssh_host_*")
        print("     shutdown -h now")
        print("  5. Convert to template:")
        print(f"     python3 create_lab_templates.py --convert-to-template --vmid {vmid}")
        print("=" * 60)
        sys.exit(0)

    # No action specified
    parser.print_help()


if __name__ == "__main__":
    main()
