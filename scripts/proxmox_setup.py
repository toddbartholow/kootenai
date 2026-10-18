#!/usr/bin/env python3
"""
Proxmox VM Template Setup Script

This script creates a Debian 12 cloud-init template for the Kootenai platform.
Uses the proxmoxer library for API interactions.

Usage:
    # Set environment variables or use .env file
    export PROXMOX_HOST=<PROXMOX_IP>
    export PROXMOX_USER=root@pam
    export PROXMOX_PASSWORD=yourpassword

    python3 proxmox_setup.py

Requirements:
    pip install proxmoxer requests python-dotenv
"""

import os
import sys
import time

from proxmoxer import ProxmoxAPI


def load_config():
    """Load configuration from environment variables and .env files."""
    # Try to load dotenv files
    try:
        from dotenv import load_dotenv
        # Load .env.local or .env file from multiple locations
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
        pass  # python-dotenv is optional

    def parse_proxmox_host(host_url: str) -> str:
        """Extract hostname/IP from Proxmox URL."""
        if not host_url:
            return ""
        # Remove protocol prefix
        host = host_url.replace("https://", "").replace("http://", "")
        # Remove port if present
        if ":" in host:
            host = host.split(":")[0]
        return host

    return {
        "host": parse_proxmox_host(os.getenv("PROXMOX_HOST", "")),
        "user": os.getenv("PROXMOX_USER", "root@pam"),
        "password": os.getenv("PROXMOX_PASSWORD", ""),
        "node": os.getenv("PROXMOX_NODE", "pve"),
    }


# Load configuration
_config = load_config()
PROXMOX_HOST = _config["host"]
PROXMOX_USER = _config["user"]
PROXMOX_PASSWORD = _config["password"]
PROXMOX_NODE = _config["node"]

# Template configuration
TEMPLATE_VMID = 9000
TEMPLATE_NAME = "debian-12-template"
CLOUD_IMAGE_URL = "https://cloud.debian.org/images/cloud/bookworm/latest/debian-12-generic-amd64.qcow2"
CLOUD_IMAGE_FILENAME = "debian-12-cloud.qcow2"


def wait_for_task(proxmox, node: str, upid: str, timeout: int = 300) -> bool:
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


def delete_vm_if_exists(proxmox, node: str, vmid: int):
    """Delete a VM if it exists."""
    try:
        proxmox.nodes(node).qemu(vmid).status.current.get()
        print(f"VM {vmid} exists, deleting...")
        upid = proxmox.nodes(node).qemu(vmid).delete(purge=1)
        if wait_for_task(proxmox, node, upid):
            print(f"VM {vmid} deleted successfully")
        else:
            print(f"Failed to delete VM {vmid}")
            sys.exit(1)
    except Exception:
        print(f"VM {vmid} does not exist, continuing...")


def download_cloud_image(proxmox, node: str, storage: str, url: str, filename: str) -> bool:
    """Download a cloud image to Proxmox storage."""
    print(f"Downloading cloud image: {filename}")

    # Check if already downloaded
    try:
        content = proxmox.nodes(node).storage(storage).content.get()
        for item in content:
            if filename in item.get("volid", ""):
                print(f"Image {filename} already exists")
                return True
    except Exception:
        pass

    # Download the image
    try:
        upid = proxmox.nodes(node).storage(storage).post(
            "download-url",
            url=url,
            filename=filename,
            content="import"
        )
        print("Download started", end="")
        if wait_for_task(proxmox, node, upid, timeout=600):
            print(f"\nImage {filename} downloaded successfully")
            return True
        else:
            print(f"\nFailed to download {filename}")
            return False
    except Exception as e:
        print(f"Error downloading image: {e}")
        return False


def create_vm_template(proxmox, node: str, vmid: int, name: str):
    """Create a VM and configure it as a cloud-init template."""

    print(f"\n=== Creating VM {vmid}: {name} ===")

    # Step 1: Create the VM
    print("Creating VM...")
    upid = proxmox.nodes(node).qemu.create(
        vmid=vmid,
        name=name,
        memory=1024,
        cores=2,
        ostype="l26",
        scsihw="virtio-scsi-single",
        agent="enabled=1",
        serial0="socket",
        vga="serial0"
    )
    if not wait_for_task(proxmox, node, upid):
        print("Failed to create VM")
        sys.exit(1)
    print("VM created")

    # Step 2: Import the cloud image disk
    print("Importing cloud image disk...")
    upid = proxmox.nodes(node).qemu(vmid).config.put(
        scsi0=f"local-lvm:0,import-from=/var/lib/vz/import/{CLOUD_IMAGE_FILENAME}"
    )
    # This returns None for sync operations, check if successful
    time.sleep(5)  # Wait for import

    # Verify disk was imported
    config = proxmox.nodes(node).qemu(vmid).config.get()
    if "scsi0" in config:
        print(f"Disk imported: {config['scsi0']}")
    else:
        print("Warning: Disk may not have imported correctly")

    # Step 3: Add cloud-init drive
    print("Adding cloud-init drive...")
    proxmox.nodes(node).qemu(vmid).config.put(
        ide2="local-lvm:cloudinit"
    )

    # Step 4: Add network interface
    print("Adding network interface...")
    proxmox.nodes(node).qemu(vmid).config.put(
        net0="virtio,bridge=vmbr0"
    )

    # Step 5: Set boot order
    print("Setting boot order...")
    proxmox.nodes(node).qemu(vmid).config.put(
        boot="order=scsi0"
    )

    # Step 6: Configure cloud-init defaults
    print("Configuring cloud-init...")
    proxmox.nodes(node).qemu(vmid).config.put(
        ciuser="labadmin",
        cipassword=os.environ["CLOUDINIT_PASSWORD"],  # Required: no default password
        ipconfig0="ip=dhcp"
    )

    # Step 7: Add description
    proxmox.nodes(node).qemu(vmid).config.put(
        description="Debian 12 cloud-init template for Kootenai platform"
    )

    print(f"VM {vmid} configured successfully")
    return True


def convert_to_template(proxmox, node: str, vmid: int):
    """Convert a VM to a template."""
    print(f"\nConverting VM {vmid} to template...")
    try:
        proxmox.nodes(node).qemu(vmid).template.post()
        print(f"VM {vmid} is now a template")
        return True
    except Exception as e:
        print(f"Error converting to template: {e}")
        return False


def verify_template(proxmox, node: str, vmid: int):
    """Verify the template was created correctly."""
    print(f"\n=== Template Verification ===")
    try:
        config = proxmox.nodes(node).qemu(vmid).config.get()
        status = proxmox.nodes(node).qemu(vmid).status.current.get()

        print(f"Name: {config.get('name')}")
        print(f"Memory: {config.get('memory')} MB")
        print(f"Cores: {config.get('cores')}")
        print(f"Disk: {config.get('scsi0', 'Not configured')}")
        print(f"Network: {config.get('net0', 'Not configured')}")
        print(f"Cloud-Init: {config.get('ide2', 'Not configured')}")
        print(f"Template: {config.get('template', 0) == 1}")
        print(f"Status: {status.get('status')}")

        return True
    except Exception as e:
        print(f"Error verifying template: {e}")
        return False


def main():
    # Validate configuration
    if not PROXMOX_HOST:
        print("Error: PROXMOX_HOST environment variable not set")
        print("Set it directly or create a .env.local file with:")
        print("  PROXMOX_HOST=your-proxmox-ip")
        print("  PROXMOX_PASSWORD=your-password")
        sys.exit(1)

    if not PROXMOX_PASSWORD:
        print("Error: PROXMOX_PASSWORD environment variable not set")
        sys.exit(1)

    print("=" * 60)
    print("Proxmox VM Template Setup")
    print("=" * 60)
    print(f"Host: {PROXMOX_HOST}")
    print(f"User: {PROXMOX_USER}")
    print(f"Node: {PROXMOX_NODE}")
    print(f"Template VMID: {TEMPLATE_VMID}")
    print(f"Template Name: {TEMPLATE_NAME}")
    print("=" * 60)

    # Connect to Proxmox
    print("\nConnecting to Proxmox...")
    try:
        proxmox = ProxmoxAPI(
            PROXMOX_HOST,
            user=PROXMOX_USER,
            password=PROXMOX_PASSWORD,
            verify_ssl=False,
            timeout=120  # Longer timeout for disk operations
        )
        # Test connection
        version = proxmox.version.get()
        print(f"Connected to Proxmox VE {version['version']}")
    except Exception as e:
        print(f"Failed to connect: {e}")
        sys.exit(1)

    # Delete existing VM if present
    delete_vm_if_exists(proxmox, PROXMOX_NODE, TEMPLATE_VMID)

    # Download cloud image if not present
    if not download_cloud_image(
        proxmox, PROXMOX_NODE, "local",
        CLOUD_IMAGE_URL, CLOUD_IMAGE_FILENAME
    ):
        sys.exit(1)

    # Create and configure the VM
    if not create_vm_template(proxmox, PROXMOX_NODE, TEMPLATE_VMID, TEMPLATE_NAME):
        sys.exit(1)

    # Convert to template
    if not convert_to_template(proxmox, PROXMOX_NODE, TEMPLATE_VMID):
        sys.exit(1)

    # Verify
    verify_template(proxmox, PROXMOX_NODE, TEMPLATE_VMID)

    print("\n" + "=" * 60)
    print("Template creation complete!")
    print(f"You can now clone VM {TEMPLATE_VMID} to create lab instances")
    print("=" * 60)


if __name__ == "__main__":
    main()
