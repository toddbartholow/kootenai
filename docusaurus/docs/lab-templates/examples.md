---
title: Example Templates
description: Annotated lab template examples
tags:
  - templates
  - examples
---

# Example Templates

This page provides annotated examples of lab templates for common use cases.

## Minimal Template

The simplest possible template with one VM and one checkpoint:

```yaml
apiVersion: v1
kind: LabTemplate

metadata:
  name: minimal-example
  version: "1.0.0"
  description: Minimal template example
  difficulty: beginner
  estimatedMinutes: 15
  tags:
    - example
    - minimal

spec:
  platform: proxmox

  vms:
    - name: student-vm
      template: ubuntu-22.04
      cpu: 1
      memory: 1024
      networks:
        - network: lab-net

  networks:
    - name: lab-net
      subnet: "10.0.100.0/24"

  checkpoints:
    - id: create-file
      name: "Create a file"
      description: "Create a file named test.txt in your home directory"
      points: 100
      trigger:
        type: file_exists
        params:
          path: "/home/labuser/test.txt"

  instructions:
    overview: |
      Create a file to complete this lab.
    steps:
      - title: "Create the file"
        content: |
          Use the `touch` command to create a file:
          ```bash
          touch ~/test.txt
          ```
```

## Linux Administration Lab

A comprehensive Linux sysadmin lab:

```yaml
apiVersion: v1
kind: LabTemplate

metadata:
  name: linux-administration-basics
  version: "1.2.0"
  description: Learn essential Linux system administration tasks
  difficulty: intermediate
  estimatedMinutes: 90
  maxPoints: 100
  passThreshold: 70
  tags:
    - linux
    - sysadmin
    - users
    - services
  category: linux-pathway
  author: Kootenai Team

spec:
  platform: proxmox

  vms:
    - name: server
      displayName: "Linux Server"
      template: ubuntu-22.04-server
      cpu: 2
      memory: 2048
      disk: 20
      networks:
        - network: internal
          ip: "10.0.100.10"
      credentials:
        username: labuser
        password: labpass123
      wazuhAgent:
        enabled: true
        configOverrides:
          syscheck:
            directories:
              - "/home"
              - "/etc"
              - "/var/www"

  networks:
    - name: internal
      displayName: "Internal Network"
      vlan: auto
      subnet: "10.0.100.0/24"
      gateway: "10.0.100.1"
      dhcp:
        enabled: false
      isolated: true

  checkpoints:
    # User Management (25 points)
    - id: create-webadmin
      name: "Create webadmin user"
      description: "Create a new user named 'webadmin'"
      points: 10
      order: 1
      trigger:
        type: user_created
        params:
          username: webadmin
      hints:
        - "Use the useradd or adduser command"
        - "Try: sudo adduser webadmin"

    - id: add-to-sudo
      name: "Add webadmin to sudo group"
      description: "Give webadmin sudo privileges"
      points: 15
      order: 2
      dependsOn:
        - create-webadmin
      trigger:
        type: user_created
        params:
          username: webadmin
          groups:
            - sudo
      hints:
        - "Use the usermod command"
        - "Try: sudo usermod -aG sudo webadmin"

    # Package Management (25 points)
    - id: install-nginx
      name: "Install Nginx"
      description: "Install the Nginx web server package"
      points: 15
      order: 3
      trigger:
        type: package
        params:
          package: nginx
          state: installed

    - id: start-nginx
      name: "Start Nginx service"
      description: "Ensure Nginx is running and enabled at boot"
      points: 10
      order: 4
      dependsOn:
        - install-nginx
      trigger:
        type: service
        params:
          service: nginx
          state: running
          enabled: true

    # Web Configuration (30 points)
    - id: create-webroot
      name: "Create web root directory"
      description: "Create /var/www/mysite directory"
      points: 10
      order: 5
      trigger:
        type: file_exists
        params:
          path: "/var/www/mysite"

    - id: create-index
      name: "Create index page"
      description: "Create an index.html with 'Hello World'"
      points: 10
      order: 6
      dependsOn:
        - create-webroot
      trigger:
        type: file_content
        params:
          path: "/var/www/mysite/index.html"
          contains: "Hello World"

    - id: configure-nginx
      name: "Configure Nginx virtual host"
      description: "Configure Nginx to serve the site"
      points: 10
      order: 7
      dependsOn:
        - create-index
      trigger:
        type: file_content
        params:
          path: "/etc/nginx/sites-available/mysite"
          contains: "/var/www/mysite"

    # Verification (20 points)
    - id: verify-site
      name: "Verify website is accessible"
      description: "Website should respond on port 80"
      points: 20
      order: 8
      required: true
      dependsOn:
        - configure-nginx
      trigger:
        type: network_connection
        params:
          host: "10.0.100.10"
          port: 80
          protocol: tcp
      successMessage: "Congratulations! Your web server is working!"

  snapshots:
    - name: initial
      description: "Fresh Ubuntu server install"
      default: true

    - name: nginx-installed
      description: "After Nginx installation"
      afterCheckpoints:
        - install-nginx
        - start-nginx

  instructions:
    overview: |
      # Linux Administration Basics

      In this lab, you will learn essential Linux system administration tasks:

      - User and group management
      - Package installation
      - Service management
      - Web server configuration

      ## Prerequisites

      - Basic command line familiarity
      - Understanding of file permissions

    steps:
      - title: "Step 1: Connect to your server"
        content: |
          Open the console and log in with:

          - Username: `labuser`
          - Password: `labpass123`

      - title: "Step 2: Create a new user"
        content: |
          Create a new user named `webadmin`:

          ```bash
          sudo adduser webadmin
          ```

          Follow the prompts to set a password.

      - title: "Step 3: Give sudo privileges"
        content: |
          Add the user to the sudo group:

          ```bash
          sudo usermod -aG sudo webadmin
          ```

      - title: "Step 4: Install Nginx"
        content: |
          Update packages and install Nginx:

          ```bash
          sudo apt update
          sudo apt install nginx -y
          ```

      - title: "Step 5: Start and enable Nginx"
        content: |
          ```bash
          sudo systemctl start nginx
          sudo systemctl enable nginx
          ```

      - title: "Step 6: Create your website"
        content: |
          Create the web root and content:

          ```bash
          sudo mkdir -p /var/www/mysite
          echo "Hello World" | sudo tee /var/www/mysite/index.html
          ```

      - title: "Step 7: Configure virtual host"
        content: |
          Create the Nginx configuration for your site.

          See hints if you need help with the configuration format.
        hints:
          - "Create a file in /etc/nginx/sites-available/"
          - "Use sites-enabled symlink to enable it"
          - "Don't forget to reload nginx"

    summary: |
      ## Great job!

      You've learned how to:

      - Create and manage users
      - Install packages with apt
      - Manage systemd services
      - Configure Nginx virtual hosts

    resources:
      - title: "Ubuntu Server Guide"
        url: "https://ubuntu.com/server/docs"
      - title: "Nginx Documentation"
        url: "https://nginx.org/en/docs/"
```

## Multi-VM Network Lab

A networking lab with multiple VMs:

```yaml
apiVersion: v1
kind: LabTemplate

metadata:
  name: vlan-routing-lab
  version: "1.0.0"
  description: Learn VLAN configuration and inter-VLAN routing
  difficulty: intermediate
  estimatedMinutes: 120
  tags:
    - networking
    - vlan
    - routing

spec:
  platform: proxmox

  vms:
    # Router
    - name: router
      displayName: "VyOS Router"
      template: vyos-1.4
      cpu: 1
      memory: 512
      startOrder: 1
      networks:
        - network: management
          ip: "10.0.0.1"
        - network: vlan10
          ip: "10.10.0.1"
        - network: vlan20
          ip: "10.20.0.1"
      credentials:
        username: vyos
        password: vyos

    # VLAN 10 Workstation
    - name: ws-vlan10
      displayName: "Workstation VLAN 10"
      template: ubuntu-22.04-desktop
      cpu: 1
      memory: 1024
      startOrder: 2
      startDelay: 10
      networks:
        - network: vlan10
          ip: "10.10.0.10"
      credentials:
        username: student
        password: student123

    # VLAN 20 Server
    - name: srv-vlan20
      displayName: "Server VLAN 20"
      template: ubuntu-22.04-server
      cpu: 1
      memory: 1024
      startOrder: 2
      startDelay: 10
      networks:
        - network: vlan20
          ip: "10.20.0.10"
      credentials:
        username: student
        password: student123

  networks:
    - name: management
      displayName: "Management"
      subnet: "10.0.0.0/24"
      gateway: "10.0.0.1"
      isolated: true

    - name: vlan10
      displayName: "VLAN 10 - Workstations"
      vlan: 10
      subnet: "10.10.0.0/24"
      gateway: "10.10.0.1"
      isolated: true

    - name: vlan20
      displayName: "VLAN 20 - Servers"
      vlan: 20
      subnet: "10.20.0.0/24"
      gateway: "10.20.0.1"
      isolated: true

  checkpoints:
    - id: configure-vlan10-interface
      name: "Configure VLAN 10 interface"
      description: "Configure the router interface for VLAN 10"
      points: 20
      trigger:
        type: command_executed
        params:
          vm: router
          pattern: "set interfaces ethernet.*vif 10"

    - id: configure-vlan20-interface
      name: "Configure VLAN 20 interface"
      description: "Configure the router interface for VLAN 20"
      points: 20
      trigger:
        type: command_executed
        params:
          vm: router
          pattern: "set interfaces ethernet.*vif 20"

    - id: ping-vlan10-to-router
      name: "VLAN 10 can reach router"
      description: "Workstation can ping the router"
      points: 15
      trigger:
        type: network_connection
        params:
          sourceVm: ws-vlan10
          host: "10.10.0.1"
          protocol: icmp

    - id: ping-vlan20-to-router
      name: "VLAN 20 can reach router"
      description: "Server can ping the router"
      points: 15
      trigger:
        type: network_connection
        params:
          sourceVm: srv-vlan20
          host: "10.20.0.1"
          protocol: icmp

    - id: inter-vlan-routing
      name: "Inter-VLAN routing works"
      description: "VLAN 10 can reach VLAN 20 through the router"
      points: 30
      required: true
      dependsOn:
        - ping-vlan10-to-router
        - ping-vlan20-to-router
      trigger:
        type: network_connection
        params:
          sourceVm: ws-vlan10
          host: "10.20.0.10"
          protocol: icmp

  instructions:
    overview: |
      # VLAN Routing Lab

      Configure inter-VLAN routing on a VyOS router.

      ## Topology

      ```
      ┌─────────────┐     ┌─────────────┐     ┌─────────────┐
      │ Workstation │     │   Router    │     │   Server    │
      │  VLAN 10    │─────│    VyOS     │─────│  VLAN 20    │
      │ 10.10.0.10  │     │             │     │ 10.20.0.10  │
      └─────────────┘     └─────────────┘     └─────────────┘
                          │ 10.10.0.1
                          │ 10.20.0.1
      ```

    steps:
      - title: "Step 1: Access the router"
        content: |
          Connect to the VyOS router console.

          Default credentials: vyos/vyos

      - title: "Step 2: Enter configuration mode"
        content: |
          ```
          configure
          ```

      - title: "Step 3: Configure VLAN interfaces"
        content: |
          Configure the VLAN subinterfaces with the correct IP addresses.
        hints:
          - "Use 'set interfaces ethernet eth1 vif 10 address 10.10.0.1/24'"
```

## Security Lab

A cybersecurity-focused lab:

```yaml
apiVersion: v1
kind: LabTemplate

metadata:
  name: firewall-basics
  version: "1.0.0"
  description: Learn iptables firewall configuration
  difficulty: intermediate
  estimatedMinutes: 60
  tags:
    - security
    - firewall
    - iptables

spec:
  platform: proxmox

  vms:
    - name: firewall-server
      displayName: "Firewall Server"
      template: ubuntu-22.04-server
      cpu: 1
      memory: 1024
      networks:
        - network: external
          ip: "192.168.1.10"
        - network: internal
          ip: "10.0.0.1"

    - name: internal-client
      displayName: "Internal Client"
      template: ubuntu-22.04-desktop
      cpu: 1
      memory: 1024
      networks:
        - network: internal
          ip: "10.0.0.100"

  networks:
    - name: external
      displayName: "External (Untrusted)"
      subnet: "192.168.1.0/24"
      internet: true

    - name: internal
      displayName: "Internal (Trusted)"
      subnet: "10.0.0.0/24"
      isolated: true

  checkpoints:
    - id: enable-forwarding
      name: "Enable IP forwarding"
      description: "Enable packet forwarding in the kernel"
      points: 15
      trigger:
        type: file_content
        params:
          vm: firewall-server
          path: "/etc/sysctl.conf"
          pattern: "net.ipv4.ip_forward\\s*=\\s*1"

    - id: nat-masquerade
      name: "Configure NAT"
      description: "Set up masquerading for outbound traffic"
      points: 25
      trigger:
        type: command_executed
        params:
          vm: firewall-server
          pattern: "iptables.*MASQUERADE"

    - id: block-ssh-external
      name: "Block SSH from external"
      description: "Block SSH connections from the external network"
      points: 30
      trigger:
        type: command_executed
        params:
          vm: firewall-server
          pattern: "iptables.*DROP.*22"

    - id: internal-can-browse
      name: "Internal client can browse internet"
      description: "Verify NAT is working for internal clients"
      points: 30
      required: true
      trigger:
        type: network_connection
        params:
          sourceVm: internal-client
          host: "8.8.8.8"
          port: 443
          protocol: tcp
```

## Tips for Template Authors

1. **Start simple** - Begin with minimal template and add features
2. **Test incrementally** - Validate after each change
3. **Use meaningful IDs** - Checkpoint IDs should be descriptive
4. **Provide hints** - Help students without giving answers
5. **Order dependencies** - Use `dependsOn` to enforce logical order
6. **Document well** - Clear instructions reduce support requests

## Related Documentation

- [YAML Schema](../lab-templates/yaml-schema.md) - Complete schema reference
- [Template Creation Guide](../lab-templates/template-creation-guide.md) - Step-by-step guide
- [Wazuh Integration](../admin/wazuh-integration.md) - Checkpoint detection setup
