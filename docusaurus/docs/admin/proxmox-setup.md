# Proxmox Setup Guide

This guide covers connecting Kootenai to your Proxmox VE cluster for real VM provisioning.

## Prerequisites

- Proxmox VE 7.0+ installed and running
- Network access from Kootenai server to Proxmox
- Admin access to Proxmox to create API tokens
- At least one VM template prepared

## Architecture Overview

```
┌─────────────────────┐     HTTPS/API      ┌─────────────────────┐
│   Kootenai API   │ ◄──────────────────► │    Proxmox VE      │
│   (labctl)          │                      │    Cluster         │
└─────────────────────┘                      └─────────────────────┘
         │                                            │
         │ Creates/Manages                            │ Hosts
         ▼                                            ▼
┌─────────────────────┐                      ┌─────────────────────┐
│   Pods (metadata)   │ ◄────────────────────► │   VMs (actual)     │
│   in PostgreSQL     │     1:1 mapping       │   on Proxmox       │
└─────────────────────┘                      └─────────────────────┘
```

## Step 1: Create Proxmox API Token

### Via Web UI (Recommended)

1. Log into Proxmox Web UI (`https://your-proxmox:8006`)

2. Navigate to: **Datacenter** → **Permissions** → **API Tokens**

3. Click **Add**:
   - **User**: `root@pam` (or a dedicated user)
   - **Token ID**: `kootenai`
   - **Privilege Separation**: **Unchecked** (for full access)
   - **Expire**: Leave empty (no expiration)

4. Click **Add** and **SAVE THE TOKEN SECRET** - it's only shown once!

   ```
   Token ID: root@pam!kootenai
   Secret: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
   ```

### Via CLI

```bash
# SSH to Proxmox host
ssh root@your-proxmox

# Create token
pveum user token add root@pam kootenai --privsep 0

# Output shows the secret - save it!
```

## Step 2: Configure Kootenai

### Environment Variables

Add to your `.env` file:

```bash
# Proxmox Configuration
PROXMOX_HOST=https://your-proxmox.local:8006
PROXMOX_NODE=pve                           # Your Proxmox node name
PROXMOX_TOKEN_ID=root@pam!kootenai       # From step 1
PROXMOX_TOKEN=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx

# Optional: Skip TLS verification (for self-signed certs)
PROXMOX_INSECURE=true

# Optional: Connection settings
PROXMOX_TIMEOUT=30s
```

### Finding Your Node Name

```bash
# On Proxmox host
hostname

# Or via API
curl -k https://your-proxmox:8006/api2/json/nodes \
  -H "Authorization: PVEAPIToken=root@pam!kootenai=YOUR_SECRET"
```

## Step 3: Test Connection

### Using curl

```bash
# Test API access
curl -k "https://your-proxmox:8006/api2/json/version" \
  -H "Authorization: PVEAPIToken=root@pam!kootenai=YOUR_SECRET"

# Expected response:
# {"data":{"version":"7.4-3","release":"7.4",...}}
```

### Using Kootenai

```bash
# Start the API
cd api && go run ./cmd/labctl serve

# Check health endpoint
curl http://localhost:8080/ready | jq .checks.proxmox

# Expected:
# {"status":"healthy","circuit_state":"closed"}
```

### Using Mage

```bash
# Test Proxmox connection
mage proxmox:test

# List VMs on Proxmox
mage proxmox:listVMs

# List templates
mage proxmox:listTemplates
```

## Step 4: Create VM Templates

Lab pods are created by cloning VM templates. You need at least one template.

### Naming Convention

Templates should follow this naming pattern:
```
<os>-<version>-template

Examples:
- ubuntu-22.04-template
- rocky-9-template
- windows-2022-template
```

### Creating a Template (Manual)

1. **Create a VM** in Proxmox with your desired OS

2. **Install required software**:
   ```bash
   # For Linux VMs - install Wazuh agent
   curl -s https://packages.wazuh.com/4.x/apt/pool/main/w/wazuh-agent/wazuh-agent_4.7.0-1_amd64.deb -o wazuh-agent.deb
   WAZUH_MANAGER='your-wazuh-manager' dpkg -i wazuh-agent.deb

   # Install cloud-init (for automated configuration)
   apt install cloud-init
   ```

3. **Configure cloud-init** (`/etc/cloud/cloud.cfg`):
   ```yaml
   users:
     - default
     - name: student
       sudo: ALL=(ALL) NOPASSWD:ALL
       shell: /bin/bash
       lock_passwd: false

   chpasswd:
     expire: false

   ssh_pwauth: true
   ```

4. **Clean up for template**:
   ```bash
   # Remove SSH host keys (regenerated on clone)
   rm -f /etc/ssh/ssh_host_*

   # Clean cloud-init
   cloud-init clean

   # Clear logs
   truncate -s 0 /var/log/*.log

   # Clear bash history
   history -c
   ```

5. **Convert to template** in Proxmox UI:
   - Right-click VM → **Convert to Template**

### Creating a Template (Automated)

Use the provided script:

```bash
cd scripts
python3 setup_cloud_templates.py --help

# Create Ubuntu 22.04 template
python3 setup_cloud_templates.py \
  --image ubuntu-22.04 \
  --vmid 9000 \
  --name ubuntu-22.04-template
```

See [VM Template Creation Guide](./proxmox-vm-templates.md) for detailed instructions.

## Step 5: Configure Lab Templates

Lab templates reference Proxmox VM templates by name:

```yaml
# templates/linux-pathway/linux-foundations.yaml
apiVersion: v1
kind: LabTemplate
metadata:
  name: "Linux Foundations"
  description: "Learn basic Linux commands"
spec:
  platform: proxmox
  network:
    segments:
      - name: lab-net
        vlan: 100
        subnet: "10.0.100.0/24"
  vms:
    - name: student-vm
      template: "ubuntu-22.04-template"  # Must exist in Proxmox
      memory: 2048
      cores: 2
      network: lab-net
      cloudInit:
        user: student
        password: student123
```

## Step 6: Network Configuration

### VLAN Setup

Kootenai creates isolated networks using VLANs:

1. **Configure Proxmox bridge** for VLAN awareness:
   ```bash
   # In /etc/network/interfaces
   auto vmbr0
   iface vmbr0 inet static
       address <PROXMOX_IP>/24
       gateway 192.168.1.1
       bridge-ports eno1
       bridge-stp off
       bridge-fd 0
       bridge-vlan-aware yes  # Enable VLAN awareness
   ```

2. **Reload network**:
   ```bash
   ifreload -a
   ```

### VLAN assignment

There is no global VLAN range setting. Each lab template declares the VLAN for
every network segment it defines, so the range in use is whatever your
templates ask for:

```yaml
# In a lab template's spec.networks
networks:
  - name: lab-network
    vlan: 100
    subnet: 10.10.100.0/24
```

The shipped templates use VLANs in the 100-999 band by convention. Make sure
that band is trunked to the Proxmox bridge your pods attach to.

### Firewall Rules

If using Proxmox firewall, allow traffic between VMs in the same VLAN:

```bash
# /etc/pve/firewall/cluster.fw
[RULES]
IN ACCEPT -source +lab-vms
OUT ACCEPT -dest +lab-vms
```

## Troubleshooting

### "Connection refused" or "No route to host"

```bash
# Check Proxmox is accessible
ping your-proxmox.local
curl -k https://your-proxmox:8006

# Check firewall
# On Proxmox:
iptables -L -n | grep 8006
```

### "401 Unauthorized"

```bash
# Verify token format
echo $PROXMOX_TOKEN_ID  # Should be: user@realm!tokenid

# Test token directly
curl -k "https://your-proxmox:8006/api2/json/version" \
  -H "Authorization: PVEAPIToken=$PROXMOX_TOKEN_ID=$PROXMOX_TOKEN"
```

### "SSL certificate problem"

```bash
# Option 1: Add cert to trusted store
# Option 2: Enable insecure mode (dev only)
PROXMOX_INSECURE=true
```

### "Template not found"

```bash
# List templates on Proxmox
curl -k "https://your-proxmox:8006/api2/json/nodes/pve/qemu" \
  -H "Authorization: PVEAPIToken=..." | jq '.data[] | select(.template==1)'

# Ensure template name matches lab spec exactly
```

### "VLAN not working"

```bash
# Check bridge is VLAN-aware
cat /etc/network/interfaces | grep vlan-aware

# Check VM is using correct bridge
qm config <vmid> | grep net

# Test VLAN from inside VM
ip link show | grep vlan
```

### Circuit Breaker Open

If Proxmox health shows "unhealthy" with circuit_state "open":

```bash
# Check API logs
docker compose logs api | grep proxmox

# Circuit opens after repeated failures
# Wait 30 seconds for half-open state, then retry
```

## Performance characteristics

None of the following is configurable — this section documents what the code
does, so you can plan capacity around it.

### Clone mode

Pods are always provisioned with **linked clones**. Both call sites pass
`linked=true` unconditionally (`orchestrator.go` and `async_provisioning.go`),
which becomes `full=0` on the Proxmox clone API.

This is what makes provisioning fast, and it has one hard consequence:
**a linked clone shares its base disk with the template, so deleting or
moving a template breaks every pod cloned from it.** Treat templates as
immutable once any pod exists.

### Storage

The clone call does not pass a `storage` parameter, so Proxmox places each
clone on the same storage as its template. To control where pods land, place
the template on the storage you want.

### Concurrency

There is no concurrency limit on VM operations. Pods are provisioned as
requested, and the async provisioner does not queue or throttle. On a
single-node Proxmox host, a burst of simultaneous pod creations is bounded by
the host's own I/O, not by this application.

## Security Best Practices

1. **Use dedicated API user** instead of root:
   ```bash
   pveum user add labadmin@pve
   pveum aclmod / -user labadmin@pve -role PVEVMAdmin
   pveum user token add labadmin@pve kootenai
   ```

2. **Enable TLS verification** in production:
   ```bash
   PROXMOX_INSECURE=false
   ```

3. **Restrict token permissions** if possible:
   ```bash
   # Create custom role with minimal permissions
   pveum role add LabRole -privs "VM.Allocate VM.Clone VM.Config.Disk VM.Config.Network VM.PowerMgmt VM.Console"
   pveum aclmod /vms -user labadmin@pve -role LabRole
   ```

4. **Use network segmentation**:
   - Lab VLANs should be isolated from production
   - Student VMs shouldn't access Proxmox management

## Next Steps

- [Create VM Templates](./proxmox-vm-templates.md) - Detailed template creation
- [Lab Template Guide](../lab-templates/template-creation-guide.md) - Create lab content
- [Wazuh Integration](./wazuh-integration.md) - Set up checkpoint detection
