#!/usr/bin/env python3
"""
Setup Cloud Image Templates for Kootenai

This script downloads pre-built cloud images and creates Proxmox templates
with proper configuration for the Kootenai platform.

The script handles the Rocky Linux cloud-init password issue by:
1. Booting the VM with SSH key authentication (which works)
2. SSHing in to configure labadmin user and fix SSH settings
3. Cleaning up and converting to template

This fully automates template creation without manual intervention.

Usage:
    python3 setup_cloud_templates.py --list              # List available images
    python3 setup_cloud_templates.py --setup ubuntu-24.04  # Setup Ubuntu template
    python3 setup_cloud_templates.py --setup rocky-9       # Setup Rocky template (fully automated)
    python3 setup_cloud_templates.py --setup all           # Setup all templates
    python3 setup_cloud_templates.py --setup ubuntu-24.04 --with-wazuh  # Include Wazuh agent

Requirements:
    pip install proxmoxer requests python-dotenv

Environment variables (or .env.local file):
    PROXMOX_HOST=<PROXMOX_IP>
    PROXMOX_USER=root@pam
    PROXMOX_PASSWORD=yourpassword
    PROXMOX_NODE=pve
"""

import os
import sys
import time
import argparse
import subprocess
import tempfile
from pathlib import Path

from proxmoxer import ProxmoxAPI


# =============================================================================
# Cloud Image Definitions
# =============================================================================

CLOUD_IMAGES = {
    "ubuntu-24.04": {
        "name": "ubuntu-24.04-cloud",
        "vmid": 9010,
        "url": "https://cloud-images.ubuntu.com/noble/current/noble-server-cloudimg-amd64.img",
        "filename": "ubuntu-24.04-cloudimg-amd64.img",
        "description": "Ubuntu 24.04 LTS (Noble Numbat) Cloud Image",
        "memory": 2048,
        "cores": 2,
        "disk_resize": "20G",
        "default_user": "ubuntu",  # Default cloud-init user
        "os_family": "debian",
    },
    "ubuntu-22.04": {
        "name": "ubuntu-22.04-cloud",
        "vmid": 9011,
        "url": "https://cloud-images.ubuntu.com/jammy/current/jammy-server-cloudimg-amd64.img",
        "filename": "ubuntu-22.04-cloudimg-amd64.img",
        "description": "Ubuntu 22.04 LTS (Jammy Jellyfish) Cloud Image",
        "memory": 2048,
        "cores": 2,
        "disk_resize": "20G",
        "default_user": "ubuntu",
        "os_family": "debian",
    },
    "debian-12": {
        "name": "debian-12-cloud",
        "vmid": 9020,
        "url": "https://cloud.debian.org/images/cloud/bookworm/latest/debian-12-generic-amd64.qcow2",
        "filename": "debian-12-generic-amd64.qcow2",
        "description": "Debian 12 (Bookworm) Cloud Image",
        "memory": 1024,
        "cores": 2,
        "disk_resize": "20G",
        "default_user": "debian",
        "os_family": "debian",
    },
    "rocky-9": {
        "name": "rocky-9-cloud",
        "vmid": 9030,
        "url": "https://download.rockylinux.org/pub/rocky/9/images/x86_64/Rocky-9-GenericCloud-Base.latest.x86_64.qcow2",
        "filename": "rocky-9-genericcloud.qcow2",
        "description": "Rocky Linux 9 Cloud Image (RHEL compatible)",
        "memory": 2048,
        "cores": 2,
        "disk_resize": "20G",
        "default_user": "cloud-user",  # Rocky uses cloud-user by default
        "os_family": "rhel",
    },
    "rocky-10": {
        "name": "rocky-10-cloud",
        "vmid": 9032,
        "url": "https://dl.rockylinux.org/pub/rocky/10/images/x86_64/Rocky-10-GenericCloud-Base.latest.x86_64.qcow2",
        "filename": "rocky-10-genericcloud.qcow2",
        "description": "Rocky Linux 10 Cloud Image (RHEL compatible)",
        "memory": 2048,
        "cores": 2,
        "disk_resize": "20G",
        "default_user": "cloud-user",
        "os_family": "rhel",
    },
    "almalinux-9": {
        "name": "almalinux-9-cloud",
        "vmid": 9031,
        "url": "https://repo.almalinux.org/almalinux/9/cloud/x86_64/images/AlmaLinux-9-GenericCloud-latest.x86_64.qcow2",
        "filename": "almalinux-9-genericcloud.qcow2",
        "description": "AlmaLinux 9 Cloud Image (RHEL compatible)",
        "memory": 2048,
        "cores": 2,
        "disk_resize": "20G",
        "default_user": "almalinux",
        "os_family": "rhel",
    },
    "fedora-40": {
        "name": "fedora-40-cloud",
        "vmid": 9040,
        "url": "https://download.fedoraproject.org/pub/fedora/linux/releases/40/Cloud/x86_64/images/Fedora-Cloud-Base-Generic.x86_64-40-1.14.qcow2",
        "filename": "fedora-40-cloud.qcow2",
        "description": "Fedora 40 Cloud Image",
        "memory": 2048,
        "cores": 2,
        "disk_resize": "20G",
        "default_user": "fedora",
        "os_family": "rhel",
    },
}

# Default lab credentials
DEFAULT_LAB_USER = "labadmin"
DEFAULT_LAB_PASSWORD = os.environ.get("LAB_PASSWORD")
if not DEFAULT_LAB_PASSWORD:
    raise SystemExit("ERROR: LAB_PASSWORD environment variable is required")

# Wazuh manager address
WAZUH_MANAGER = os.environ.get("WAZUH_MANAGER", "192.0.2.10")


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


def get_ssh_key_path() -> str:
    """Get path to SSH private key, generating if needed."""
    ssh_dir = Path.home() / ".ssh"
    key_path = ssh_dir / "id_ed25519"
    pub_path = ssh_dir / "id_ed25519.pub"

    if not key_path.exists():
        print("  Generating SSH key pair...")
        ssh_dir.mkdir(mode=0o700, exist_ok=True)
        subprocess.run([
            "ssh-keygen", "-t", "ed25519", "-f", str(key_path), "-N", "", "-C", "kootenai-template-setup"
        ], check=True, capture_output=True)

    return str(key_path)


def get_ssh_public_key() -> str:
    """Get the SSH public key content."""
    key_path = get_ssh_key_path()
    pub_path = key_path + ".pub"
    with open(pub_path, "r") as f:
        return f.read().strip()


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
                print(f"\n  Task failed with status: {exit_status}")
                return False
        time.sleep(2)
        print(".", end="", flush=True)
    print("\n  Task timed out!")
    return False


def run_proxmox_ssh(host: str, user: str, password: str, command: str, timeout: int = 300) -> tuple[int, str, str]:
    """Run a command on Proxmox host via SSH using sshpass."""
    cmd = [
        "sshpass", "-p", password,
        "ssh", "-o", "StrictHostKeyChecking=accept-new", "-o", "ConnectTimeout=10",
        f"{user}@{host}",
        command
    ]
    try:
        result = subprocess.run(cmd, capture_output=True, text=True, timeout=timeout)
        return result.returncode, result.stdout, result.stderr
    except subprocess.TimeoutExpired:
        return -1, "", "Command timed out"
    except Exception as e:
        return -1, "", str(e)


def run_vm_ssh(vm_ip: str, user: str, key_path: str, command: str, timeout: int = 120) -> tuple[int, str, str]:
    """Run a command on a VM via SSH using key authentication."""
    cmd = [
        "ssh",
        "-o", "StrictHostKeyChecking=accept-new",
        "-o", "UserKnownHostsFile=/dev/null",
        "-o", "ConnectTimeout=10",
        "-o", "BatchMode=yes",
        "-i", key_path,
        f"{user}@{vm_ip}",
        command
    ]
    try:
        result = subprocess.run(cmd, capture_output=True, text=True, timeout=timeout)
        return result.returncode, result.stdout, result.stderr
    except subprocess.TimeoutExpired:
        return -1, "", "Command timed out"
    except Exception as e:
        return -1, "", str(e)


def wait_for_vm_ssh(vm_ip: str, user: str, key_path: str, timeout: int = 180) -> bool:
    """Wait for VM to become accessible via SSH."""
    start = time.time()
    while time.time() - start < timeout:
        rc, _, _ = run_vm_ssh(vm_ip, user, key_path, "echo ok", timeout=10)
        if rc == 0:
            return True
        time.sleep(5)
        print(".", end="", flush=True)
    return False


def get_vm_ip(proxmox, node: str, vmid: int, timeout: int = 120) -> str | None:
    """Get VM IP address via QEMU guest agent."""
    start = time.time()
    while time.time() - start < timeout:
        try:
            result = proxmox.nodes(node).qemu(vmid).agent("network-get-interfaces").get()
            for iface in result.get("result", []):
                if iface.get("name") == "lo":
                    continue
                for ip_info in iface.get("ip-addresses", []):
                    if ip_info.get("ip-address-type") == "ipv4":
                        ip = ip_info.get("ip-address")
                        if ip and not ip.startswith("127."):
                            return ip
        except Exception:
            pass
        time.sleep(5)
        print(".", end="", flush=True)
    return None


def download_cloud_image_ssh(host: str, user: str, password: str, url: str, filename: str, storage_path: str = "/var/lib/vz/template/iso") -> bool:
    """Download cloud image to Proxmox host via SSH."""
    print(f"  Checking if {filename} exists...")

    # Check if already exists
    rc, out, _ = run_proxmox_ssh(host, user, password, f"ls -la {storage_path}/{filename} 2>/dev/null")
    if rc == 0 and filename in out:
        print(f"  Image {filename} already exists")
        return True

    print(f"  Downloading {filename} (this may take several minutes)...")
    rc, out, err = run_proxmox_ssh(
        host, user, password,
        f"wget -q -O {storage_path}/{filename} '{url}'",
        timeout=1800  # 30 min timeout for large downloads
    )

    if rc != 0:
        print(f"  Download failed: {err}")
        return False

    print(f"  Downloaded {filename}")
    return True


def configure_vm_for_template(vm_ip: str, cloud_user: str, key_path: str, os_family: str) -> bool:
    """Configure VM after boot: create labadmin, fix SSH, disable cloud-init."""
    print(f"\n  Configuring VM via SSH as {cloud_user}@{vm_ip}...")

    # Configuration script for RHEL-family (Rocky, Alma, Fedora)
    rhel_config_script = f'''
# Create labadmin user with password
sudo useradd -m -G wheel -s /bin/bash {DEFAULT_LAB_USER} 2>/dev/null || true
echo '{DEFAULT_LAB_USER}:{DEFAULT_LAB_PASSWORD}' | sudo chpasswd

# Enable SSH password authentication
# Remove cloud-init's SSH config that disables password auth
sudo rm -f /etc/ssh/sshd_config.d/50-cloud-init.conf

# Create config to enable password auth
echo 'PasswordAuthentication yes' | sudo tee /etc/ssh/sshd_config.d/99-pwauth.conf > /dev/null

# Also fix main sshd_config if needed
sudo sed -i 's/^#*PasswordAuthentication.*/PasswordAuthentication yes/' /etc/ssh/sshd_config

# Restart sshd
sudo systemctl restart sshd

# Disable cloud-init
sudo touch /etc/cloud/cloud-init.disabled
sudo systemctl disable cloud-init cloud-init-local cloud-config cloud-final 2>/dev/null || true

# Ensure services are enabled
sudo systemctl enable qemu-guest-agent sshd

# Create SSH host key regeneration service for clones
cat << 'SSHKEYSVC' | sudo tee /etc/systemd/system/regenerate-ssh-host-keys.service > /dev/null
[Unit]
Description=Regenerate SSH Host Keys on First Boot
Before=sshd.service
ConditionPathExists=!/etc/ssh/ssh_host_ed25519_key

[Service]
Type=oneshot
ExecStart=/usr/bin/ssh-keygen -A
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
SSHKEYSVC

sudo systemctl enable regenerate-ssh-host-keys.service

echo "Configuration complete"
'''

    # Configuration script for Debian-family (Ubuntu, Debian)
    debian_config_script = f'''
# Create labadmin user with password
sudo useradd -m -G sudo -s /bin/bash {DEFAULT_LAB_USER} 2>/dev/null || true
echo '{DEFAULT_LAB_USER}:{DEFAULT_LAB_PASSWORD}' | sudo chpasswd

# Enable SSH password authentication
sudo sed -i 's/^#*PasswordAuthentication.*/PasswordAuthentication yes/' /etc/ssh/sshd_config

# Disable SSH socket activation (Ubuntu 24.04 issue)
sudo systemctl disable ssh.socket 2>/dev/null || true
sudo systemctl stop ssh.socket 2>/dev/null || true
sudo systemctl enable ssh.service
sudo systemctl restart ssh.service

# Disable cloud-init
sudo touch /etc/cloud/cloud-init.disabled
sudo systemctl disable cloud-init cloud-init-local cloud-config cloud-final 2>/dev/null || true

# Ensure services are enabled
sudo systemctl enable qemu-guest-agent ssh

# Create SSH host key regeneration service for clones
cat << 'SSHKEYSVC' | sudo tee /etc/systemd/system/regenerate-ssh-host-keys.service > /dev/null
[Unit]
Description=Regenerate SSH Host Keys on First Boot
Before=ssh.service
ConditionPathExists=!/etc/ssh/ssh_host_ed25519_key

[Service]
Type=oneshot
ExecStart=/usr/bin/ssh-keygen -A
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
SSHKEYSVC

sudo systemctl enable regenerate-ssh-host-keys.service

echo "Configuration complete"
'''

    script = rhel_config_script if os_family == "rhel" else debian_config_script

    rc, out, err = run_vm_ssh(vm_ip, cloud_user, key_path, script, timeout=120)
    if rc != 0:
        print(f"  Configuration failed: {err}")
        return False

    if "Configuration complete" in out:
        print("  VM configured successfully")
        return True

    print(f"  Unexpected output: {out}")
    return False


def verify_password_auth(vm_ip: str, timeout: int = 30) -> bool:
    """Verify password authentication works for labadmin user."""
    print("  Verifying password authentication...")

    cmd = [
        "sshpass", "-p", DEFAULT_LAB_PASSWORD,
        "ssh",
        "-o", "StrictHostKeyChecking=accept-new",
        "-o", "UserKnownHostsFile=/dev/null",
        "-o", "ConnectTimeout=10",
        "-o", "PreferredAuthentications=password",
        "-o", "PubkeyAuthentication=no",
        f"{DEFAULT_LAB_USER}@{vm_ip}",
        "echo 'Password auth works!'"
    ]

    try:
        result = subprocess.run(cmd, capture_output=True, text=True, timeout=timeout)
        if result.returncode == 0 and "Password auth works!" in result.stdout:
            print("  Password authentication verified!")
            return True
        print(f"  Password auth failed: {result.stderr}")
        return False
    except Exception as e:
        print(f"  Password auth check error: {e}")
        return False


def prepare_for_template(vm_ip: str, cloud_user: str, key_path: str) -> bool:
    """Clean up VM before converting to template."""
    print("  Preparing VM for templating...")

    cleanup_script = '''
# Truncate machine-id (will regenerate on boot)
sudo truncate -s 0 /etc/machine-id

# Clean cloud-init state
sudo cloud-init clean 2>/dev/null || true

# Remove SSH host keys (will regenerate via our service on first boot)
sudo rm -f /etc/ssh/ssh_host_*

# Clear authorized_keys (don't want setup SSH key in template)
rm -f ~/.ssh/authorized_keys
sudo rm -f /root/.ssh/authorized_keys 2>/dev/null || true

# Clear bash history
cat /dev/null > ~/.bash_history
history -c

echo "Cleanup complete"
'''

    rc, out, err = run_vm_ssh(vm_ip, cloud_user, key_path, cleanup_script, timeout=60)
    if rc != 0:
        print(f"  Cleanup warning: {err}")
        # Don't fail, cleanup is best-effort

    if "Cleanup complete" in out:
        print("  VM prepared for templating")

    return True


def create_template_from_cloud_image(
    proxmox,
    node: str,
    config: dict,
    proxmox_host: str,
    proxmox_user: str,
    proxmox_password: str,
    with_wazuh: bool = False
) -> bool:
    """Create a VM from a cloud image, configure it, and convert to template."""
    vmid = config["vmid"]
    name = config["name"]
    filename = config["filename"]
    default_user = config.get("default_user", "cloud-user")
    os_family = config.get("os_family", "rhel")

    # Use a temporary VMID for setup, then clone to final template
    setup_vmid = vmid + 10000  # e.g., 9030 -> 19030

    print(f"\n{'='*60}")
    print(f"Creating template: {name} (VMID: {vmid})")
    print(f"{'='*60}")

    # Get SSH key for authentication
    key_path = get_ssh_key_path()
    ssh_pubkey = get_ssh_public_key()

    # Step 1: Delete existing VMs if present
    for vid in [setup_vmid, vmid]:
        try:
            proxmox.nodes(node).qemu(vid).status.current.get()
            print(f"  Deleting existing VM {vid}...")
            upid = proxmox.nodes(node).qemu(vid).delete(purge=1)
            wait_for_task(proxmox, node, upid)
        except Exception:
            pass  # VM doesn't exist

    # Step 2: Create base VM
    print("  Creating VM...")
    try:
        upid = proxmox.nodes(node).qemu.create(
            vmid=setup_vmid,
            name=f"{name}-setup",
            memory=config.get("memory", 2048),
            cores=config.get("cores", 2),
            sockets=1,
            ostype="l26",
            scsihw="virtio-scsi-single",
            agent="enabled=1",
            cpu="host",
            balloon=0,
            hotplug="disk,network,usb",
            serial0="socket",
            vga="std",
        )
        if not wait_for_task(proxmox, node, upid):
            print("  Failed to create VM")
            return False
    except Exception as e:
        print(f"  Error creating VM: {e}")
        return False

    # Step 3: Import the cloud image disk via SSH
    print("  Importing disk from cloud image...")
    img_path = f"/var/lib/vz/template/iso/{filename}"
    rc, out, err = run_proxmox_ssh(
        proxmox_host, proxmox_user, proxmox_password,
        f"qm disk import {setup_vmid} {img_path} local-lvm --format raw",
        timeout=600
    )
    if rc != 0:
        print(f"  Disk import failed: {err}")
        return False

    # Step 4: Attach the imported disk
    print("  Attaching disk...")
    try:
        proxmox.nodes(node).qemu(setup_vmid).config.put(
            scsi0=f"local-lvm:vm-{setup_vmid}-disk-0,discard=on,iothread=1"
        )
    except Exception as e:
        print(f"  Error attaching disk: {e}")
        return False

    # Step 5: Resize disk if needed
    if config.get("disk_resize"):
        print(f"  Resizing disk to {config['disk_resize']}...")
        try:
            proxmox.nodes(node).qemu(setup_vmid).resize.put(
                disk="scsi0",
                size=config["disk_resize"]
            )
        except Exception as e:
            print(f"  Warning: Disk resize failed: {e}")

    # Step 6: Add cloud-init drive with SSH key only
    print("  Adding cloud-init drive...")
    try:
        proxmox.nodes(node).qemu(setup_vmid).config.put(
            ide2="local-lvm:cloudinit"
        )
    except Exception as e:
        print(f"  Error adding cloud-init drive: {e}")
        return False

    # Step 7: Add network interface
    print("  Adding network interface...")
    try:
        proxmox.nodes(node).qemu(setup_vmid).config.put(
            net0="virtio,bridge=vmbr0"
        )
    except Exception as e:
        print(f"  Error adding network: {e}")
        return False

    # Step 8: Set boot order
    print("  Setting boot order...")
    try:
        proxmox.nodes(node).qemu(setup_vmid).config.put(
            boot="order=scsi0"
        )
    except Exception as e:
        print(f"  Error setting boot order: {e}")
        return False

    # Step 9: Configure cloud-init with SSH key (NOT password - that doesn't work for Rocky)
    print("  Configuring cloud-init (SSH key only)...")
    try:
        # Write SSH key to temp file on Proxmox host
        rc, _, err = run_proxmox_ssh(
            proxmox_host, proxmox_user, proxmox_password,
            f"echo '{ssh_pubkey}' > /tmp/vm_sshkey.pub"
        )
        if rc != 0:
            print(f"  Failed to write SSH key: {err}")
            return False

        # Use qm command to set SSH keys (API has issues with this)
        rc, _, err = run_proxmox_ssh(
            proxmox_host, proxmox_user, proxmox_password,
            f"qm set {setup_vmid} --sshkeys /tmp/vm_sshkey.pub --ipconfig0 ip=dhcp"
        )
        if rc != 0:
            print(f"  Failed to set cloud-init config: {err}")
            return False

    except Exception as e:
        print(f"  Error configuring cloud-init: {e}")
        return False

    # Step 10: Start VM and wait for boot
    print("  Starting VM...")
    try:
        upid = proxmox.nodes(node).qemu(setup_vmid).status.start.post()
        if not wait_for_task(proxmox, node, upid):
            print("  Failed to start VM")
            return False
    except Exception as e:
        print(f"  Error starting VM: {e}")
        return False

    # Step 11: Wait for VM to get IP address
    print("  Waiting for VM IP address", end="")
    vm_ip = get_vm_ip(proxmox, node, setup_vmid, timeout=180)
    if not vm_ip:
        print("\n  Failed to get VM IP address")
        return False
    print(f"\n  VM IP: {vm_ip}")

    # Step 12: Wait for SSH to become available
    print("  Waiting for SSH", end="")
    if not wait_for_vm_ssh(vm_ip, default_user, key_path, timeout=180):
        print("\n  SSH not available after timeout")
        return False
    print()

    # Step 13: Configure the VM (create labadmin, fix SSH, disable cloud-init)
    if not configure_vm_for_template(vm_ip, default_user, key_path, os_family):
        print("  VM configuration failed")
        return False

    # Step 14: Verify password authentication works
    time.sleep(3)  # Give sshd time to restart
    if not verify_password_auth(vm_ip):
        print("  Password authentication verification failed")
        # Continue anyway - template may still be usable

    # Step 15: Prepare VM for templating (cleanup)
    prepare_for_template(vm_ip, default_user, key_path)

    # Step 16: Shutdown VM
    print("  Shutting down VM...")
    try:
        proxmox.nodes(node).qemu(setup_vmid).status.shutdown.post()
        # Wait for shutdown
        for _ in range(60):
            status = proxmox.nodes(node).qemu(setup_vmid).status.current.get()
            if status.get("status") == "stopped":
                break
            time.sleep(2)
        else:
            # Force stop if graceful shutdown failed
            proxmox.nodes(node).qemu(setup_vmid).status.stop.post()
            time.sleep(5)
    except Exception as e:
        print(f"  Shutdown error (continuing): {e}")

    # Step 17: Remove cloud-init drive (no longer needed)
    print("  Removing cloud-init configuration...")
    try:
        rc, _, err = run_proxmox_ssh(
            proxmox_host, proxmox_user, proxmox_password,
            f"qm set {setup_vmid} --delete ide2,sshkeys,ipconfig0"
        )
    except Exception as e:
        print(f"  Warning: Could not remove cloud-init config: {e}")

    # Step 18: Clone to final template VMID
    print(f"  Cloning to template VMID {vmid}...")
    try:
        upid = proxmox.nodes(node).qemu(setup_vmid).clone.post(
            newid=vmid,
            name=name,
            full=1
        )
        if not wait_for_task(proxmox, node, upid, timeout=300):
            print("  Clone failed")
            return False
    except Exception as e:
        print(f"  Error cloning: {e}")
        return False

    # Step 19: Set template description
    desc = f"""{config.get('description', name)}

Credentials: {DEFAULT_LAB_USER} / {DEFAULT_LAB_PASSWORD}
- QEMU guest agent enabled
- SSH service enabled (password auth)
- SSH host keys auto-regenerate on first boot
- Cloud-init disabled
- Created: {time.strftime('%Y-%m-%d')}"""

    if with_wazuh:
        desc += f"\n- Wazuh agent configuration available"

    try:
        proxmox.nodes(node).qemu(vmid).config.put(description=desc)
    except Exception as e:
        print(f"  Warning: Could not set description: {e}")

    # Step 20: Convert to template
    print("  Converting to template...")
    try:
        proxmox.nodes(node).qemu(vmid).template.post()
    except Exception as e:
        print(f"  Error converting to template: {e}")
        return False

    # Step 21: Cleanup setup VM
    print("  Cleaning up setup VM...")
    try:
        upid = proxmox.nodes(node).qemu(setup_vmid).delete(purge=1)
        wait_for_task(proxmox, node, upid)
    except Exception as e:
        print(f"  Warning: Could not delete setup VM: {e}")

    print(f"\n  Template {name} (VMID: {vmid}) created successfully!")
    return True


def create_wazuh_snippet(proxmox, node: str, host: str, user: str, password: str) -> bool:
    """Create a cloud-init snippet for Wazuh agent installation."""
    print("\nCreating Wazuh cloud-init snippet...")

    wazuh_userdata = f"""#cloud-config
# Wazuh Agent Installation for Kootenai
# This snippet is applied to VMs that need Wazuh monitoring

package_update: true
package_upgrade: false

runcmd:
  # Install prerequisites
  - apt-get update -y || yum update -y
  # Add Wazuh repository (Debian/Ubuntu)
  - |
    if command -v apt-get &> /dev/null; then
      curl -s https://packages.wazuh.com/key/GPG-KEY-WAZUH | gpg --dearmor -o /usr/share/keyrings/wazuh.gpg
      echo "deb [signed-by=/usr/share/keyrings/wazuh.gpg] https://packages.wazuh.com/4.x/apt stable main" > /etc/apt/sources.list.d/wazuh.list
      apt-get update -y
      WAZUH_MANAGER="{WAZUH_MANAGER}" apt-get install -y wazuh-agent
      systemctl daemon-reload
      systemctl enable wazuh-agent
      systemctl start wazuh-agent
    fi
  # Add Wazuh repository (RHEL/Rocky/Alma)
  - |
    if command -v yum &> /dev/null; then
      rpm --import https://packages.wazuh.com/key/GPG-KEY-WAZUH
      cat > /etc/yum.repos.d/wazuh.repo << EOF
[wazuh]
gpgcheck=1
gpgkey=https://packages.wazuh.com/key/GPG-KEY-WAZUH
enabled=1
name=Wazuh repository
baseurl=https://packages.wazuh.com/4.x/yum/
protect=1
EOF
      WAZUH_MANAGER="{WAZUH_MANAGER}" yum install -y wazuh-agent
      systemctl daemon-reload
      systemctl enable wazuh-agent
      systemctl start wazuh-agent
    fi
  # Configure agent (if not already set by env var)
  - |
    if [ -f /var/ossec/etc/ossec.conf ]; then
      sed -i 's/MANAGER_IP/{WAZUH_MANAGER}/g' /var/ossec/etc/ossec.conf
    fi

final_message: "Wazuh agent installation complete. Manager: {WAZUH_MANAGER}"
"""

    snippet_path = "/var/lib/vz/snippets/wazuh-agent.yaml"

    rc, out, err = run_proxmox_ssh(
        host, user, password,
        f"mkdir -p /var/lib/vz/snippets && cat > {snippet_path} << 'WAZUH_EOF'\n{wazuh_userdata}\nWAZUH_EOF",
        timeout=30
    )

    if rc != 0:
        print(f"  Failed to create snippet: {err}")
        return False

    print(f"  Created {snippet_path}")
    return True


def list_templates(proxmox, node: str):
    """List existing templates on Proxmox."""
    print("\n=== Existing Templates on Proxmox ===")
    try:
        vms = proxmox.nodes(node).qemu.get()
        templates = [vm for vm in vms if vm.get("template", 0) == 1]

        if not templates:
            print("  No templates found")
        else:
            for t in sorted(templates, key=lambda x: x["vmid"]):
                print(f"  VMID {t['vmid']}: {t['name']} - {t.get('status', 'unknown')}")
    except Exception as e:
        print(f"  Error listing templates: {e}")

    print("\n=== Available Cloud Images ===")
    for key, img in CLOUD_IMAGES.items():
        print(f"  {key}: {img['name']} (VMID: {img['vmid']})")
        print(f"    {img['description']}")


def main():
    parser = argparse.ArgumentParser(
        description="Setup Cloud Image Templates for Kootenai",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
    python3 setup_cloud_templates.py --list
    python3 setup_cloud_templates.py --setup ubuntu-24.04
    python3 setup_cloud_templates.py --setup rocky-9
    python3 setup_cloud_templates.py --setup all
    python3 setup_cloud_templates.py --setup ubuntu-24.04 --with-wazuh
        """
    )
    parser.add_argument("--list", action="store_true", help="List available cloud images and existing templates")
    parser.add_argument("--setup", help="Setup template(s): 'all' or specific image name (e.g., ubuntu-24.04, rocky-9)")
    parser.add_argument("--with-wazuh", action="store_true", help="Create Wazuh cloud-init snippet")
    parser.add_argument("--vmid", type=int, help="Override VMID for the template")
    parser.add_argument("--force", action="store_true", help="Force recreation of existing templates")
    args = parser.parse_args()

    config = load_config()

    if not config["host"] or not config["password"]:
        print("Error: PROXMOX_HOST and PROXMOX_PASSWORD must be set")
        print("Set via environment variables or create api/.env.local")
        sys.exit(1)

    print("=" * 60)
    print("Kootenai Cloud Template Setup")
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

    # List mode
    if args.list:
        list_templates(proxmox, config["node"])
        sys.exit(0)

    # Setup mode
    if args.setup:
        # Create Wazuh snippet if requested
        if args.with_wazuh:
            create_wazuh_snippet(proxmox, config["node"], config["host"], config["user"], config["password"])

        # Determine which images to setup
        if args.setup == "all":
            images_to_setup = list(CLOUD_IMAGES.keys())
        elif args.setup in CLOUD_IMAGES:
            images_to_setup = [args.setup]
        else:
            print(f"Unknown image: {args.setup}")
            print(f"Available: {', '.join(CLOUD_IMAGES.keys())}")
            sys.exit(1)

        # Setup each image
        success_count = 0
        for image_key in images_to_setup:
            img_config = CLOUD_IMAGES[image_key].copy()

            # Override VMID if specified
            if args.vmid and len(images_to_setup) == 1:
                img_config["vmid"] = args.vmid

            # Download cloud image
            print(f"\nDownloading {img_config['filename']}...")
            if not download_cloud_image_ssh(
                config["host"], config["user"], config["password"],
                img_config["url"], img_config["filename"]
            ):
                print(f"Failed to download {image_key}")
                continue

            # Create template with full automation
            if create_template_from_cloud_image(
                proxmox, config["node"], img_config,
                config["host"], config["user"], config["password"],
                with_wazuh=args.with_wazuh
            ):
                success_count += 1

        print(f"\n{'='*60}")
        print(f"Setup complete: {success_count}/{len(images_to_setup)} templates created")
        print("=" * 60)

        if success_count > 0:
            print("\nNext steps:")
            print("1. Templates are ready for cloning")
            print(f"2. Default credentials: {DEFAULT_LAB_USER} / {DEFAULT_LAB_PASSWORD}")
            print("3. Use the API or UI to create lab pods from these templates")
            if args.with_wazuh:
                print("4. VMs cloned from these templates can have Wazuh pre-configured")

        sys.exit(0)

    # No action specified
    parser.print_help()


if __name__ == "__main__":
    main()
