-- Kootenai Platform Demo Seed Data
-- Sample data for local development and demonstrations

-- -----------------------------------------------------------------------------
-- Demo Users
-- -----------------------------------------------------------------------------

-- Instructor user
INSERT INTO users (id, external_id, username, email, display_name, role)
VALUES (
    '11111111-1111-1111-1111-111111111111',
    'instructor1',
    'instructor',
    'instructor@irqstudio.com',
    'Demo Instructor',
    'instructor'
);

-- Student users
INSERT INTO users (id, external_id, username, email, display_name, role)
VALUES
    (
        '22222222-2222-2222-2222-222222222222',
        'student1',
        'alice',
        'alice@student.irqstudio.com',
        'Alice Student',
        'student'
    ),
    (
        '33333333-3333-3333-3333-333333333333',
        'student2',
        'bob',
        'bob@student.irqstudio.com',
        'Bob Student',
        'student'
    );

-- -----------------------------------------------------------------------------
-- Demo Lab Templates
-- -----------------------------------------------------------------------------

-- Linux Fundamentals Lab
INSERT INTO lab_templates (
    id, name, description, version, platform, duration_minutes,
    difficulty, max_points, pass_threshold, spec, checkpoints, is_active
)
VALUES (
    'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
    'linux-fundamentals',
    'Introduction to Linux system administration covering file management, permissions, and user administration',
    '1.0.0',
    'proxmox',
    120,
    'beginner',
    100,
    70,
    '{
        "platform": "proxmox",
        "network": {
            "segments": [
                {
                    "name": "lab-net",
                    "vlan": 100,
                    "subnet": "10.100.0.0/24",
                    "gateway": "10.100.0.1",
                    "dhcp": true
                }
            ]
        },
        "vms": [
            {
                "name": "workstation",
                "template": "ubuntu-22.04-base",
                "resources": {"cpu": 2, "memory": 2048, "disk": 20},
                "networks": [{"segment": "lab-net", "ip": "10.100.0.10"}],
                "startOnCreate": true,
                "wazuhAgent": true,
                "agentConfig": {
                    "monitor_paths": ["/home", "/etc", "/var/log"],
                    "audit_commands": true,
                    "realtime": true
                }
            }
        ],
        "checkpoints": {
            "enabled": true,
            "pass_threshold": 70,
            "allow_retry": true,
            "show_hints": true,
            "realtime_update": true
        },
        "objectives": []
    }'::jsonb,
    '[
        {
            "id": "create-user",
            "description": "Create a new user account named labuser",
            "points": 20,
            "hint": "Use the useradd or adduser command",
            "order": 1,
            "triggers": [
                {
                    "type": "user_created",
                    "target": "workstation",
                    "match": {"username": "labuser"}
                }
            ]
        },
        {
            "id": "create-directory",
            "description": "Create directory /home/labuser/projects with correct permissions",
            "points": 20,
            "hint": "Use mkdir and chmod commands",
            "order": 2,
            "depends_on": ["create-user"],
            "triggers": [
                {
                    "type": "file_exists",
                    "target": "workstation",
                    "match": {"path": "/home/labuser/projects", "mode": "drwxr-xr-x"}
                }
            ]
        },
        {
            "id": "set-ownership",
            "description": "Set labuser as owner of the projects directory",
            "points": 20,
            "hint": "Use the chown command",
            "order": 3,
            "depends_on": ["create-directory"],
            "triggers": [
                {
                    "type": "permission_changed",
                    "target": "workstation",
                    "match": {"path": "/home/labuser/projects", "owner": "labuser"}
                }
            ]
        },
        {
            "id": "install-nginx",
            "description": "Install the nginx web server package",
            "points": 20,
            "hint": "Use apt install nginx",
            "order": 4,
            "triggers": [
                {
                    "type": "package",
                    "target": "workstation",
                    "match": {"package": "nginx", "state": "installed"}
                }
            ]
        },
        {
            "id": "start-nginx",
            "description": "Start and enable the nginx service",
            "points": 20,
            "hint": "Use systemctl start and systemctl enable",
            "order": 5,
            "depends_on": ["install-nginx"],
            "triggers": [
                {
                    "type": "service",
                    "target": "workstation",
                    "match": {"name": "nginx", "state": "active"}
                }
            ]
        }
    ]'::jsonb,
    true
);

-- Security Basics Lab
INSERT INTO lab_templates (
    id, name, description, version, platform, duration_minutes,
    difficulty, max_points, pass_threshold, spec, checkpoints, is_active
)
VALUES (
    'bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb',
    'security-basics',
    'Introduction to security monitoring and log analysis using Wazuh',
    '1.0.0',
    'proxmox',
    90,
    'intermediate',
    100,
    70,
    '{
        "platform": "proxmox",
        "network": {
            "segments": [
                {
                    "name": "dmz",
                    "vlan": 200,
                    "subnet": "10.200.0.0/24",
                    "gateway": "10.200.0.1"
                },
                {
                    "name": "internal",
                    "vlan": 201,
                    "subnet": "10.201.0.0/24",
                    "gateway": "10.201.0.1"
                }
            ]
        },
        "vms": [
            {
                "name": "webserver",
                "template": "ubuntu-22.04-lamp",
                "resources": {"cpu": 2, "memory": 2048, "disk": 20},
                "networks": [{"segment": "dmz", "ip": "10.200.0.10"}],
                "startOnCreate": true,
                "wazuhAgent": true,
                "agentConfig": {
                    "monitor_paths": ["/var/www", "/etc/apache2", "/var/log/apache2"],
                    "audit_commands": true,
                    "realtime": true
                }
            },
            {
                "name": "attacker",
                "template": "kali-2024",
                "resources": {"cpu": 2, "memory": 4096, "disk": 40},
                "networks": [{"segment": "dmz", "ip": "10.200.0.50"}],
                "startOnCreate": true,
                "wazuhAgent": false
            }
        ],
        "checkpoints": {
            "enabled": true,
            "pass_threshold": 70,
            "allow_retry": true,
            "show_hints": true,
            "realtime_update": true
        }
    }'::jsonb,
    '[
        {
            "id": "configure-firewall",
            "description": "Configure UFW to allow only HTTP and SSH traffic",
            "points": 25,
            "hint": "Use ufw commands to enable and configure rules",
            "order": 1,
            "triggers": [
                {
                    "type": "command_executed",
                    "target": "webserver",
                    "match": {"pattern": "ufw enable"}
                }
            ]
        },
        {
            "id": "detect-scan",
            "description": "Detect the port scan from the attacker machine",
            "points": 25,
            "hint": "Check Wazuh alerts or /var/log/auth.log",
            "order": 2,
            "triggers": [
                {
                    "type": "custom",
                    "target": "webserver",
                    "match": {}
                }
            ]
        },
        {
            "id": "harden-ssh",
            "description": "Disable root SSH login in sshd_config",
            "points": 25,
            "hint": "Edit /etc/ssh/sshd_config",
            "order": 3,
            "triggers": [
                {
                    "type": "file_content",
                    "target": "webserver",
                    "match": {
                        "path": "/etc/ssh/sshd_config",
                        "contains": "PermitRootLogin no"
                    }
                }
            ]
        },
        {
            "id": "restart-ssh",
            "description": "Restart SSH service to apply changes",
            "points": 25,
            "hint": "Use systemctl restart sshd",
            "order": 4,
            "depends_on": ["harden-ssh"],
            "triggers": [
                {
                    "type": "service",
                    "target": "webserver",
                    "match": {"name": "sshd", "state": "active"}
                }
            ]
        }
    ]'::jsonb,
    true
);

-- Network Troubleshooting Lab
INSERT INTO lab_templates (
    id, name, description, version, platform, duration_minutes,
    difficulty, max_points, pass_threshold, spec, checkpoints, is_active
)
VALUES (
    'cccccccc-cccc-cccc-cccc-cccccccccccc',
    'network-troubleshooting',
    'Practice diagnosing and fixing common network connectivity issues',
    '1.0.0',
    'proxmox',
    60,
    'intermediate',
    100,
    60,
    '{
        "platform": "proxmox",
        "network": {
            "segments": [
                {
                    "name": "client-net",
                    "vlan": 300,
                    "subnet": "192.168.1.0/24",
                    "gateway": "192.168.1.1"
                },
                {
                    "name": "server-net",
                    "vlan": 301,
                    "subnet": "192.168.2.0/24",
                    "gateway": "192.168.2.1"
                }
            ]
        },
        "vms": [
            {
                "name": "client",
                "template": "ubuntu-22.04-desktop",
                "resources": {"cpu": 2, "memory": 2048, "disk": 20},
                "networks": [{"segment": "client-net"}],
                "startOnCreate": true,
                "wazuhAgent": true
            },
            {
                "name": "router",
                "template": "vyos-1.4",
                "resources": {"cpu": 1, "memory": 512, "disk": 4},
                "networks": [
                    {"segment": "client-net", "ip": "192.168.1.1"},
                    {"segment": "server-net", "ip": "192.168.2.1"}
                ],
                "startOnCreate": true,
                "wazuhAgent": false
            },
            {
                "name": "server",
                "template": "ubuntu-22.04-server",
                "resources": {"cpu": 2, "memory": 2048, "disk": 20},
                "networks": [{"segment": "server-net", "ip": "192.168.2.10"}],
                "startOnCreate": true,
                "wazuhAgent": true
            }
        ],
        "checkpoints": {
            "enabled": true,
            "pass_threshold": 60,
            "show_hints": true
        }
    }'::jsonb,
    '[
        {
            "id": "fix-client-ip",
            "description": "Configure correct IP address on the client machine",
            "points": 25,
            "hint": "Check /etc/netplan/ or use nmcli",
            "order": 1,
            "triggers": [
                {
                    "type": "network_connection",
                    "target": "client",
                    "match": {"destination": "192.168.1.1"}
                }
            ]
        },
        {
            "id": "fix-routing",
            "description": "Add default route on the client",
            "points": 25,
            "hint": "Use ip route add default",
            "order": 2,
            "triggers": [
                {
                    "type": "command_executed",
                    "target": "client",
                    "match": {"pattern": "ip route add default"}
                }
            ]
        },
        {
            "id": "verify-connectivity",
            "description": "Verify connectivity to the server",
            "points": 25,
            "hint": "Use ping to test connectivity",
            "order": 3,
            "triggers": [
                {
                    "type": "network_connection",
                    "target": "client",
                    "match": {"destination": "192.168.2.10", "protocol": "icmp"}
                }
            ]
        },
        {
            "id": "document-solution",
            "description": "Create a file documenting the solution",
            "points": 25,
            "hint": "Create /root/solution.txt with your findings",
            "order": 4,
            "triggers": [
                {
                    "type": "file_exists",
                    "target": "client",
                    "match": {"path": "/root/solution.txt"}
                }
            ]
        }
    ]'::jsonb,
    true
);

-- -----------------------------------------------------------------------------
-- Demo Pod (Active Lab Instance)
-- -----------------------------------------------------------------------------

-- Create a running pod for alice
INSERT INTO pods (
    id, lab_template_id, owner_id, platform, status, vms, networks, expires_at
)
VALUES (
    'dddddddd-dddd-dddd-dddd-dddddddddddd',
    'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
    '22222222-2222-2222-2222-222222222222',
    'proxmox',
    'running',
    '[
        {
            "name": "workstation",
            "platformId": "100",
            "platform": "proxmox",
            "node": "pve",
            "status": "running",
            "ipAddress": "10.100.0.10",
            "currentSnapshot": "clean"
        }
    ]'::jsonb,
    '[
        {
            "name": "lab-net",
            "platformId": "vmbr100",
            "vlan": 100,
            "subnet": "10.100.0.0/24"
        }
    ]'::jsonb,
    NOW() + INTERVAL '4 hours'
);

-- -----------------------------------------------------------------------------
-- Demo Lab Session
-- -----------------------------------------------------------------------------

-- Create an active session for alice
INSERT INTO lab_sessions (
    id, pod_id, user_id, lab_template_id,
    canvas_course_id, canvas_assignment_id,
    max_points, earned_points, percentage, passed
)
VALUES (
    'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee',
    'dddddddd-dddd-dddd-dddd-dddddddddddd',
    '22222222-2222-2222-2222-222222222222',
    'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
    'CYBER101',
    'lab-linux-fundamentals',
    100,
    40,
    40.00,
    false
);

-- -----------------------------------------------------------------------------
-- Demo Checkpoint Progress
-- -----------------------------------------------------------------------------

-- Alice has completed first two checkpoints
INSERT INTO checkpoint_progress (
    id, session_id, checkpoint_id, status, points, earned_points, passed_at, attempt_count
)
VALUES
    (
        'f1111111-1111-1111-1111-111111111111',
        'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee',
        'create-user',
        'passed',
        20,
        20,
        NOW() - INTERVAL '30 minutes',
        1
    ),
    (
        'f2222222-2222-2222-2222-222222222222',
        'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee',
        'create-directory',
        'passed',
        20,
        20,
        NOW() - INTERVAL '25 minutes',
        2
    ),
    (
        'f3333333-3333-3333-3333-333333333333',
        'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee',
        'set-ownership',
        'pending',
        20,
        0,
        NULL,
        0
    ),
    (
        'f4444444-4444-4444-4444-444444444444',
        'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee',
        'install-nginx',
        'pending',
        20,
        0,
        NULL,
        0
    ),
    (
        'f5555555-5555-5555-5555-555555555555',
        'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee',
        'start-nginx',
        'pending',
        20,
        0,
        NULL,
        0
    );

-- -----------------------------------------------------------------------------
-- Demo Wazuh Agents
-- -----------------------------------------------------------------------------

INSERT INTO wazuh_agents (
    id, agent_id, agent_name, pod_id, vm_name, ip_address, os_name, os_version, status
)
VALUES (
    'a1111111-1111-1111-1111-111111111111',
    '001',
    'workstation-dddddddd',
    'dddddddd-dddd-dddd-dddd-dddddddddddd',
    'workstation',
    '10.100.0.10',
    'Ubuntu',
    '22.04 LTS',
    'active'
);

-- -----------------------------------------------------------------------------
-- Demo Events (Sample Wazuh Events)
-- -----------------------------------------------------------------------------

INSERT INTO events (
    timestamp, pod_id, session_id, vm_name, agent_id,
    event_type, rule_id, rule_level, description, location, data, processed, matched_checkpoints
)
VALUES
    (
        NOW() - INTERVAL '30 minutes',
        'dddddddd-dddd-dddd-dddd-dddddddddddd',
        'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee',
        'workstation',
        '001',
        'user_created',
        5901,
        3,
        'New user added: labuser',
        '/var/log/auth.log',
        '{"user": {"username": "labuser", "uid": 1001, "gid": 1001, "home": "/home/labuser", "shell": "/bin/bash"}}'::jsonb,
        true,
        ARRAY['create-user']
    ),
    (
        NOW() - INTERVAL '25 minutes',
        'dddddddd-dddd-dddd-dddd-dddddddddddd',
        'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee',
        'workstation',
        '001',
        'file_created',
        550,
        3,
        'File created: /home/labuser/projects',
        '/home/labuser/projects',
        '{"syscheck": {"path": "/home/labuser/projects", "event": "added", "mode": "drwxr-xr-x", "uid": "1001", "gid": "1001", "owner": "labuser", "group": "labuser"}}'::jsonb,
        true,
        ARRAY['create-directory']
    );

-- -----------------------------------------------------------------------------
-- Demo Assessment Result
-- -----------------------------------------------------------------------------

INSERT INTO assessment_results (
    id, session_id, score, max_score, percentage, item_count, passed_count,
    status, components, devices
)
VALUES (
    'a1111111-2222-3333-4444-555555555555',
    'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee',
    40,
    100,
    40.00,
    5,
    2,
    'in_progress',
    '[
        {"name": "User Management", "score": 20, "maxScore": 20, "items": [{"description": "Create labuser account", "correct": true, "points": 20}]},
        {"name": "File Management", "score": 20, "maxScore": 40, "items": [{"description": "Create projects directory", "correct": true, "points": 20}, {"description": "Set correct ownership", "correct": false, "points": 0}]},
        {"name": "Service Management", "score": 0, "maxScore": 40, "items": [{"description": "Install nginx", "correct": false, "points": 0}, {"description": "Start nginx service", "correct": false, "points": 0}]}
    ]'::jsonb,
    '[
        {"name": "workstation", "type": "Ubuntu VM", "score": 40, "maxScore": 100, "componentResults": []}
    ]'::jsonb
);

RAISE NOTICE 'Demo seed data loaded successfully';
