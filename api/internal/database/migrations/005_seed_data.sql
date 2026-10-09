-- Migration 005: Seed Data
-- Demo users and lab templates for development/testing
-- Uses ON CONFLICT DO NOTHING to be idempotent

-- -----------------------------------------------------------------------------
-- Demo Users
-- -----------------------------------------------------------------------------
INSERT INTO users (id, external_id, username, email, display_name, role, is_active)
VALUES
    ('11111111-1111-1111-1111-111111111111', 'admin', 'admin', 'admin@kootenai.local', 'Lab Administrator', 'admin', true),
    ('22222222-2222-2222-2222-222222222222', 'instructor1', 'instructor1', 'instructor1@kootenai.local', 'Demo Instructor', 'instructor', true),
    ('33333333-3333-3333-3333-333333333333', 'student1', 'student1', 'student1@kootenai.local', 'Alice Student', 'student', true),
    ('44444444-4444-4444-4444-444444444444', 'student2', 'student2', 'student2@kootenai.local', 'Bob Student', 'student', true),
    ('55555555-5555-5555-5555-555555555555', 'student3', 'student3', 'student3@kootenai.local', 'Carol Student', 'student', true)
ON CONFLICT (external_id) DO NOTHING;

-- -----------------------------------------------------------------------------
-- Lab Template: Linux Firewall Basics
-- -----------------------------------------------------------------------------
INSERT INTO lab_templates (
    name, description, version, platform, duration_minutes, difficulty,
    max_points, pass_threshold, spec, checkpoints, is_active
)
VALUES (
    'Linux Firewall Basics',
    'Learn to configure UFW (Uncomplicated Firewall) on Ubuntu Linux. Students will install, configure, and test basic firewall rules to secure a Linux system.',
    '1.0.0',
    'proxmox',
    90,
    'beginner',
    110,
    70,
    '{
        "platform": "proxmox",
        "network": {
            "segments": [
                {"name": "lab-network", "vlan": 100, "subnet": "10.10.100.0/24", "gateway": "10.10.100.1", "dhcp": false}
            ]
        },
        "vms": [
            {
                "name": "workstation",
                "template": "ubuntu-22.04-desktop",
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
                "template": "ubuntu-22.04-server",
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
    '[
        {"id": "install-ufw", "description": "Install the UFW firewall package", "points": 10, "order": 1, "hint": "Use sudo apt install ufw"},
        {"id": "check-ufw-status", "description": "Check the current UFW status", "points": 5, "order": 2, "depends_on": ["install-ufw"], "hint": "Run sudo ufw status"},
        {"id": "enable-ufw", "description": "Enable the UFW firewall", "points": 15, "order": 3, "depends_on": ["install-ufw"], "hint": "Run sudo ufw enable"},
        {"id": "allow-ssh", "description": "Allow SSH connections (port 22)", "points": 15, "order": 4, "depends_on": ["enable-ufw"], "hint": "Use sudo ufw allow ssh"},
        {"id": "allow-http", "description": "Allow HTTP traffic (port 80)", "points": 10, "order": 5, "depends_on": ["enable-ufw"], "hint": "Use sudo ufw allow http"},
        {"id": "allow-https", "description": "Allow HTTPS traffic (port 443)", "points": 10, "order": 6, "depends_on": ["enable-ufw"], "hint": "Use sudo ufw allow https"},
        {"id": "deny-telnet", "description": "Explicitly deny Telnet (port 23)", "points": 15, "order": 7, "depends_on": ["enable-ufw"], "hint": "Use sudo ufw deny 23/tcp"},
        {"id": "limit-ssh", "description": "Enable rate limiting for SSH to prevent brute force attacks", "points": 20, "order": 8, "depends_on": ["allow-ssh"], "hint": "Use sudo ufw limit ssh"},
        {"id": "verify-connectivity", "description": "Verify you can still connect to the target server", "points": 10, "order": 9, "depends_on": ["allow-http"], "hint": "Try ping or curl to 10.10.100.20"}
    ]'::jsonb,
    true
)
ON CONFLICT (name) DO UPDATE SET
    description = EXCLUDED.description,
    version = EXCLUDED.version,
    duration_minutes = EXCLUDED.duration_minutes,
    max_points = EXCLUDED.max_points,
    spec = EXCLUDED.spec,
    checkpoints = EXCLUDED.checkpoints,
    updated_at = NOW();

-- -----------------------------------------------------------------------------
-- Lab Template: Linux Routing Fundamentals
-- -----------------------------------------------------------------------------
INSERT INTO lab_templates (
    name, description, version, platform, duration_minutes, difficulty,
    max_points, pass_threshold, spec, checkpoints, is_active
)
VALUES (
    'Linux Routing Fundamentals',
    'Learn basic IP routing concepts on Linux. Configure static routes, enable IP forwarding, and understand routing tables.',
    '1.0.0',
    'proxmox',
    60,
    'intermediate',
    100,
    75,
    '{
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
                "template": "ubuntu-22.04-server-minimal",
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
                "template": "ubuntu-22.04-desktop",
                "wazuhAgent": true,
                "agentConfig": {"monitor_paths": ["/etc/netplan"], "audit_commands": true, "realtime": true},
                "resources": {"cpu": 1, "memory": 2048, "disk": 16},
                "networks": [{"segment": "network-a", "ip": "10.10.101.10"}],
                "snapshots": [{"name": "initial", "description": "Client on Network A", "default": true}]
            },
            {
                "name": "client-b",
                "template": "ubuntu-22.04-desktop",
                "wazuhAgent": true,
                "agentConfig": {"monitor_paths": ["/etc/netplan"], "audit_commands": true, "realtime": true},
                "resources": {"cpu": 1, "memory": 2048, "disk": 16},
                "networks": [{"segment": "network-b", "ip": "10.10.102.10"}],
                "snapshots": [{"name": "initial", "description": "Client on Network B", "default": true}]
            }
        ]
    }'::jsonb,
    '[
        {"id": "view-routing-table", "description": "View the current routing table on the router", "points": 5, "order": 1, "hint": "Use ip route or route -n"},
        {"id": "enable-ip-forwarding", "description": "Enable IP forwarding on the router", "points": 20, "order": 2, "hint": "Edit /etc/sysctl.conf and set net.ipv4.ip_forward=1"},
        {"id": "set-gateway-client-a", "description": "Configure default gateway on Client A to use the router", "points": 15, "order": 3, "depends_on": ["enable-ip-forwarding"], "hint": "Use ip route add default via 10.10.101.1"},
        {"id": "set-gateway-client-b", "description": "Configure default gateway on Client B to use the router", "points": 15, "order": 4, "depends_on": ["enable-ip-forwarding"], "hint": "Use ip route add default via 10.10.102.1"},
        {"id": "ping-router-from-a", "description": "Ping the router Network B interface from Client A", "points": 10, "order": 5, "depends_on": ["set-gateway-client-a"], "hint": "Run ping 10.10.102.1"},
        {"id": "ping-client-b-from-a", "description": "Successfully ping Client B from Client A", "points": 20, "order": 6, "depends_on": ["set-gateway-client-a", "set-gateway-client-b"], "required": true, "hint": "Run ping 10.10.102.10"},
        {"id": "traceroute", "description": "Run traceroute from Client A to Client B to verify path", "points": 15, "order": 7, "depends_on": ["ping-client-b-from-a"], "hint": "Use traceroute 10.10.102.10"}
    ]'::jsonb,
    true
)
ON CONFLICT (name) DO UPDATE SET
    description = EXCLUDED.description,
    version = EXCLUDED.version,
    duration_minutes = EXCLUDED.duration_minutes,
    max_points = EXCLUDED.max_points,
    spec = EXCLUDED.spec,
    checkpoints = EXCLUDED.checkpoints,
    updated_at = NOW();

-- -----------------------------------------------------------------------------
-- Lab Template: pfSense Firewall Fundamentals
-- -----------------------------------------------------------------------------
INSERT INTO lab_templates (
    name, description, version, platform, duration_minutes, difficulty,
    max_points, pass_threshold, spec, checkpoints, is_active
)
VALUES (
    'pfSense Firewall Fundamentals',
    'Learn pfSense firewall configuration including NAT, firewall rules, and VLANs.',
    '1.0.0',
    'proxmox',
    90,
    'beginner',
    0,
    70,
    '{
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
                "template": "rocky9-desktop-minimal",
                "resources": {"cpu": 1, "memory": 2048},
                "networks": [{"segment": "lan", "ip": "192.168.10.10"}],
                "snapshots": [{"name": "clean", "default": true}],
                "startOnCreate": true
            },
            {
                "name": "web-server",
                "template": "ubuntu22-server-nginx",
                "resources": {"cpu": 1, "memory": 1024},
                "networks": [{"segment": "dmz", "ip": "192.168.20.10"}],
                "snapshots": [{"name": "clean", "default": true}],
                "startOnCreate": true
            }
        ]
    }'::jsonb,
    '[]'::jsonb,
    true
)
ON CONFLICT (name) DO UPDATE SET
    description = EXCLUDED.description,
    version = EXCLUDED.version,
    duration_minutes = EXCLUDED.duration_minutes,
    spec = EXCLUDED.spec,
    updated_at = NOW();

-- -----------------------------------------------------------------------------
-- Lab Template: Blue Team Incident Response
-- -----------------------------------------------------------------------------
INSERT INTO lab_templates (
    name, description, version, platform, duration_minutes, difficulty,
    max_points, pass_threshold, spec, checkpoints, is_active
)
VALUES (
    'Blue Team Incident Response',
    'Investigate a compromised network with pre-planted artifacts. Practice digital forensics and incident response techniques.',
    '1.0.0',
    'proxmox',
    120,
    'intermediate',
    0,
    70,
    '{
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
                "template": "rocky9-desktop-soc",
                "resources": {"cpu": 2, "memory": 4096, "disk": 40},
                "networks": [{"segment": "corporate", "ip": "192.168.1.10"}],
                "snapshots": [{"name": "clean", "description": "Fresh workstation", "default": true}],
                "startOnCreate": true
            },
            {
                "name": "web-server",
                "template": "ubuntu22-server-lamp",
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
    '[]'::jsonb,
    true
)
ON CONFLICT (name) DO UPDATE SET
    description = EXCLUDED.description,
    version = EXCLUDED.version,
    duration_minutes = EXCLUDED.duration_minutes,
    spec = EXCLUDED.spec,
    updated_at = NOW();

-- -----------------------------------------------------------------------------
-- Lab Template: CloudStack IaaS Fundamentals
-- -----------------------------------------------------------------------------
INSERT INTO lab_templates (
    name, description, version, platform, duration_minutes, difficulty,
    max_points, pass_threshold, spec, checkpoints, is_active
)
VALUES (
    'CloudStack IaaS Fundamentals',
    'Learn IaaS concepts using Apache CloudStack. Create virtual machines, networks, and storage using the CloudStack API.',
    '1.0.0',
    'cloudstack',
    120,
    'intermediate',
    0,
    70,
    '{
        "platform": "cloudstack",
        "network": {
            "segments": [
                {"name": "isolated-network", "vlan": 300, "subnet": "10.1.1.0/24", "gateway": "10.1.1.1", "dhcp": true}
            ]
        },
        "vms": [
            {
                "name": "jumpbox",
                "template": "rocky9-minimal-cloud",
                "resources": {"cpu": 1, "memory": 1024},
                "networks": [{"segment": "isolated-network"}],
                "snapshots": [{"name": "baseline", "description": "Jumpbox with CloudMonkey CLI", "default": true}],
                "startOnCreate": true
            }
        ]
    }'::jsonb,
    '[]'::jsonb,
    true
)
ON CONFLICT (name) DO UPDATE SET
    description = EXCLUDED.description,
    version = EXCLUDED.version,
    duration_minutes = EXCLUDED.duration_minutes,
    spec = EXCLUDED.spec,
    updated_at = NOW();
