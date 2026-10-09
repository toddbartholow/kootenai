---
title: Example Templates
description: Annotated lab template examples
tags:
  - templates
  - examples
---

# Example Templates

Complete lab templates for common shapes of lab. Every template on this page
parses and validates as written — copy one whole, change the names, and it will
load.

Field-by-field documentation is in the [YAML Schema](./yaml-schema.md). To check
your own edits:

```bash
cd api && go build -o bin/labctl ./cmd/labctl
./bin/labctl lab validate ../templates/my-lab.yaml
```

## Minimal template

One VM, one network, one graded objective — the smallest template that is
actually useful. A few fields here are optional to the parsers but load-bearing
in practice; the notes below say which.

```yaml
apiVersion: v1
kind: LabTemplate

metadata:
  name: minimal-example
  description: The smallest useful lab — create one file to pass.
  duration: 15m
  difficulty: beginner
  version: "1.0.0"
  tags:
    - example
    - minimal

spec:
  platform: proxmox

  network:
    segments:
      - name: lab-network
        vlan: 100
        subnet: 10.10.100.0/24
        gateway: 10.10.100.1
        dhcp: false

  vms:
    - name: student-vm
      template: ubuntu-22.04-server
      resources:
        cpu: 1
        memory: 1024
        disk: 16
      networks:
        - segment: lab-network
          ip: 10.10.100.10
      wazuhAgent: true
      agentConfig:
        monitor_paths:
          - /home/labadmin
        realtime: true

  objectives:
    - id: create-file
      description: Create a file named test.txt in your home directory
      points: 100
      hint: "Use 'touch ~/test.txt'"
      triggers:
        - type: file_exists
          target: student-vm
          match:
            path: /home/labadmin/test.txt
```

Three things are load-bearing and easy to get wrong:

- `agentConfig.monitor_paths` has to cover the path the trigger watches. The
  `file_exists` trigger fires on a Wazuh file-integrity event, and the agent
  only reports paths it was told to monitor.
- `networks[].segment` must name a segment declared in
  `spec.network.segments`. Only `labctl lab validate` checks this, so a typo
  here survives both the server-side validator and the Python parser and
  surfaces when the pod is built.
- `triggers[].target` must name a VM declared in `spec.vms`. This one is
  checked everywhere.

## Linux administration lab

A single-VM sysadmin lab: users, packages, services, and a web root. It shows
the grading configuration block, a progressive hint ladder, dependencies
between objectives, and a full set of student instructions.

````yaml
apiVersion: v1
kind: LabTemplate

metadata:
  name: linux-administration-basics
  description: |
    Learn essential Linux system administration: user management, package
    installation, service control, and serving a static site with Nginx.
  duration: 90m
  difficulty: intermediate
  version: "1.2.0"
  author: Kootenai Team
  tags:
    - linux
    - sysadmin
    - users
    - services

  instructions:
    overview: |
      In this lab you will take a bare Ubuntu server and turn it into a working
      web host. Along the way you will create a user, grant it administrative
      rights, install and start a service, and publish a page.

    learning_objectives:
      - Create user accounts and manage group membership
      - Install packages with apt
      - Start and enable systemd services
      - Serve a static site from Nginx

    prerequisites:
      - Basic command line familiarity
      - Understanding of file permissions

    steps:
      - id: step-1
        title: "Connect to your server"
        content: |
          Open the console and log in as `labadmin`. Everything in this lab is
          done from the shell.

      - id: step-2
        title: "Create the webadmin user"
        objective_id: create-webadmin
        content: |
          Create a dedicated account for web administration:

          ```bash
          sudo adduser webadmin
          ```

          Follow the prompts to set a password.

      - id: step-3
        title: "Grant sudo rights"
        objective_id: add-to-sudo
        content: |
          Add the new account to the `sudo` group:

          ```bash
          sudo usermod -aG sudo webadmin
          ```

      - id: step-4
        title: "Install Nginx"
        objective_id: install-nginx
        content: |
          Refresh the package index and install the web server:

          ```bash
          sudo apt update
          sudo apt install nginx -y
          ```

      - id: step-5
        title: "Start and enable the service"
        objective_id: start-nginx
        content: |
          Installing a package does not always start it. Make sure it is
          running now and after a reboot:

          ```bash
          sudo systemctl enable --now nginx
          ```

      - id: step-6
        title: "Publish a page"
        objective_id: create-index
        content: |
          Create a web root and put something in it:

          ```bash
          sudo mkdir -p /var/www/mysite
          echo "Hello World" | sudo tee /var/www/mysite/index.html
          ```

      - id: step-7
        title: "Point Nginx at your site"
        objective_id: configure-nginx
        content: |
          Write a server block in `/etc/nginx/sites-available/mysite` whose
          root is `/var/www/mysite`, symlink it into `sites-enabled`, and
          reload Nginx.

    summary: |
      You created and privileged a user, installed a package, managed a
      systemd unit, and configured an Nginx virtual host.

    tips:
      - "systemctl status nginx tells you why a service refused to start"
      - "nginx -t checks your configuration before you reload it"

    resources:
      - title: "Ubuntu Server Guide"
        url: "https://ubuntu.com/server/docs"
      - title: "Nginx Documentation"
        url: "https://nginx.org/en/docs/"

spec:
  platform: proxmox

  network:
    segments:
      - name: internal
        vlan: 100
        subnet: 10.0.100.0/24
        gateway: 10.0.100.1
        dhcp: false

  vms:
    - name: server
      template: ubuntu-22.04-server
      startOnCreate: true
      resources:
        cpu: 2
        memory: 2048
        disk: 20
      networks:
        - segment: internal
          ip: 10.0.100.10
      snapshots:
        - name: initial
          description: Fresh Ubuntu server install
          default: true
      wazuhAgent: true
      agentConfig:
        monitor_paths:
          - /home
          - /etc
          - /var/www
        audit_commands: true
        realtime: true

  checkpoints:
    enabled: true
    pass_threshold: 70
    allow_retry: true
    show_hints: true
    realtime_update: true

  objectives:
    # User management
    - id: create-webadmin
      description: Create a user account named webadmin
      points: 10
      order: 1
      hints:
        - level: 1
          text: "The command that creates an account is adduser or useradd"
          penalty: 2
        - level: 2
          text: "Run 'sudo adduser webadmin' and answer the prompts"
          penalty: 5
      triggers:
        - type: user_created
          target: server
          match:
            username: webadmin

    - id: add-to-sudo
      description: Add webadmin to the sudo group
      points: 15
      order: 2
      depends_on:
        - create-webadmin
      hint: "usermod -aG adds a supplementary group without removing the others"
      triggers:
        - type: command_executed
          target: server
          match:
            pattern: "usermod.*-aG.*sudo.*webadmin"

    # Package and service management
    - id: install-nginx
      description: Install the Nginx web server package
      points: 15
      order: 3
      triggers:
        - type: package
          target: server
          match:
            package: nginx
            state: installed

    - id: start-nginx
      description: Start Nginx and enable it at boot
      points: 10
      order: 4
      depends_on:
        - install-nginx
      hint: "'systemctl enable --now' does both in one step"
      triggers:
        - type: service
          target: server
          match:
            name: nginx
            state: active

    # Web configuration
    - id: create-webroot
      description: Create the /var/www/mysite directory
      points: 10
      order: 5
      triggers:
        - type: file_exists
          target: server
          match:
            path: /var/www/mysite

    - id: create-index
      description: Create an index.html containing 'Hello World'
      points: 10
      order: 6
      depends_on:
        - create-webroot
      triggers:
        - type: file_content
          target: server
          match:
            path: /var/www/mysite/index.html
            contains: "Hello World"

    - id: configure-nginx
      description: Configure an Nginx server block for the site
      points: 10
      order: 7
      depends_on:
        - create-index
      triggers:
        - type: file_content
          target: server
          match:
            path: /etc/nginx/sites-available/mysite
            contains: "/var/www/mysite"

    # Verification
    - id: verify-site
      description: Serve the site successfully over HTTP
      points: 20
      order: 8
      required: true
      depends_on:
        - configure-nginx
        - start-nginx
      triggers:
        - type: active_check
          target: server
          script: |
            #!/bin/bash
            curl -fsS "http://${VM_IP}/" | grep -q "Hello World"
````

Worth noting:

- `add-to-sudo` grades on `command_executed` rather than `user_created`. The
  `user_created` evaluator reads only `match.username` — it ignores `groups` —
  so group membership has to be detected another way.
- `verify-site` is an `active_check`: a script runs and its exit code decides.
  The script executes **on the API server**, not inside the VM, with `${VM_IP}`
  substituted for the target's address — which is why it curls `${VM_IP}`
  rather than localhost. To inspect state inside the VM, SSH to `${VM_IP}`
  yourself, as the routing lab below does. Reach for `active_check` when the
  thing you want to grade is a *state* rather than an *event*, since event
  triggers only fire when the student performs the action while the agent is
  watching.
- `required: true` marks the objective as mandatory for a human reader and for
  `labctl lab show`, but grading does not currently enforce it — a pass is
  decided by `pass_threshold` against the total score. Treat it as intent, not
  as a gate. `depends_on` *is* enforced: the evaluator will not credit an
  objective whose dependencies have not passed.

## Multi-VM routing lab

Three VMs across two network segments. This is the shape to copy for any
topology lab. It is adapted from `templates/networking/basic-routing.yaml`.

```yaml
apiVersion: v1
kind: LabTemplate

metadata:
  name: linux-routing-fundamentals
  description: |
    Configure static routing between two networks on Linux. Enable IP
    forwarding on a router and verify end-to-end connectivity between clients.
  duration: 60m
  difficulty: intermediate
  version: "1.0.0"
  author: Kootenai Team
  tags:
    - linux
    - networking
    - routing

spec:
  platform: proxmox

  network:
    segments:
      - name: network-a
        vlan: 101
        subnet: 10.10.101.0/24
        gateway: 10.10.101.1
        dhcp: false
      - name: network-b
        vlan: 102
        subnet: 10.10.102.0/24
        gateway: 10.10.102.1
        dhcp: false

  vms:
    # The router straddles both segments.
    - name: router
      template: ubuntu-22.04-server-minimal
      resources:
        cpu: 1
        memory: 1024
        disk: 8
      networks:
        - segment: network-a
          ip: 10.10.101.1
        - segment: network-b
          ip: 10.10.102.1
      snapshots:
        - name: initial
          description: Base router — IP forwarding disabled
          default: true
      wazuhAgent: true
      agentConfig:
        monitor_paths:
          - /etc/sysctl.conf
          - /etc/sysctl.d
          - /etc/netplan
        audit_commands: true
        realtime: true

    - name: client-a
      template: ubuntu-22.04-desktop
      resources:
        cpu: 1
        memory: 2048
        disk: 16
      networks:
        - segment: network-a
          ip: 10.10.101.10
      snapshots:
        - name: initial
          description: Client on Network A
          default: true
      wazuhAgent: true
      agentConfig:
        monitor_paths:
          - /etc/netplan
        audit_commands: true
        realtime: true

    - name: client-b
      template: ubuntu-22.04-desktop
      resources:
        cpu: 1
        memory: 2048
        disk: 16
      networks:
        - segment: network-b
          ip: 10.10.102.10
      snapshots:
        - name: initial
          description: Client on Network B
          default: true
      wazuhAgent: true
      agentConfig:
        monitor_paths:
          - /etc/netplan
        audit_commands: true
        realtime: true

  checkpoints:
    enabled: true
    pass_threshold: 75
    allow_retry: true
    show_hints: true
    realtime_update: true

  objectives:
    - id: view-routing-table
      description: View the current routing table on the router
      points: 5
      order: 1
      hint: "Use 'ip route' or 'route -n' to view routes"
      triggers:
        - type: command_executed
          target: router
          match:
            pattern: "ip route|route -n|netstat -rn"

    - id: enable-ip-forwarding
      description: Enable IP forwarding on the router
      points: 20
      order: 2
      hint: "Set net.ipv4.ip_forward=1 in /etc/sysctl.conf, then run 'sysctl -p'"
      triggers:
        # Two triggers: the config file says so, and the kernel agrees.
        - type: file_content
          target: router
          match:
            path: /etc/sysctl.conf
            regex: "^net\\.ipv4\\.ip_forward\\s*=\\s*1"
        - type: active_check
          target: router
          script: |
            #!/bin/bash
            ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 "${VM_IP}" \
              'test "$(cat /proc/sys/net/ipv4/ip_forward)" = 1'

    - id: set-gateway-client-a
      description: Configure the default gateway on Client A to use the router
      points: 15
      order: 3
      depends_on:
        - enable-ip-forwarding
      hint: "Use netplan, or 'ip route add default via 10.10.101.1'"
      triggers:
        - type: active_check
          target: client-a
          script: |
            #!/bin/bash
            ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 "${VM_IP}" \
              'ip route' | grep -q "default via 10.10.101.1"

    - id: set-gateway-client-b
      description: Configure the default gateway on Client B to use the router
      points: 15
      order: 4
      depends_on:
        - enable-ip-forwarding
      hint: "Use netplan, or 'ip route add default via 10.10.102.1'"
      triggers:
        - type: active_check
          target: client-b
          script: |
            #!/bin/bash
            ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 "${VM_IP}" \
              'ip route' | grep -q "default via 10.10.102.1"

    - id: ping-client-b-from-a
      description: Successfully ping Client B from Client A
      points: 30
      order: 5
      required: true
      depends_on:
        - set-gateway-client-a
        - set-gateway-client-b
      hint: "From Client A, run 'ping 10.10.102.10' — this tests full routing"
      triggers:
        - type: active_check
          target: client-a
          script: |
            #!/bin/bash
            ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 "${VM_IP}" \
              'ping -c 3 -W 2 10.10.102.10'

    - id: traceroute
      description: Trace the path from Client A to Client B
      points: 15
      order: 6
      depends_on:
        - ping-client-b-from-a
      hint: "Use 'traceroute 10.10.102.10' to see each hop"
      triggers:
        - type: command_executed
          target: client-a
          match:
            pattern: "traceroute.*10\\.10\\.102\\.10|tracepath.*10\\.10\\.102\\.10"
```

Two things about this lab are worth copying:

- A trigger is evaluated against one VM — the one named in `target`. There is
  no "source VM" concept. To grade a connection, point an `active_check` at the
  host that initiates it and SSH in from the script, as `ping-client-b-from-a`
  does: `${VM_IP}` resolves to Client A, and the `ping` then runs from there.
- `enable-ip-forwarding` carries two triggers, a `file_content` check and an
  `active_check`. On the event path any one of an objective's triggers
  completes it, so whichever arrives first credits the student. The
  active-check runner is stricter and requires all of them, so treat a second
  trigger as a safety net rather than as an additional requirement.

## Firewall lab

Two VMs, two segments, and a mix of configuration and behaviour checks.

```yaml
apiVersion: v1
kind: LabTemplate

metadata:
  name: firewall-basics
  description: |
    Configure an iptables firewall: enable forwarding, masquerade outbound
    traffic, and block SSH from the untrusted network.
  duration: 60m
  difficulty: intermediate
  version: "1.0.0"
  tags:
    - security
    - firewall
    - iptables

spec:
  platform: proxmox

  network:
    segments:
      - name: external
        vlan: 110
        subnet: 192.168.1.0/24
        gateway: 192.168.1.1
        dhcp: false
      - name: internal
        vlan: 120
        subnet: 10.0.0.0/24
        gateway: 10.0.0.1
        dhcp: false

  vms:
    - name: firewall-server
      template: ubuntu-22.04-server
      resources:
        cpu: 1
        memory: 1024
        disk: 16
      networks:
        - segment: external
          ip: 192.168.1.10
        - segment: internal
          ip: 10.0.0.1
      snapshots:
        - name: initial
          description: Firewall before any rules are applied
          default: true
      wazuhAgent: true
      agentConfig:
        monitor_paths:
          - /etc/sysctl.conf
          - /etc/iptables
        audit_commands: true
        realtime: true

    - name: internal-client
      template: ubuntu-22.04-desktop
      resources:
        cpu: 1
        memory: 2048
        disk: 16
      networks:
        - segment: internal
          ip: 10.0.0.100
      wazuhAgent: true
      agentConfig:
        monitor_paths:
          - /etc/netplan
        audit_commands: true
        realtime: true

  checkpoints:
    enabled: true
    pass_threshold: 70
    allow_retry: true
    show_hints: true
    realtime_update: true

  objectives:
    - id: enable-forwarding
      description: Enable packet forwarding in the kernel
      points: 15
      order: 1
      hint: "net.ipv4.ip_forward = 1 in /etc/sysctl.conf, then 'sysctl -p'"
      triggers:
        - type: file_content
          target: firewall-server
          match:
            path: /etc/sysctl.conf
            regex: "^net\\.ipv4\\.ip_forward\\s*=\\s*1"

    - id: nat-masquerade
      description: Masquerade outbound traffic from the internal network
      points: 25
      order: 2
      depends_on:
        - enable-forwarding
      hint: "A MASQUERADE rule belongs in the nat table's POSTROUTING chain"
      triggers:
        - type: command_executed
          target: firewall-server
          match:
            pattern: "iptables.*MASQUERADE"

    - id: block-ssh-external
      description: Drop inbound SSH from the external network
      points: 30
      order: 3
      hints:
        - level: 1
          text: "You want an INPUT rule matching tcp destination port 22"
          penalty: 5
        - level: 2
          text: "Match the external interface with -i and end the rule with -j DROP"
          penalty: 10
      triggers:
        - type: command_executed
          target: firewall-server
          match:
            pattern: "iptables.*(--dport|--destination-port) 22.*DROP"

    - id: internal-can-reach-out
      description: Confirm the internal client can still reach the external network
      points: 30
      order: 4
      required: true
      depends_on:
        - nat-masquerade
      triggers:
        - type: active_check
          target: internal-client
          script: |
            #!/bin/bash
            ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 "${VM_IP}" \
              'ping -c 3 -W 2 192.168.1.1'
```

## Inheritance: a base template and a child

A base template holds the VM and network configuration that a family of labs
shares. A child names it in `metadata.extends` and adds to it.

The parent is an ordinary template. Note the `${LAB_SUBNET}` and
`${LAB_GATEWAY}` references, which are substituted from `metadata.variables`:

```yaml
apiVersion: v1
kind: LabTemplate

metadata:
  name: base-linux-lab
  description: Base template for Linux labs — common VM and network setup.
  duration: 60m
  difficulty: beginner
  version: "1.0.0"
  author: Kootenai Team
  variables:
    LAB_SUBNET: "10.0.0.0/24"
    LAB_GATEWAY: "10.0.0.1"

spec:
  platform: proxmox

  network:
    segments:
      - name: lab-network
        subnet: ${LAB_SUBNET}
        gateway: ${LAB_GATEWAY}
        dhcp: false

  vms:
    - name: student-vm
      template: ubuntu-22.04-desktop
      resources:
        cpu: 2
        memory: 4096
        disk: 32
      wazuhAgent: true
      agentConfig:
        monitor_paths:
          - /home
          - /etc
        audit_commands: true
```

The child overrides the variables, adds a second VM, and contributes
objectives. Objectives are added to the parent's rather than replacing them,
and the child does not repeat `spec.network` — it inherits it:

```yaml
apiVersion: v1
kind: LabTemplate

metadata:
  name: network-security-intro
  description: Introduction to network security — extends base-linux-lab.
  extends: base-linux-lab
  duration: 120m
  difficulty: intermediate
  version: "1.0.0"
  author: Security Team
  tags:
    - security
    - networking
  variables:
    LAB_SUBNET: "192.168.100.0/24"
    LAB_GATEWAY: "192.168.100.1"

spec:
  # student-vm comes from the parent; this one is added alongside it.
  vms:
    - name: target-server
      template: debian-11
      resources:
        cpu: 1
        memory: 2048
      networks:
        - segment: lab-network
          ip: 192.168.100.100

  objectives:
    - id: scan-target
      description: Successfully scan the target server
      points: 20
      hint: "Use nmap to discover open ports"
      triggers:
        - type: command_executed
          target: student-vm
          match:
            pattern: nmap

    - id: identify-vuln
      description: Identify a vulnerability on the target and record it
      points: 30
      triggers:
        - type: file_content
          target: student-vm
          match:
            path: /home/student/report.txt
            contains: "CVE-"
```

Both files are in the repository as `templates/examples/base-linux-lab.yaml`
and `templates/examples/child-security-lab.yaml`.

Two things to expect when validating an inheritance pair locally:

- `labctl lab validate` reports the child's `target: student-vm` as an unknown
  VM and its `segment: lab-network` as an unknown segment. The Go side has no
  notion of a fragment and resolves both against the child's own document,
  before the merge. The Python parser skips these checks when
  `metadata.extends` is set. The messages are advisory — the template loads and
  merges correctly.
- The parent validates cleanly despite its `${LAB_SUBNET}`, because
  `labctl lab validate` does not check address formats. The server's loader
  does, and it validates before substituting, so it logs a CIDR warning for the
  same file.

Note also that the merge does not carry `metadata.instructions` through. If a
lab needs a student-facing walkthrough, write it as a standalone template
rather than a child.

## Writing your own

1. **Start from a template that already validates.** The minimal template above
   is the smallest starting point; `templates/examples/simple-linux-intro.yaml`
   is the most heavily commented one in the repository.
2. **Match `monitor_paths` to your triggers.** An event trigger only fires for
   a path the Wazuh agent was configured to watch, and `command_executed`
   needs `audit_commands: true`.
3. **Prefer `active_check` for state, event triggers for actions.** "The
   service is running now" is a state; "the student ran usermod" is an action.
   Remember that an `active_check` script runs on the API server — use
   `${VM_IP}` to reach the VM, and SSH in if you need to read something local
   to it.
4. **Give every trigger a `match` block.** An empty `match` matches any event
   on the target VM, which silently awards the points for doing nothing in
   particular.
5. **Use `depends_on` to order the work.** It gates objectives rather than just
   sorting them; `order` handles presentation.
6. **Validate before committing.** CI parses every lab template under
   `templates/` with the stricter Python parser, so run that one too.

## Related documentation

- [YAML Schema](./yaml-schema.md) — complete field reference, including the trigger catalogue
- [Wazuh Integration](../admin/wazuh-integration.md) — how file, command, and package events reach the checkpoint evaluator
- [Template Creation Guide](./template-creation-guide.md) — building the Proxmox VM images that `vms[].template` refers to
