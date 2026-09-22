# Kootenai Template Creation Guide

## Overview

This document describes the process for creating reliable VM templates for the Kootenai platform. These templates are used by the orchestrator to provision lab pods for students.

## Automated Template Creation (Recommended)

The `scripts/setup_cloud_templates.py` script fully automates template creation from cloud images, including handling the Rocky Linux cloud-init password issue.

```bash
# Install dependencies
pip install proxmoxer python-dotenv

# Set environment variables (or create .env.local)
export PROXMOX_HOST=<PROXMOX_IP>
export PROXMOX_USER=root@pam
export PROXMOX_PASSWORD=yourpassword

# List available images and existing templates
python3 scripts/setup_cloud_templates.py --list

# Create a single template
python3 scripts/setup_cloud_templates.py --setup ubuntu-24.04
python3 scripts/setup_cloud_templates.py --setup rocky-9

# Create all templates
python3 scripts/setup_cloud_templates.py --setup all

# Create with Wazuh agent snippet
python3 scripts/setup_cloud_templates.py --setup ubuntu-24.04 --with-wazuh
```

The script handles:
- Downloading cloud images to Proxmox
- Creating VMs with cloud-init (SSH key auth)
- Booting and configuring the VM via SSH
- Creating the `labadmin` user with password
- Fixing SSH to allow password authentication
- Disabling cloud-init
- Installing SSH host key regeneration service
- Cleaning up and converting to template

This approach works for all distributions including Rocky Linux, which has issues with cloud-init password processing.

## Manual Template Creation

The following sections describe manual template creation for cases where the automated script is not suitable.

## Key Requirements for Working Templates

Based on extensive testing, templates must meet these requirements:

| Requirement | Why It Matters |
|-------------|----------------|
| SSH **service** enabled (not socket) | Ubuntu 24.04's socket-based SSH activation causes connection issues |
| SSH host key regeneration service | Ensures unique host keys on each clone |
| Cloud-init disabled | Prevents interference with authentication and machine-id |
| QEMU guest agent installed | Required for IP discovery and guest commands |
| Known user/password | Consistent access across all cloned VMs |
| Wazuh agent installed | Required for checkpoint detection (optional) |

## Working Template: 9100 (Ubuntu 24.04)

### Specifications

| Component | Configuration |
|-----------|---------------|
| **VMID** | 9100 |
| **Name** | ubuntu-24.04-lab-template |
| **OS** | Ubuntu 24.04.3 LTS |
| **Disk** | 20GB |
| **Memory** | 2GB |
| **CPU** | 2 cores |
| **User** | labadmin |
| **Password** | (see `LAB_PASSWORD` env var) |
| **SSH** | Service enabled (not socket), password auth, auto-regenerating host keys |
| **QEMU Guest Agent** | Enabled and running |
| **Cloud-init** | Disabled |

### Creation Process

Template 9100 (v3) was created with these steps:

1. Fresh Ubuntu 24.04 installation from ISO
2. Created `labadmin` user with sudo access during installation
3. Installed OpenSSH server during installation
4. Post-installation configuration:
   - Installed qemu-guest-agent
   - Disabled ssh.socket, enabled ssh.service
   - Disabled cloud-init
   - Added SSH host key regeneration systemd service
5. Cleaned machine-specific identifiers
6. Converted to template

## Template Creation Checklist

When creating a new template, follow this checklist:

### 1. Base Installation
- [ ] Install OS from ISO or cloud image
- [ ] Create lab user: `labadmin` with password from `LAB_PASSWORD` env var
- [ ] Grant sudo access: `usermod -aG sudo labadmin` (or wheel for RHEL)
- [ ] Set hostname to generic name

### 2. SSH Configuration
- [ ] Install openssh-server
- [ ] **CRITICAL**: Disable socket activation, enable service:
  ```bash
  # Ubuntu 24.04
  systemctl disable ssh.socket
  systemctl stop ssh.socket
  systemctl enable ssh.service
  systemctl start ssh.service

  # Rocky/RHEL
  systemctl enable sshd
  systemctl start sshd
  ```
- [ ] Ensure password authentication is enabled in `/etc/ssh/sshd_config`:
  ```
  PasswordAuthentication yes
  ```

### 3. QEMU Guest Agent
- [ ] Install: `apt install qemu-guest-agent` or `dnf install qemu-guest-agent`
- [ ] Enable: `systemctl enable qemu-guest-agent`
- [ ] Start: `systemctl start qemu-guest-agent`

### 4. Wazuh Agent (Optional but Recommended)
- [ ] Install Wazuh agent (see Wazuh docs)
- [ ] Configure manager IP in `/var/ossec/etc/ossec.conf`
- [ ] Enable service: `systemctl enable wazuh-agent`
- [ ] Note: Agent will need registration after clone

### 5. Cloud-init Handling
- [ ] If cloud-init is installed, disable it:
  ```bash
  touch /etc/cloud/cloud-init.disabled
  systemctl disable cloud-init
  ```
- [ ] Clean any cloud-init state: `cloud-init clean`

### 6. SSH Host Key Regeneration Service
- [ ] **CRITICAL**: Create a systemd service to regenerate SSH host keys on first boot:
  ```bash
  cat > /etc/systemd/system/regenerate-ssh-host-keys.service << 'EOF'
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
  EOF
  ```
- [ ] Enable the service:
  ```bash
  systemctl enable regenerate-ssh-host-keys.service
  ```

This service ensures that when SSH host keys are removed before templating, they are automatically regenerated on the first boot of each clone. Without this, SSH will fail to start on cloned VMs.

### 7. Pre-Template Cleanup
Before converting to template, clean machine-specific data:

```bash
# Truncate machine-id (will regenerate on boot)
truncate -s 0 /etc/machine-id

# Remove SSH host keys (will regenerate via our service on first boot)
rm -f /etc/ssh/ssh_host_*

# Clean Wazuh agent keys (needs re-registration)
rm -f /var/ossec/etc/client.keys

# Clear bash history
history -c
cat /dev/null > ~/.bash_history

# Clear logs
journalctl --rotate
journalctl --vacuum-time=1s

# Shutdown
shutdown -h now
```

### 8. Convert to Template (on Proxmox)
```bash
# Create a snapshot first (safety)
qm snapshot <VMID> pre-template -description "Before template conversion"

# Clone to template VMID (9xxx range)
qm clone <VMID> 9XXX --name "descriptive-template-name" --full

# Update description
qm set 9XXX --description "Template description with credentials"

# Convert to template
qm template 9XXX
```

## Configuration Mapping

Templates are mapped to lab definitions in `config/config.yaml`:

```yaml
template_vmids:
  # Ubuntu templates
  ubuntu-lab-wazuh: 9100
  ubuntu-22.04-desktop: 9100
  ubuntu-22.04-server: 9100
  ubuntu-24.04-server: 9100

  # Rocky Linux templates
  rocky9-minimal-cloud: 9101
  rocky9-desktop-minimal: 9101
  rocky9-desktop-soc: 9101
```

And in the deployed `.env` file:
```
TEMPLATE_VMIDS=ubuntu-lab-wazuh:9100,ubuntu-22.04-server:9100,rocky9-minimal-cloud:9101
```

## Troubleshooting

### SSH Connection Issues

**Symptom**: "Connection reset by peer" on SSH connect

**Cause**: Ubuntu 24.04 uses socket-based SSH activation by default

**Fix**:
```bash
systemctl disable ssh.socket
systemctl enable ssh.service
systemctl restart ssh.service
```

### Password Authentication Fails

**Symptom**: "Permission denied" even with correct password

**Possible Causes**:
1. Cloud-init changed the password
2. PAM restrictions
3. SSH config doesn't allow password auth

**Fix**: Check `/etc/ssh/sshd_config` and disable cloud-init

### Same IP for Multiple VMs

**Symptom**: Cloned VMs get the same IP address

**Cause**: Same machine-id causes DHCP to assign same lease

**Fix**: Ensure machine-id is truncated before templating (will regenerate on boot)

### Template Shows Wrong Base

**Symptom**: API creates VMs from wrong template

**Cause**: Environment variables not updated in container

**Fix**: Recreate container (not just restart):
```bash
docker compose up -d --force-recreate api
```

### Rocky Linux Cloud Image Password Not Working

**Symptom**: Can't login with password via SSH or VNC console after using `cicustom` user-data

**Cause**: Rocky Linux GenericCloud images have restrictive cloud-init behavior that doesn't properly process custom user-data passwords, even with correct SHA-512 hashes

**What DOESN'T work**:
- `cicustom "user=local:snippets/rocky-userdata.yml"` with hashed passwords
- `cipassword` parameter
- Any combination of cloud-init password settings

**What WORKS**:
1. Use the default `cloud-user` username with SSH key authentication
2. SSH in and manually create `labadmin` user with `useradd` + `chpasswd`
3. Remove `/etc/ssh/sshd_config.d/50-cloud-init.conf` (disables password auth)

See "Creating a Rocky Linux Template from Cloud Image" section for full working procedure.

### SSH Authentication Methods Not Available

**Symptom**: SSH says "Permission denied (publickey,gssapi-keyex,gssapi-with-mic)" - no password option

**Cause**: Cloud-init created `/etc/ssh/sshd_config.d/50-cloud-init.conf` with `PasswordAuthentication no`

**Fix**:
```bash
sudo rm /etc/ssh/sshd_config.d/50-cloud-init.conf
sudo systemctl restart sshd
```

### SSH Host Keys Not Regenerating on Clone

**Symptom**: Multiple cloned VMs have the same SSH host key, causing "host key changed" warnings

**Cause**: SSH host key regeneration service not installed or not working

**Fix**: Ensure the systemd service exists and is enabled:
```bash
# Check service exists
systemctl status regenerate-ssh-host-keys.service

# Check ConditionPathExists syntax (no shell escaping issues)
cat /etc/systemd/system/regenerate-ssh-host-keys.service | grep Condition
# Should show: ConditionPathExists=!/etc/ssh/ssh_host_ed25519_key
# NOT: ConditionPathExists=\!/etc/ssh/ssh_host_ed25519_key
```

## Template Inventory

| VMID | Name | OS | Status | Notes |
|------|------|-----|--------|-------|
| 9002 | ubuntu-lab-template | Ubuntu 22.04 | Deprecated | SSH socket issues |
| 9003 | ubuntu-lab-wazuh-template | Ubuntu 22.04 | Deprecated | SSH socket issues |
| 9100 | ubuntu-24.04-lab-template | Ubuntu 24.04 | **Working** | Primary template (v3), SSH host keys auto-regenerate |
| 9101 | rocky-10-lab-template | Rocky 10.1 | **Working** | Created from cloud image, SSH host keys auto-regenerate |

## Creating a Rocky Linux Template from Cloud Image

This is the working approach for creating Rocky Linux templates from cloud images. Template 9101 was created using this method.

> **IMPORTANT**: Rocky Linux cloud images have restrictive cloud-init behavior. Custom user-data with passwords (`cicustom`) does NOT work reliably - passwords are not set even with correct SHA-512 hashes. The working approach is to use the default `cloud-user` with SSH key authentication, then manually configure the `labadmin` user after boot.

### Step 1: Download Cloud Image

```bash
# On Proxmox host
cd /var/lib/vz/template/iso/
wget https://dl.rockylinux.org/pub/rocky/10/images/x86_64/Rocky-10-GenericCloud-Base.latest.x86_64.qcow2 \
  -O Rocky-10-GenericCloud.qcow2
```

### Step 2: Create VM from Cloud Image (Using Default cloud-user)

```bash
# Create VM
qm create 420913 --name "rocky-10-cloud-setup" --memory 2048 --cores 2 \
    --cpu host --net0 virtio,bridge=vmbr0 --scsihw virtio-scsi-single \
    --ostype l26 --agent enabled=1 --balloon 0 --vga std

# Import cloud image disk
qm importdisk 420913 /var/lib/vz/template/iso/Rocky-10-GenericCloud.qcow2 local-lvm

# Attach disk and configure boot
qm set 420913 --scsi0 local-lvm:vm-420913-disk-0,discard=on,iothread=1
qm set 420913 --boot order=scsi0

# Add cloud-init with SSH key ONLY (no cicustom, no cipassword)
qm set 420913 --ide2 local-lvm:cloudinit
qm set 420913 --ciuser cloud-user
qm set 420913 --sshkeys ~/.ssh/id_ed25519.pub  # Your SSH public key
qm set 420913 --ipconfig0 ip=dhcp

# Start VM
qm start 420913
```

### Step 3: Get VM IP and SSH In

```bash
# Wait ~45 seconds for boot, then get IP via QEMU agent
qm guest cmd 420913 network-get-interfaces

# SSH with key as cloud-user
ssh -i ~/.ssh/id_ed25519 cloud-user@<VM_IP>
```

### Step 4: Configure labadmin User and SSH

Once logged in as `cloud-user`, configure the template:

```bash
# Create labadmin user with password
sudo useradd -m -G wheel -s /bin/bash labadmin
echo "labadmin:$LAB_PASSWORD" | sudo chpasswd

# Enable SSH password authentication
# CRITICAL: Remove cloud-init's SSH config that disables password auth
sudo rm -f /etc/ssh/sshd_config.d/50-cloud-init.conf

# Enable password auth in main config
sudo sed -i 's/^#*PasswordAuthentication.*/PasswordAuthentication yes/' /etc/ssh/sshd_config
echo 'PasswordAuthentication yes' | sudo tee /etc/ssh/sshd_config.d/99-pwauth.conf
sudo systemctl restart sshd

# Disable cloud-init
sudo touch /etc/cloud/cloud-init.disabled
sudo systemctl disable cloud-init cloud-init-local cloud-config cloud-final

# Ensure services are enabled
sudo systemctl enable qemu-guest-agent sshd

# Create SSH host key regeneration service
echo '[Unit]
Description=Regenerate SSH Host Keys on First Boot
Before=sshd.service
ConditionPathExists=!/etc/ssh/ssh_host_ed25519_key

[Service]
Type=oneshot
ExecStart=/usr/bin/ssh-keygen -A
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target' | sudo tee /etc/systemd/system/regenerate-ssh-host-keys.service

sudo systemctl enable regenerate-ssh-host-keys.service
```

### Step 5: Verify Password Authentication Works

Before proceeding, verify you can SSH with the new credentials:

```bash
# From your local machine, test password auth
sshpass -p "$LAB_PASSWORD" ssh labadmin@<VM_IP> "echo 'Password auth works!'"
```

### Step 6: Prepare for Templating

```bash
# SSH back in and clean up
ssh -i ~/.ssh/id_ed25519 cloud-user@<VM_IP>

# Pre-template cleanup
sudo truncate -s 0 /etc/machine-id
sudo cloud-init clean
sudo rm -f /etc/ssh/ssh_host_*

# Clear cloud-user's authorized_keys (don't want setup SSH key in template)
rm -f ~/.ssh/authorized_keys

cat /dev/null > ~/.bash_history
history -c
sudo shutdown -h now
```

### Step 7: Remove Cloud-Init Drive and Convert to Template

```bash
# On Proxmox host - wait for VM to stop
qm status 420913

# Remove cloud-init settings
qm set 420913 --delete ide2,ciuser,sshkeys,ipconfig0

# Clone to template
qm clone 420913 9101 --name "rocky-10-lab-template" --full

# Set description
qm set 9101 --description "Rocky Linux 10 Lab Template
- OS: Rocky Linux 10.1
- User: labadmin / (see LAB_PASSWORD env var)
- QEMU guest agent enabled
- SSH service enabled (password auth)
- SSH host keys auto-regenerate on first boot
- Cloud-init disabled
- Created: $(date +%Y-%m-%d)"

# Convert to template
qm template 9101

# Clean up source VM
qm destroy 420913 --purge
```

## Creating a Rocky Linux Template from ISO (Alternative)

If you prefer to install from ISO for more control, follow these steps:

### Step 1: Create VM from ISO

```bash
qm create 420901 --name "rocky-10-setup" --memory 2048 --cores 2 \
  --net0 virtio,bridge=vmbr0 \
  --scsi0 local-lvm:20 \
  --cdrom local:iso/Rocky-10.1-x86_64-dvd1.iso \
  --boot order=scsi0 \
  --ostype l26 \
  --scsihw virtio-scsi-single \
  --agent enabled=1

qm start 420901
```

### Step 2: Install Rocky Linux (via VNC console)

1. Open Proxmox web UI and access VM console
2. Install Rocky Linux with these settings:
   - Minimal install (Server without GUI)
   - Create user `labadmin` with password from `LAB_PASSWORD` env var
   - Give labadmin admin privileges
   - Set hostname to `rocky-template`

### Step 3: Post-Installation Configuration

SSH to the VM after installation completes and run the same fixes as Step 4 above, plus:

```bash
# Update system
dnf update -y

# Install QEMU guest agent
dnf install -y qemu-guest-agent
systemctl enable qemu-guest-agent
systemctl start qemu-guest-agent

# Enable password authentication
sed -i 's/^PasswordAuthentication no/PasswordAuthentication yes/' /etc/ssh/sshd_config
systemctl restart sshd
```

Then follow Steps 5-6 from the cloud image approach above

## References

- [Proxmox VM Templates](https://pve.proxmox.com/wiki/VM_Templates_and_Clones)
- [Wazuh Agent Installation](https://documentation.wazuh.com/current/installation-guide/wazuh-agent/index.html)
- [Ubuntu SSH Configuration](https://ubuntu.com/server/docs/service-openssh)
- [Rocky Linux Installation Guide](https://docs.rockylinux.org/guides/installation/)
