-- Migration 038: Update lab template specs to use available Proxmox templates
-- Maps old template names to actually available templates on Proxmox:
--   ubuntu-lab-template (VMID 9002) - generic Ubuntu
--   ubuntu-lab-wazuh-template (VMID 9003) - Ubuntu with Wazuh agent
--   ubuntu-24.04-lab-template (VMID 9100) - Ubuntu 24.04
--   rocky-10-lab-template (VMID 9101) - Rocky Linux 10
--
-- NOTE: pfSense and Windows Server templates still need to be created on Proxmox

-- +goose Up

-- Update Linux Firewall Basics lab
UPDATE lab_templates
SET spec = '{
    "platform": "proxmox",
    "network": {
        "segments": [
            {"name": "lab-network", "vlan": 100, "subnet": "10.10.100.0/24", "gateway": "10.10.100.1", "dhcp": false}
        ]
    },
    "vms": [
        {
            "name": "workstation",
            "template": "ubuntu-lab-wazuh-template",
            "wazuhAgent": true,
            "agentConfig": {
                "monitor_paths": ["/etc/ufw", "/etc/default/ufw", "/var/log/ufw.log"],
                "audit_commands": true,
                "realtime": true
            },
            "resources": {"cpu": 2, "memory": 4096, "disk": 32},
            "networks": [{"segment": "lab-network", "ip": "10.10.100.10"}],
            "snapshots": [{"name": "initial", "description": "Fresh Ubuntu installation", "default": true}]
        },
        {
            "name": "target-server",
            "template": "ubuntu-lab-wazuh-template",
            "wazuhAgent": true,
            "agentConfig": {
                "monitor_paths": ["/var/log/auth.log"],
                "audit_commands": false,
                "realtime": true
            },
            "resources": {"cpu": 1, "memory": 2048, "disk": 16},
            "networks": [{"segment": "lab-network", "ip": "10.10.100.20"}],
            "snapshots": [{"name": "initial", "description": "Server with SSH and HTTP running", "default": true}]
        }
    ]
}'::jsonb,
    updated_at = NOW()
WHERE name = 'Linux Firewall Basics';

-- Update Linux Routing Fundamentals lab
UPDATE lab_templates
SET spec = '{
    "platform": "proxmox",
    "network": {
        "segments": [
            {"name": "network-a", "vlan": 101, "subnet": "10.10.101.0/24", "gateway": "10.10.101.1", "dhcp": false},
            {"name": "network-b", "vlan": 102, "subnet": "10.10.102.0/24", "gateway": "10.10.102.1", "dhcp": false}
        ]
    },
    "vms": [
        {
            "name": "router",
            "template": "ubuntu-lab-wazuh-template",
            "wazuhAgent": true,
            "agentConfig": {
                "monitor_paths": ["/etc/sysctl.conf", "/etc/sysctl.d", "/etc/netplan", "/etc/network/interfaces"],
                "audit_commands": true,
                "realtime": true
            },
            "resources": {"cpu": 1, "memory": 1024, "disk": 8},
            "networks": [
                {"segment": "network-a", "ip": "10.10.101.1"},
                {"segment": "network-b", "ip": "10.10.102.1"}
            ],
            "snapshots": [{"name": "initial", "description": "Base router - IP forwarding disabled", "default": true}]
        },
        {
            "name": "client-a",
            "template": "ubuntu-lab-template",
            "wazuhAgent": true,
            "agentConfig": {"monitor_paths": ["/etc/netplan"], "audit_commands": true, "realtime": true},
            "resources": {"cpu": 1, "memory": 2048, "disk": 16},
            "networks": [{"segment": "network-a", "ip": "10.10.101.10"}],
            "snapshots": [{"name": "initial", "description": "Client on Network A", "default": true}]
        },
        {
            "name": "client-b",
            "template": "ubuntu-lab-template",
            "wazuhAgent": true,
            "agentConfig": {"monitor_paths": ["/etc/netplan"], "audit_commands": true, "realtime": true},
            "resources": {"cpu": 1, "memory": 2048, "disk": 16},
            "networks": [{"segment": "network-b", "ip": "10.10.102.10"}],
            "snapshots": [{"name": "initial", "description": "Client on Network B", "default": true}]
        }
    ]
}'::jsonb,
    updated_at = NOW()
WHERE name = 'Linux Routing Fundamentals';

-- Update pfSense Firewall Fundamentals lab (uses rocky for clients, still needs pfsense template)
-- NOTE: pfsense-ce-2.7 template still needs to be created on Proxmox
UPDATE lab_templates
SET spec = '{
    "platform": "proxmox",
    "network": {
        "segments": [
            {"name": "wan", "vlan": 200, "subnet": "10.99.99.0/24", "gateway": "10.99.99.1"},
            {"name": "lan", "vlan": 201, "subnet": "192.168.10.0/24", "gateway": "192.168.10.1", "dhcp": true},
            {"name": "dmz", "vlan": 202, "subnet": "192.168.20.0/24", "gateway": "192.168.20.1"}
        ]
    },
    "vms": [
        {
            "name": "pfsense",
            "template": "pfsense-ce-2.7",
            "resources": {"cpu": 2, "memory": 2048, "disk": 20},
            "networks": [
                {"segment": "wan", "ip": "10.99.99.2"},
                {"segment": "lan", "ip": "192.168.10.1"},
                {"segment": "dmz", "ip": "192.168.20.1"}
            ],
            "snapshots": [{"name": "basic-config", "description": "WAN/LAN configured", "default": true}],
            "startOnCreate": true
        },
        {
            "name": "lan-client",
            "template": "rocky-10-lab-template",
            "resources": {"cpu": 1, "memory": 2048},
            "networks": [{"segment": "lan", "ip": "192.168.10.10"}],
            "snapshots": [{"name": "clean", "default": true}],
            "startOnCreate": true
        },
        {
            "name": "web-server",
            "template": "ubuntu-lab-template",
            "resources": {"cpu": 1, "memory": 1024},
            "networks": [{"segment": "dmz", "ip": "192.168.20.10"}],
            "snapshots": [{"name": "clean", "default": true}],
            "startOnCreate": true
        }
    ]
}'::jsonb,
    updated_at = NOW()
WHERE name = 'pfSense Firewall Fundamentals';

-- Update Blue Team Incident Response lab
-- NOTE: windows-server-2022-ad template still needs to be created on Proxmox
UPDATE lab_templates
SET spec = '{
    "platform": "proxmox",
    "network": {
        "segments": [
            {"name": "corporate", "vlan": 100, "subnet": "192.168.1.0/24", "gateway": "192.168.1.1", "dhcp": false},
            {"name": "dmz", "vlan": 101, "subnet": "192.168.2.0/24", "gateway": "192.168.2.1", "dhcp": false}
        ]
    },
    "vms": [
        {
            "name": "analyst-workstation",
            "template": "rocky-10-lab-template",
            "resources": {"cpu": 2, "memory": 4096, "disk": 40},
            "networks": [{"segment": "corporate", "ip": "192.168.1.10"}],
            "snapshots": [{"name": "clean", "description": "Fresh workstation", "default": true}],
            "startOnCreate": true
        },
        {
            "name": "web-server",
            "template": "ubuntu-lab-template",
            "resources": {"cpu": 2, "memory": 2048, "disk": 20},
            "networks": [{"segment": "dmz", "ip": "192.168.2.10"}],
            "snapshots": [{"name": "compromised", "description": "Server with webshell planted", "default": true}],
            "startOnCreate": true
        },
        {
            "name": "dc01",
            "template": "windows-server-2022-ad",
            "resources": {"cpu": 4, "memory": 8192, "disk": 60},
            "networks": [{"segment": "corporate", "ip": "192.168.1.1"}],
            "snapshots": [{"name": "post-lateral-movement", "description": "After attacker lateral movement", "default": true}],
            "startOnCreate": true
        }
    ]
}'::jsonb,
    updated_at = NOW()
WHERE name = 'Blue Team Incident Response';

-- Update CloudStack IaaS Fundamentals lab
UPDATE lab_templates
SET spec = '{
    "platform": "cloudstack",
    "network": {
        "segments": [
            {"name": "isolated-network", "vlan": 300, "subnet": "10.1.1.0/24", "gateway": "10.1.1.1", "dhcp": true}
        ]
    },
    "vms": [
        {
            "name": "jumpbox",
            "template": "rocky-10-lab-template",
            "resources": {"cpu": 1, "memory": 1024},
            "networks": [{"segment": "isolated-network"}],
            "snapshots": [{"name": "baseline", "description": "Jumpbox with CloudMonkey CLI", "default": true}],
            "startOnCreate": true
        }
    ]
}'::jsonb,
    updated_at = NOW()
WHERE name = 'CloudStack IaaS Fundamentals';

-- Update network-security-intro lab (if it exists with old templates)
UPDATE lab_templates
SET spec = '{
    "platform": "proxmox",
    "network": {
        "segments": [
            {"name": "lab-network", "vlan": 100, "subnet": "10.10.100.0/24", "gateway": "10.10.100.1", "dhcp": false}
        ]
    },
    "vms": [
        {
            "name": "attacker",
            "template": "ubuntu-lab-template",
            "resources": {"cpu": 2, "memory": 4096, "disk": 32},
            "networks": [{"segment": "lab-network", "ip": "10.10.100.50"}],
            "snapshots": [{"name": "initial", "default": true}],
            "startOnCreate": true
        },
        {
            "name": "target",
            "template": "ubuntu-lab-wazuh-template",
            "wazuhAgent": true,
            "resources": {"cpu": 1, "memory": 2048, "disk": 16},
            "networks": [{"segment": "lab-network", "ip": "10.10.100.100"}],
            "snapshots": [{"name": "initial", "default": true}],
            "startOnCreate": true
        }
    ]
}'::jsonb,
    updated_at = NOW()
WHERE slug = 'network-security-intro' AND spec::text != '{}';

-- +goose Down
-- Note: Down migration would restore the original template names but those templates
-- don't exist, so we leave spec unchanged on rollback
