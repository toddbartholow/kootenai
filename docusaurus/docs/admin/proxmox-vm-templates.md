# Proxmox VM Templates Setup Guide

This guide describes the VM templates that need to be created on the Proxmox server for the Kootenai platform.

## Overview

The Kootenai platform clones VMs from templates stored in Proxmox. These templates must be created manually on the Proxmox server before labs can be provisioned.

## Required Templates

### Primary Linux Template

**Template Name:** `ubuntu-22.04-server`

**Purpose:** Used by most Linux and cybersecurity labs

**Base Image:** Ubuntu 22.04.x LTS Server

**Configuration:**
- CPU: 1 core (labs override as needed)
- Memory: 2048 MB
- Disk: 20 GB (thin provisioned)
- Network: DHCP on default bridge
- Cloud-init: Enabled for user/network configuration

**Required Software:**
```bash
# Base packages
apt update && apt upgrade -y
apt install -y qemu-guest-agent cloud-init openssh-server curl wget vim nano

# Enable guest agent
systemctl enable qemu-guest-agent
systemctl start qemu-guest-agent
```

**Wazuh Agent Installation:**
```bash
# Add Wazuh repository
curl -s https://packages.wazuh.com/key/GPG-KEY-WAZUH | gpg --dearmor -o /usr/share/keyrings/wazuh.gpg
echo "deb [signed-by=/usr/share/keyrings/wazuh.gpg] https://packages.wazuh.com/4.x/apt stable main" > /etc/apt/sources.list.d/wazuh.list

# Install agent
apt update
apt install -y wazuh-agent

# Configure agent (update MANAGER_IP)
sed -i 's/MANAGER_IP/<INFRA_VM_IP>/' /var/ossec/etc/ossec.conf

# Enable but don't start (will be configured per-lab)
systemctl enable wazuh-agent
```

**Create Template:**
```bash
# On Proxmox host, after VM is configured:
qm template <VMID>
```

---

### Windows Server Template

**Template Name:** `windows-server-2022`

**Purpose:** Active Directory and Windows administration labs

**Base Image:** Windows Server 2022 Standard/Datacenter

**Configuration:**
- CPU: 2 cores
- Memory: 4096 MB
- Disk: 60 GB
- Network: DHCP on default bridge

**Required Software:**
- QEMU Guest Agent for Windows
- OpenSSH Server (optional)
- Wazuh Agent for Windows

**Sysprep:** Run sysprep with OOBE/Generalize before converting to template

---

### pfSense Template

**Template Name:** `pfsense-2.x`

**Purpose:** Firewall and network security labs

**Base Image:** pfSense CE 2.7.x

**Configuration:**
- CPU: 1 core
- Memory: 1024 MB
- Disk: 8 GB
- Network: 2 NICs (WAN and LAN)

**Post-Install:**
- Complete initial setup wizard
- Enable SSH access
- Configure default credentials

---

### Kali Linux Template

**Template Name:** `kali-linux`

**Purpose:** Penetration testing and security labs

**Base Image:** Kali Linux (latest rolling)

**Configuration:**
- CPU: 2 cores
- Memory: 4096 MB
- Disk: 40 GB
- Network: DHCP

**Required Software:**
- QEMU Guest Agent
- Default Kali metapackages
- Wazuh Agent

---

## Template Creation Workflow

### 1. Create Base VM

```bash
# Create VM with specific VMID (use 9000+ range for templates)
qm create 9001 --name ubuntu-22.04-server --memory 2048 --cores 1 \
  --net0 virtio,bridge=vmbr0 --scsihw virtio-scsi-single

# Add disk
qm set 9001 --scsi0 local-lvm:20,discard=on,ssd=1

# Add cloud-init drive
qm set 9001 --ide2 local-lvm:cloudinit

# Set boot order
qm set 9001 --boot order=scsi0
```

### 2. Install OS and Software

1. Attach ISO and boot VM
2. Complete OS installation
3. Install required packages (see above)
4. Configure Wazuh agent
5. Clean up (remove SSH host keys, clear logs)

### 3. Enable Proxmox Guest Agent Communication

Before converting to a template, ensure the VM has the `agent: 1` flag set in its Proxmox configuration. This tells Proxmox to communicate with the QEMU guest agent inside the VM, which is required for IP address discovery and other guest operations.

```bash
# Verify agent flag is set
qm config 9001 | grep agent

# If missing, set it
qm set 9001 --agent 1
```

:::caution
Installing `qemu-guest-agent` inside the VM is not enough — the Proxmox-side `agent: 1` flag must also be set. Without it, Proxmox won't attempt to communicate with the guest agent, and IP lookup via the API will fail.
:::

### 4. Convert to Template

```bash
# Shutdown VM
qm shutdown 9001

# Convert to template
qm template 9001
```

### 4. Verify Template

```bash
# List templates
qm list | grep template

# Check template config
qm config 9001
```

## Template Naming Convention

| Template Name | VMID Range | Purpose |
|---------------|------------|---------|
| `ubuntu-22.04-server` | 9001-9010 | Linux labs |
| `ubuntu-20.04` | 9011-9020 | Legacy Linux |
| `windows-server-2022` | 9021-9030 | Windows/AD labs |
| `pfsense-2.x` | 9031-9040 | Firewall labs |
| `kali-linux` | 9041-9050 | Security labs |

## Cloud-Init Configuration

Templates should support cloud-init for automated configuration:

```yaml
# Example cloud-init user-data
#cloud-config
hostname: ${VM_NAME}
users:
  - name: student
    sudo: ALL=(ALL) NOPASSWD:ALL
    shell: /bin/bash
    ssh_authorized_keys:
      - ${SSH_PUBLIC_KEY}
```

## Wazuh Agent Configuration

Each lab VM should have Wazuh agent configured to report to the manager:

**Manager Address:** `<INFRA_VM_IP>` (Infra VM)

**Agent Groups:** Labs can specify agent groups for different monitoring profiles:
- `linux-labs` - Standard Linux monitoring
- `security-labs` - Enhanced security monitoring
- `command-audit` - Command execution auditing

## Troubleshooting

### Template Clone Fails

```bash
# Check storage space
pvesm status

# Check template exists
qm list | grep 9001
```

### Guest Agent Not Responding

```bash
# On guest VM
systemctl status qemu-guest-agent

# On Proxmox host
qm agent <VMID> ping
```

### Wazuh Agent Not Connecting

```bash
# Check agent status
systemctl status wazuh-agent

# Check manager connectivity
/var/ossec/bin/agent-control -l

# Check logs
tail -f /var/ossec/logs/ossec.log
```
