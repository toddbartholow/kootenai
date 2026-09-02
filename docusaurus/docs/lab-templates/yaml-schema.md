---
title: Lab Template YAML Schema
description: Complete specification for lab template YAML files
tags:
  - templates
  - schema
  - yaml
---

# Lab Template YAML Schema

This document defines the complete schema for lab template YAML files.

## Overview

Lab templates are YAML files that define virtual lab environments. They specify:

- Virtual machines and their configuration
- Network topology
- Checkpoints (objectives) for grading
- Instructions and metadata

## Template Location

Templates are stored in `templates/` directory, organized by category:

```
templates/
├── cybersecurity/
│   ├── linux-security-basics.yaml
│   └── incident-response.yaml
├── networking/
│   ├── vlan-basics.yaml
│   └── routing-fundamentals.yaml
├── linux-pathway/
│   ├── 01-shell-basics.yaml
│   └── 02-file-management.yaml
└── examples/
    └── minimal-template.yaml
```

## Basic Structure

```yaml
apiVersion: v1
kind: LabTemplate

metadata:
  name: linux-foundations
  version: "1.0.0"
  description: Introduction to Linux command line
  difficulty: beginner
  estimatedMinutes: 60
  tags:
    - linux
    - command-line
    - beginner

spec:
  platform: proxmox

  vms:
    - name: student-vm
      template: ubuntu-22.04
      # ... VM configuration

  networks:
    - name: lab-network
      # ... Network configuration

  checkpoints:
    - id: create-user
      name: Create a user account
      # ... Checkpoint configuration

  instructions:
    overview: |
      In this lab, you will learn...
    # ... Instructions
```

## Schema Reference

### Root Level

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `apiVersion` | string | Yes | Always `v1` |
| `kind` | string | Yes | Always `LabTemplate` |
| `metadata` | object | Yes | Template metadata |
| `spec` | object | Yes | Template specification |

### Metadata

```yaml
metadata:
  name: unique-template-name      # Required: URL-safe identifier
  version: "1.0.0"                # Required: Semantic version
  description: "Description..."   # Required: Short description
  difficulty: beginner            # Required: beginner|intermediate|advanced
  estimatedMinutes: 60            # Required: Estimated completion time
  maxPoints: 100                  # Optional: Total points (default: sum of checkpoints)
  passThreshold: 70               # Optional: Passing score % (default: 70)
  tags:                           # Optional: Searchable tags
    - linux
    - networking
  category: cybersecurity         # Optional: Category for organization
  author: "John Doe"              # Optional: Template author
  prerequisites:                  # Optional: Required prior knowledge
    - "Basic computer skills"
```

### Spec: Platform

```yaml
spec:
  platform: proxmox               # Required: proxmox|cloudstack|any
  platformOptions:                # Optional: Platform-specific options
    proxmox:
      storage: local-lvm
      pool: lab-pool
    cloudstack:
      zone: zone-1
      serviceOffering: small
```

### Spec: VMs

```yaml
spec:
  vms:
    - name: student-vm            # Required: Unique name within template
      displayName: "Ubuntu Server"# Optional: Human-readable name
      template: ubuntu-22.04      # Required: Base VM template
      description: "Main lab VM"  # Optional: Description

      # Resources
      cpu: 2                      # Optional: CPU cores (default: from template)
      memory: 2048                # Optional: RAM in MB (default: from template)
      disk: 32                    # Optional: Disk in GB (default: from template)

      # Network
      networks:                   # Required: Network connections
        - network: lab-network
          ip: "10.0.100.10"       # Optional: Static IP
          dhcp: false             # Optional: Use DHCP (default: false)

      # Startup
      startOrder: 1               # Optional: Boot order (default: 1)
      startDelay: 0               # Optional: Delay in seconds
      autoStart: true             # Optional: Start with pod (default: true)

      # Access
      credentials:                # Optional: Default credentials
        username: labuser
        password: labpass123
      sshKey: false               # Optional: SSH key injection

      # Wazuh agent for checkpoint detection
      wazuhAgent:
        enabled: true             # Optional: Enable Wazuh agent (default: true)
        configOverrides:          # Optional: Custom syscheck paths
          syscheck:
            directories:
              - "/home/labuser"
              - "/etc/nginx"
```

### Spec: Networks

```yaml
spec:
  networks:
    - name: lab-network           # Required: Unique name
      displayName: "Internal"     # Optional: Human-readable name
      vlan: auto                  # Optional: auto|specific number
      subnet: "10.0.100.0/24"     # Required: CIDR notation
      gateway: "10.0.100.1"       # Optional: Gateway IP
      dhcp:                       # Optional: DHCP configuration
        enabled: true
        rangeStart: "10.0.100.100"
        rangeEnd: "10.0.100.200"
      isolated: true              # Optional: Isolate from other pods

    - name: external
      displayName: "External Network"
      vlan: 999
      subnet: "10.99.0.0/24"
      gateway: "10.99.0.1"
      internet: true              # Optional: Internet access
```

### Spec: Checkpoints

```yaml
spec:
  checkpoints:
    # File existence check
    - id: create-file
      name: "Create configuration file"
      description: "Create /etc/myapp/config.yaml"
      points: 10
      trigger:
        type: file_exists
        params:
          path: "/etc/myapp/config.yaml"
          vm: student-vm          # Optional: specific VM (default: any)

    # File content check
    - id: configure-app
      name: "Configure application"
      description: "Set correct hostname in config"
      points: 15
      trigger:
        type: file_content
        params:
          path: "/etc/myapp/config.yaml"
          contains: "hostname: myserver"
          # OR use regex:
          # pattern: "hostname:\\s+\\w+"

    # Package installation check
    - id: install-nginx
      name: "Install Nginx"
      description: "Install the Nginx web server"
      points: 15
      trigger:
        type: package
        params:
          package: nginx
          state: installed

    # Service check
    - id: start-nginx
      name: "Start Nginx service"
      description: "Ensure Nginx is running"
      points: 20
      trigger:
        type: service
        params:
          service: nginx
          state: running
          enabled: true           # Optional: check enabled at boot

    # User creation check
    - id: create-user
      name: "Create application user"
      description: "Create user 'appuser'"
      points: 10
      trigger:
        type: user_created
        params:
          username: appuser
          groups:                 # Optional: check group membership
            - www-data

    # Permission check
    - id: set-permissions
      name: "Set file permissions"
      description: "Set correct permissions on config"
      points: 10
      trigger:
        type: permission_changed
        params:
          path: "/etc/myapp/config.yaml"
          mode: "0644"            # Optional: check specific mode
          owner: appuser          # Optional: check owner
          group: appuser          # Optional: check group

    # Command execution check
    - id: run-script
      name: "Run setup script"
      description: "Execute the installation script"
      points: 15
      trigger:
        type: command_executed
        params:
          command: "/opt/setup.sh"
          # OR pattern:
          # pattern: "setup\\.sh"

    # Network connection check
    - id: verify-connectivity
      name: "Verify web access"
      description: "Nginx should respond on port 80"
      points: 20
      trigger:
        type: network_connection
        params:
          host: "10.0.100.10"
          port: 80
          protocol: tcp

    # Custom/manual check
    - id: custom-check
      name: "Complete custom task"
      description: "Manually verified by instructor"
      points: 25
      trigger:
        type: manual
        params:
          instructions: "Instructor will verify deployment"
```

### Checkpoint Options

```yaml
checkpoints:
  - id: example
    name: "Example checkpoint"
    points: 20

    # Ordering
    order: 3                      # Optional: Display order
    required: true                # Optional: Must complete to pass

    # Dependencies
    dependsOn:                    # Optional: Prerequisites
      - create-file
      - install-nginx

    # Hints
    hints:                        # Optional: Progressive hints
      - "Check the package manager"
      - "Use 'apt install nginx'"

    # Feedback
    successMessage: "Great job!"  # Optional: Shown on completion
    failureHint: "Check syntax"   # Optional: Shown on failure

    # Partial credit
    partialCredit: true           # Optional: Allow partial points
    partialThreshold: 50          # Optional: Minimum % for partial
```

### Spec: Instructions

```yaml
spec:
  instructions:
    overview: |
      # Lab Overview

      In this lab, you will learn how to set up a basic web server.

      ## Learning Objectives

      - Install and configure Nginx
      - Understand basic Linux permissions
      - Configure firewall rules

    steps:
      - title: "Step 1: Update the system"
        content: |
          First, update your package manager:

          ```bash
          sudo apt update && sudo apt upgrade -y
          ```
        hints:
          - "Use sudo for elevated privileges"

      - title: "Step 2: Install Nginx"
        content: |
          Install the Nginx web server:

          ```bash
          sudo apt install nginx -y
          ```

      - title: "Step 3: Configure firewall"
        content: |
          Allow HTTP traffic through the firewall...

    summary: |
      Congratulations! You've successfully set up a web server.

    resources:
      - title: "Nginx Documentation"
        url: "https://nginx.org/en/docs/"
      - title: "Ubuntu Server Guide"
        url: "https://ubuntu.com/server/docs"
```

### Spec: Snapshots

```yaml
spec:
  snapshots:
    - name: initial               # Required: Snapshot name
      description: "Clean install"
      default: true               # Optional: Default reset point
      vms:                        # Optional: Specific VMs (default: all)
        - student-vm

    - name: nginx-installed
      description: "After Nginx installation"
      afterCheckpoints:           # Optional: Auto-snapshot after these
        - install-nginx
        - configure-nginx
```

## Variable Interpolation

Use variables for dynamic values:

```yaml
spec:
  variables:
    pod_id: "{{ .Pod.ID }}"
    user_name: "{{ .User.Name }}"
    vlan: "{{ .Network.VLAN }}"

  vms:
    - name: student-vm
      networks:
        - network: lab-network
          ip: "10.{{ .Network.VLAN }}.100.10"
```

### Available Variables

| Variable | Description |
|----------|-------------|
| `.Pod.ID` | Unique pod identifier |
| `.Pod.Name` | Pod name |
| `.User.ID` | User identifier |
| `.User.Name` | User display name |
| `.User.Email` | User email |
| `.Network.VLAN` | Assigned VLAN number |
| `.Timestamp` | Current timestamp |

## Validation

Validate templates before use:

```bash
# Using labctl
cd api && ./bin/labctl template validate templates/my-template.yaml

# Using mage
mage template:validate templates/my-template.yaml
```

## Example Templates

See `templates/examples/` for complete examples:

- `minimal-template.yaml` - Minimal viable template
- `full-featured.yaml` - All features demonstrated
- `multi-vm.yaml` - Multiple VMs with networking

## Related Documentation

- [Template Creation Guide](../lab-templates/template-creation-guide.md) - Step-by-step guide
- [Example Templates](../lab-templates/examples.md) - Annotated examples
- [Wazuh Integration](../admin/wazuh-integration.md) - Checkpoint detection
