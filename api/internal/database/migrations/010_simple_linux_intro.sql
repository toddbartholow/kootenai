-- Migration 010: Simple Linux Introduction Lab Template
-- A beginner-friendly lab for testing the end-to-end workflow

INSERT INTO lab_templates (
    name, description, version, platform, duration_minutes, difficulty,
    max_points, pass_threshold, spec, checkpoints, is_active
)
VALUES (
    'Simple Linux Introduction',
    'A beginner-friendly lab to learn basic Linux commands and file operations. Perfect for testing the lab completion and grading system.',
    '1.0.0',
    'proxmox',
    30,
    'beginner',
    100,
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
                "name": "linux-vm",
                "template": "ubuntu-lab-wazuh-template",
                "templateVmId": 9003,
                "wazuhAgent": true,
                "agentConfig": {
                    "monitor_paths": ["/home/student", "/var/log"],
                    "audit_commands": true,
                    "realtime": true
                },
                "resources": {"cpu": 1, "memory": 2048, "disk": 16},
                "networks": [{"segment": "lab-network", "ip": "10.10.100.10"}],
                "snapshots": [{"name": "initial", "description": "Fresh Ubuntu installation", "default": true}]
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
            "id": "create-directory",
            "description": "Create a directory called mywork in your home directory",
            "points": 20,
            "order": 1,
            "hint": "Use mkdir ~/mywork to create the directory",
            "triggers": [{"type": "file_exists", "target": "linux-vm", "match": {"path": "/home/student/mywork"}}]
        },
        {
            "id": "create-file",
            "description": "Create a file called notes.txt in the mywork directory",
            "points": 20,
            "order": 2,
            "depends_on": ["create-directory"],
            "hint": "Use touch ~/mywork/notes.txt to create the file",
            "triggers": [{"type": "file_exists", "target": "linux-vm", "match": {"path": "/home/student/mywork/notes.txt"}}]
        },
        {
            "id": "write-content",
            "description": "Write Hello, Kootenai! into notes.txt",
            "points": 30,
            "order": 3,
            "depends_on": ["create-file"],
            "hint": "Use echo \"Hello, Kootenai!\" > ~/mywork/notes.txt",
            "triggers": [{"type": "file_content", "target": "linux-vm", "match": {"path": "/home/student/mywork/notes.txt", "contains": "Hello, Kootenai!"}}]
        },
        {
            "id": "check-disk-space",
            "description": "Check available disk space",
            "points": 10,
            "order": 4,
            "hint": "Run the df -h command",
            "triggers": [{"type": "command_executed", "target": "linux-vm", "match": {"pattern": "df"}}]
        },
        {
            "id": "list-files",
            "description": "List all files in your mywork directory",
            "points": 20,
            "order": 5,
            "depends_on": ["create-file"],
            "hint": "Use ls ~/mywork to list the files",
            "triggers": [{"type": "command_executed", "target": "linux-vm", "match": {"pattern": "ls.*mywork"}}]
        }
    ]'::jsonb,
    true
)
ON CONFLICT (name) DO UPDATE SET
    description = EXCLUDED.description,
    version = EXCLUDED.version,
    duration_minutes = EXCLUDED.duration_minutes,
    max_points = EXCLUDED.max_points,
    pass_threshold = EXCLUDED.pass_threshold,
    spec = EXCLUDED.spec,
    checkpoints = EXCLUDED.checkpoints,
    updated_at = NOW();
