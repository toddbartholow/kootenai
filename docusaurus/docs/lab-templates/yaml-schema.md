---
title: Lab Template YAML Schema
description: Complete specification for lab template YAML files
tags:
  - templates
  - schema
  - yaml
---

# Lab Template YAML Schema

A lab template is a YAML document that describes a complete lab environment:
the virtual machines, the networks that connect them, the graded objectives,
and the student-facing instructions. Templates are hypervisor-agnostic — the
same document targets Proxmox or CloudStack — and nothing in the format is
specific to either.

This page is the field-by-field reference. Every YAML block on this page is
either a complete template or a fragment, and each has been validated against
the parsers described below.

## The two parsers

The schema has two independent implementations, and they are not identical.
Know which one you are targeting.

| | Go | Python |
|---|---|---|
| Structs / models | `api/internal/models/types.go`, `checkpoint.go` | `python/lab_templates/parser.py` |
| Schema checks | `api/internal/templates/validator/validator.go` | pydantic models |
| Used by | the API server's template loader, `labctl lab validate` | the Python tooling and the CI template test |

The Go side is the production parser: it is what loads `templates/` into the
database at server start. The Python side is stricter, and CI parses every
shipped template with it, so a template that satisfies only Go can still fail
the build.

Where they differ:

| Rule | Go | Python |
|---|---|---|
| `apiVersion` | **required** | optional; defaults to `v1` |
| `metadata.duration` | optional | **required** |
| `metadata.difficulty` | optional | **required** |
| `metadata.description` | must be at least 10 characters | any non-empty string |
| `metadata.name` format | must be slug-form, but see below | any string, 1–128 chars |
| Trigger `match` block | optional for every type | **required** except for `active_check` and `custom` |
| `objectives[].triggers` | may be empty | **at least one required** |
| `depends_on` targets | checked by `labctl lab validate` only; the server-side validator ignores them | checked, and self-dependency rejected |
| Unknown fields | silently discarded | silently discarded, except inside `match`, which rejects them |

The practical rule: write for Python and you satisfy both.

> **metadata.name**
>
> The Go validator requires `metadata.name` to match
> `^[a-z0-9][a-z0-9-]*[a-z0-9]$`, but the loader also derives a URL slug from the
> name itself (`generateSlug` in `api/internal/templates/loader.go`), which only
> makes sense if the name is a display string. The corpus follows the loader, not
> the validator: 57 of the 62 shipped templates use a human-readable name like
> `Linux Routing Fundamentals`.
>
> This is survivable because validation is advisory — the loader logs validation
> failures and carries on unless it was built with `WithStrictMode()`, which
> nothing enables. Examples on this page use slug-form names so they pass every
> check cleanly, but a display name will load.

## Template location

Templates live under `templates/`, one directory per subject area:

```
templates/
├── aws/
├── cloud/
├── cloud-security/
├── cybersecurity/
├── database/
├── devops/
├── examples/
│   ├── base-linux-lab.yaml       # parent template for inheritance
│   ├── child-security-lab.yaml   # child that extends it
│   └── simple-linux-intro.yaml   # the commented reference template
├── linux-advanced/
├── linux-pathway/
├── networking/
├── networking-advanced/
├── python/
├── scenarios/
├── security/
├── vim/
└── windows-ad/
```

Files ending in `.scenario.yaml` under `templates/scenarios/` are `labtest`
simulator scenarios, not lab templates, and are not parsed by this schema.

## Minimal template

The smallest document that both parsers accept:

```yaml
apiVersion: v1
kind: LabTemplate

metadata:
  name: minimal-example
  description: The smallest lab template that validates.
  duration: 15m
  difficulty: beginner
  version: "1.0.0"

spec:
  platform: proxmox

  network:
    segments:
      - name: lab-network
        vlan: 100
        subnet: 10.10.100.0/24
        gateway: 10.10.100.1

  vms:
    - name: linux-vm
      template: ubuntu-22.04-server
      resources:
        cpu: 1
        memory: 2048
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
          target: linux-vm
          match:
            path: /home/labadmin/test.txt
```

`wazuhAgent` and `agentConfig` are not required by either parser, but without
them the agent reports nothing and a `file_exists` trigger can never fire.
Python's `validate_template()` warns about exactly this: an objective whose
target VM has no agent enabled.

## Root level

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `apiVersion` | string | Go: yes, Python: no | `v1`. Python defaults it to `v1` when absent; Go rejects a template without it. The legacy value `virtuallab.dev/v1` is still accepted for templates written before the rename, but is never emitted. |
| `kind` | string | Yes | Always `LabTemplate`. |
| `metadata` | object | Yes | Identity and student-facing content. |
| `spec` | object | Yes | The environment and its grading. |

## metadata

```yaml
metadata:
  name: linux-routing-fundamentals
  description: |
    Learn basic IP routing on Linux. Configure static routes, enable IP
    forwarding, and read a routing table.
  duration: 60m
  difficulty: intermediate
  version: "1.0.0"
  author: Kootenai Team
  tags:
    - linux
    - networking
    - routing
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Lab name. The loader slugifies it for URLs. See the note above on format. |
| `description` | string | Yes | Go additionally requires at least 10 characters; Python accepts any non-empty string. |
| `duration` | string | Go: no, Python: **yes** | Duration form: `30m`, `2h`, `1h30m`. It must be a *string* — an unquoted `60` is a YAML integer, which Go coerces but Python rejects. Quote it as `"60"` if you want a bare minute count. |
| `difficulty` | string | Go: no, Python: **yes** | One of `beginner`, `intermediate`, `advanced`, `expert`. |
| `version` | string | No | Semantic version, e.g. `"1.0.0"` or `"v1.0"`. Quote it, or YAML reads `1.0` as a float. |
| `author` | string | No | Free text. |
| `tags` | list of string | No | Free-form labels. |
| `extends` | string | No | Name of a parent template. See [Template inheritance](#template-inheritance). |
| `variables` | map of string to string | No | Substitution values. See [Variable interpolation](#variable-interpolation). |
| `instructions` | object | No | The student-facing walkthrough. See below. |

### metadata.instructions

The whole educational body of the lab lives here — **under `metadata`, not
under `spec`**. The server stores it and serves it from
`GET /api/v1/labs/{id}/instructions`.

````yaml
metadata:
  instructions:
    overview: |
      Welcome to your first Linux lab. You will create directories and files
      and learn how the filesystem is laid out.

    learning_objectives:
      - Create directories with mkdir
      - Create and write files with touch and echo

    prerequisites:
      - Basic computer literacy

    steps:
      - id: step-1
        title: "Understanding your home directory"
        objective_id: create-directory
        content: |
          Every user has a home directory. The `~` shortcut always refers to
          yours.

          ```bash
          mkdir ~/mywork
          ```

      - id: step-2
        title: "Creating an empty file"
        objective_id: create-file
        content: |
          `touch` creates an empty file if one does not already exist.

          ```bash
          touch ~/mywork/notes.txt
          ```

    summary: |
      You have learned mkdir, touch, and the meaning of `~`.

    tips:
      - "Press Tab to auto-complete paths"
      - "Run 'man ls' to read the manual for any command"

    resources:
      - title: "Linux Command Line Basics"
        url: "https://ubuntu.com/tutorials/command-line-for-beginners"
````

| Field | Type | Description |
|-------|------|-------------|
| `overview` | string | Markdown introduction. |
| `learning_objectives` | list of string | Shown before the lab starts. |
| `prerequisites` | list of string | Assumed prior knowledge. |
| `steps` | list of object | The walkthrough. Each step takes `id`, `title`, `content`, and an optional `objective_id`. |
| `summary` | string | Markdown wrap-up. |
| `tips` | list of string | Short asides. |
| `resources` | list of object | Each takes `title` and `url`. |

`steps[].objective_id` ties a step to an entry in `spec.objectives`, which is
what lets the UI show progress against the text. A step has **no** `hints`
field — hints belong to objectives.

## spec

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `platform` | string | No | `proxmox`, `cloudstack`, or `any`. Defaults to `proxmox` in Python and to `any` at load time in Go. |
| `network` | object | Python: yes, unless `metadata.extends` is set | Contains a single key, `segments`, which must hold at least one entry. Go requires neither. |
| `vms` | list of object | Yes | Go requires at least one unconditionally; Python requires at least one unless `metadata.extends` is set. The Go validator caps the list at 10. |
| `checkpoints` | object | No | Grading **configuration**, not a list of graded items. |
| `objectives` | list of object | No | The graded items. |

Two further keys, `spec.assessment` and `spec.questions`, exist on the Go
struct — Packet-Tracer-style device checks and manual Q&A respectively — but no
shipped template uses either, and the Python parser does not model them, so it
would drop them. Treat them as unfinished rather than as part of the format.

### spec.network

`network` is an object with one key — `segments`. There is no top-level
`spec.networks` list.

```yaml
spec:
  network:
    segments:
      - name: lab-network
        vlan: 100
        subnet: 10.10.100.0/24
        gateway: 10.10.100.1
        dhcp: false

      - name: dmz
        vlan: 200
        subnet: 10.10.200.0/24
        gateway: 10.10.200.1
        dhcp: true
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Unique within the template. VMs reference it by this name. |
| `vlan` | integer | No | 1–4094. An integer only — there is no `auto` keyword. Omit it to leave the VLAN unassigned. |
| `subnet` | string | Yes | CIDR, e.g. `10.10.100.0/24`. |
| `gateway` | string | No | A plain IPv4 address. |
| `dhcp` | boolean | No | A boolean, not a range object. Defaults to `false`. |

A VM attaches to a segment by name through `vms[].networks[].segment`. Only
`labctl lab validate` cross-checks that reference — neither the server-side
validator nor the Python parser does — so a typo there is caught at runtime
rather than at validation. Run the CLI over a template that changes its network
layout.

### spec.vms

```yaml
spec:
  vms:
    - name: linux-vm
      template: ubuntu-22.04-server
      startOnCreate: true
      resources:
        cpu: 1
        memory: 2048
        disk: 16
      networks:
        - segment: lab-network
          ip: 10.10.100.10
      snapshots:
        - name: initial
          description: Fresh Ubuntu installation
          default: true
      wazuhAgent: true
      agentConfig:
        monitor_paths:
          - /home/labadmin
          - /var/log
        audit_commands: true
        realtime: true
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Unique within the template. Triggers target a VM by this name. |
| `template` | string | Yes | Base image name on the hypervisor. |
| `resources` | object | Yes | Nested. There are no top-level `cpu`/`memory`/`disk` fields on a VM. |
| `networks` | list of object | No | Connections to declared segments. |
| `snapshots` | list of object | No | Named reset points. Snapshots are per-VM; there is no `spec.snapshots`. |
| `startOnCreate` | boolean | No | Start the VM when the pod is created. Python defaults it to `true`, Go to `false`. Set it explicitly. |
| `wazuhAgent` | boolean | No | A boolean switch. Per-VM tuning goes in `agentConfig`. |
| `agentConfig` | object | No | Wazuh agent tuning. |

#### resources

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `cpu` | integer | Yes | vCPUs. Python: 1–64. Go validator: at most 16. |
| `memory` | integer | Yes | MB. Python: at least 512. Go validator: at most 65536. |
| `disk` | integer | No | GB. Go validator: at most 500. |

#### networks

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `segment` | string | Yes | Must name a segment in `spec.network.segments`. The key is `segment`, not `network`. |
| `ip` | string | No | Static address. |

#### snapshots

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Snapshot name. |
| `description` | string | No | Shown in the UI. |
| `default` | boolean | No | The state "reset lab" returns to. |
| `includeRAM` | boolean | No | Capture memory as well as disk. |

#### agentConfig

| Field | Type | Description |
|-------|------|-------------|
| `monitor_paths` | list of string | Directories the agent watches for file events. |
| `audit_commands` | boolean | Record executed commands. Needed by `command_executed` triggers. |
| `audit_syscalls` | boolean | Record syscalls. |
| `realtime` | boolean | Report file events as they happen rather than on a scan interval. |

Note the underscores: these four keys are snake_case even though `wazuhAgent`
and `startOnCreate` beside them are camelCase. The schema is inconsistent here;
both parsers agree on it.

### spec.checkpoints

`checkpoints` is a **configuration object** describing how the lab is graded.
It is not a list of graded items — those are `spec.objectives`.

```yaml
spec:
  checkpoints:
    enabled: true
    pass_threshold: 75
    allow_retry: true
    show_hints: true
    realtime_update: true
    grade_on_submit: false
```

| Field | Type | Description |
|-------|------|-------------|
| `enabled` | boolean | Whether checkpoint grading runs at all. |
| `pass_threshold` | integer | Percentage needed to pass, 0–100. Defaults to 70. |
| `allow_retry` | boolean | Let a student re-attempt a failed objective. |
| `show_hints` | boolean | Expose the hint ladder in the UI. |
| `realtime_update` | boolean | Push progress as events arrive rather than at submit. |
| `grade_on_submit` | boolean | Evaluate everything at submission instead of continuously. |

### spec.objectives

Each objective is one graded item.

```yaml
spec:
  objectives:
    - id: view-routing-table
      description: View the current routing table
      points: 5
      order: 1
      triggers:
        - type: command_executed
          target: linux-vm
          match:
            pattern: "ip route|route -n|netstat -rn"

    - id: enable-ip-forwarding
      description: Enable IP forwarding on the router
      points: 20
      hint: "Edit /etc/sysctl.conf and set net.ipv4.ip_forward=1"
      order: 2
      required: true
      timeout_minutes: 15
      depends_on:
        - view-routing-table
      triggers:
        - type: file_content
          target: linux-vm
          match:
            path: /etc/sysctl.conf
            regex: "^net\\.ipv4\\.ip_forward\\s*=\\s*1"
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | Yes | Unique within the template. Referenced by `depends_on` and by `instructions.steps[].objective_id`. |
| `description` | string | Yes | What the student has to do. There is no separate `name` field — `description` is the label. |
| `points` | integer | Yes | Non-negative. The lab's max score is the sum across objectives. |
| `hint` | string | No | Single free hint. |
| `hints` | list of object | No | Progressive hint ladder. See below. |
| `depends_on` | list of string | No | Objective IDs that must pass first. Snake_case — `dependsOn` is not read. This one is enforced: the evaluator checks it before crediting an objective. |
| `timeout_minutes` | integer | No | Accepted by both parsers, but nothing reads it today. |
| `required` | boolean | No | Marks the objective as mandatory. Stored and displayed — `labctl lab show` flags it — but grading does not enforce it; passing is decided by `pass_threshold` alone. |
| `order` | integer | No | Display order. The lab builder in the web UI renumbers these from 1. |
| `triggers` | list of object | Python: **yes** | How completion is detected. |

`hint` and `hints` can both be set; the ladder wins.

#### hints

```yaml
spec:
  objectives:
    - id: list-databases
      description: List the databases on the PostgreSQL server
      points: 10
      hints:
        - level: 1
          text: "Connect with 'sudo -u postgres psql' first"
          penalty: 2
        - level: 2
          text: "Once connected, the list command is backslash-l"
          penalty: 5
      triggers:
        - type: command_executed
          target: linux-vm
          match:
            pattern: "psql"
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `level` | integer | Yes | 1 or greater. Must be unique within the objective. |
| `text` | string | Yes | The hint. |
| `penalty` | integer | No | Points deducted when the student reveals it. Defaults to 0. |

A hint entry is an object. A bare list of strings is not accepted.

#### triggers

`triggers` is a **list**. Each entry is one detection rule.

How many of them have to succeed depends on which path credits the objective.
On the event path — the Wazuh-driven evaluator, which is how most objectives
are graded — **any one** trigger firing completes the objective. The
active-check runner, which polls on a timer, is stricter: it credits an
objective only when **every** trigger passes. An objective mixing an event
trigger with an `active_check` will therefore usually be credited by the event
that arrives first.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | Yes | See the catalogue below. |
| `target` | string | Yes | A VM name from `spec.vms`. Both parsers reject a target that is not declared. |
| `match` | object | Usually | The matching criteria. Optional only for `active_check` and `custom`. |
| `script` | string | For `active_check` | Shell script. It runs **on the API server**, not on the VM — see below. Exit code 0 passes. |

> **Always write a match block**
>
> Go treats each `match` field as a constraint applied only if it is set, so an
> empty `match` is not "matches nothing" — it is "matches anything". A
> `file_exists` trigger with no `match` awards the points when the student
> creates *any* file on the target VM. Python rejects this outright for every
> type that is graded by a match.

## Trigger catalogue

`match` is one flat object shared by every trigger type, and each type reads
only its own subset of the fields. A key outside a type's subset still parses —
it simply has no effect on grading. The Python parser rejects a key that
belongs to no type at all, because its `TriggerMatch` model sets
`extra: "forbid"`; Go accepts and ignores it.

| Type | `match` fields read | Notes |
|---|---|---|
| `file_exists` | `path` | Fires on an `added` or `modified` file event. `contains` and `regex` are **not** read for this type. A trailing `*` on the path prefix-matches, and a directory path also matches an event for a file inside it. |
| `file_content` | `path`, `contains`, `regex` | The only file type that inspects content. `contains` is a literal substring; `regex` is a Go regular expression. Both are tested against the new content and the diff. |
| `file_deleted` | `path` | Fires on a `deleted` file event. `contains` and `regex` are not read. |
| `package` | `package`, `state` | `state` is `installed` or `removed`. |
| `service` | `name`, `state` | `name` is the unit name; `state` is `active`, `inactive`, or `failed`. |
| `command_executed` | `pattern`, `user` | `pattern` is a regex against the command line. Needs `audit_commands: true` on the VM. |
| `user_created` | `username` | Only `username` is read; `groups` and `uid` are on the schema but nothing reads them. |
| `permission_changed` | `path`, `permission`, `owner`, `group` | Use `permission`, e.g. `"755"`. See the note below on `mode`. |
| `network_connection` | `destination` (or `address`), `port`, `protocol`, `state` | The remote endpoint is `destination`, not `host`. |
| `active_check` | none | Runs `script` on the API server; exit 0 passes. No `match` field is read at all — the `match: {state: success}` seen in shipped templates is inert. |
| `custom` | — | Open-ended; no match required. |
| `disk_usage` | `mount_point`, `threshold_pct`, `operator` | `operator` is one of `lt`, `le`, `eq`, `ge`, `gt`. |
| `cpu_load` | `threshold_pct`, `threshold_value`, `operator` | |
| `memory_usage` | `threshold_pct`, `operator` | |
| `process_running` | `process_name` | |
| `port_listening` | `port`, `protocol`, `listen_address`, `state` | |
| `cron_job` | `schedule`, `command`, `pattern`, `user` | `command` here is the cron command; `command_executed` matches on `pattern`. |
| `firewall_rule` | `action`, `chain`, `protocol`, `port`, `destination`, `interface` | |

Three keys are on the `match` model but read by nothing at all: `uid`,
`groups`, and `process_count`. They validate and then do nothing.

A fourth, `mode`, belongs to a second code path. The active verifier
(`api/internal/checkpoint/active_verify.go`) cross-checks a passed checkpoint
against the live VM through the QEMU guest agent, and it is enabled whenever
the server has an orchestrator configured. It reads `mode` and `user` where the
event evaluator reads `permission` and `username`. A `permission_changed` or
`user_created` objective that should hold up under both paths needs both
spellings set.

There is no `manual` trigger type. For instructor-verified work, use `custom`.

### Where an active check actually runs

This catches people out, and much of the shipped corpus gets it wrong.

`active_check` scripts are executed with `bash -c` **on the API server**, not
inside the lab VM. Before running, the checker substitutes two placeholders:

| Placeholder | Value |
|---|---|
| `${VM_IP}` | the resolved IP of the trigger's `target` VM |
| `${TARGET}` | the trigger's `target` VM name |

So a script that reaches the VM over the network — `curl`, `ping`, `nc` — works
as written. A script that inspects local state does not: `cat /proc/sys/...`
reads the API server's `/proc`, and passes or fails on the server's
configuration rather than the student's. To inspect state inside the VM, SSH to
it yourself:

```bash
ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 "${VM_IP}" \
  'test "$(cat /proc/sys/net/ipv4/ip_forward)" = 1'
```

The other actively-checked types — `file_exists`, `file_content`, and `service`
with a `state` — build their own `ssh ${VM_IP} '...'` command, so those do run
on the VM without you doing anything.

Eight of these are exercised by the shipped corpus — `command_executed` (267
triggers), `file_content` (186), `file_exists` (46), `active_check` (8),
`package` (2), `service` (1), `user_created` (1), and `permission_changed` (1).
The other ten are implemented and accepted by both validators but unused so
far, so treat them as less well-trodden.

### Worked trigger examples

Each block below is a fragment of `spec.objectives`.

```yaml
spec:
  objectives:
    # A file exists at a path
    - id: create-directory
      description: Create a directory called mywork in your home directory
      points: 10
      triggers:
        - type: file_exists
          target: linux-vm
          match:
            path: /home/labadmin/mywork

    # A file contains a literal string
    - id: write-content
      description: Write 'Hello, Kootenai!' into notes.txt
      points: 10
      triggers:
        - type: file_content
          target: linux-vm
          match:
            path: /home/labadmin/mywork/notes.txt
            contains: "Hello, Kootenai!"

    # A file matches a regular expression
    - id: enable-forwarding
      description: Enable IP forwarding in sysctl.conf
      points: 10
      triggers:
        - type: file_content
          target: linux-vm
          match:
            path: /etc/sysctl.conf
            regex: "^net\\.ipv4\\.ip_forward\\s*=\\s*1"

    # A package is installed
    - id: install-nginx
      description: Install the Nginx web server
      points: 10
      triggers:
        - type: package
          target: linux-vm
          match:
            package: nginx
            state: installed

    # A service is running
    - id: start-nginx
      description: Start the Nginx service
      points: 10
      triggers:
        - type: service
          target: linux-vm
          match:
            name: nginx
            state: active

    # A command was run
    - id: check-disk-space
      description: Check available disk space
      points: 10
      triggers:
        - type: command_executed
          target: linux-vm
          match:
            pattern: "df"

    # A user account was created
    - id: create-user
      description: Create a user account named labuser
      points: 10
      triggers:
        - type: user_created
          target: linux-vm
          match:
            username: labuser

    # Permissions were set on a file
    - id: make-executable
      description: Make myscript.sh executable
      points: 10
      triggers:
        - type: permission_changed
          target: linux-vm
          match:
            path: /home/labadmin/myscript.sh
            permission: "755"

    # A script decides, by exit code. It runs on the API server, so reach
    # into the VM over SSH using the ${VM_IP} placeholder.
    - id: forwarding-actually-on
      description: Verify IP forwarding is live in the kernel
      points: 10
      triggers:
        - type: active_check
          target: linux-vm
          script: |
            #!/bin/bash
            ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 "${VM_IP}" \
              'test "$(cat /proc/sys/net/ipv4/ip_forward)" = 1'

    # Instructor-verified, no automatic detection
    - id: demo-to-instructor
      description: Demonstrate your configuration to the instructor
      points: 10
      triggers:
        - type: custom
          target: linux-vm
```

An objective may carry several triggers, as `enable-ip-forwarding` does in
`templates/networking/basic-routing.yaml` — a `file_content` check on the
config file plus an `active_check` confirming the running kernel agrees.

## Variable interpolation

Substitution is shell-style, not Go-template-style. The loader matches
`${NAME}` and `$NAME` — identifiers only, no dotted paths and no double-brace
expressions.

Variables are declared at `metadata.variables`, not under `spec`:

```yaml
apiVersion: v1
kind: LabTemplate
metadata:
  name: base-linux-lab
  description: Base template for Linux labs, with common VM configuration.
  duration: 60m
  difficulty: beginner
  version: "1.0.0"
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

Three variables are always defined and can be overridden per template:

| Variable | Default |
|----------|---------|
| `LAB_SUBNET` | `10.0.0.0/24` |
| `LAB_GATEWAY` | `10.0.0.1` |
| `LAB_DNS` | `10.0.0.1` |

There are no pod, user, or timestamp variables. An unrecognised `$NAME` is left
in place verbatim rather than replaced with an empty string.

`${VM_IP}` and `${TARGET}` are a separate mechanism entirely — they are
substituted by the active-check runner into `active_check` scripts, not by the
loader, and they are not available anywhere else. See
[Where an active check actually runs](#where-an-active-check-actually-runs).

Substitution is applied to a fixed set of fields, not to the whole document.
`VariableProcessor.Process` walks exactly these:

- `metadata.name` and `metadata.description`
- `vms[].name` and `vms[].template`
- `network.segments[].subnet` and `.gateway`
- `objectives[].id`, `.description`, and `.hint`
- `objectives[].triggers[].target`

A `${VAR}` anywhere else — in a VM's static `ip`, in a trigger's `match.path`,
anywhere under `instructions` — survives into the database unexpanded.

> **Variables and validation order**
>
> The loader validates before it substitutes: `loadFile` runs the validator and
> only then calls `processVariables`. A template whose `subnet` is
> `${LAB_SUBNET}` therefore logs a CIDR warning at load even though the
> resolved value is a perfectly good subnet.
>
> `labctl lab validate` does not check address formats at all, so it reports
> `templates/examples/base-linux-lab.yaml` as valid.

## Template inheritance

A template with `metadata.extends` is a fragment: it names a parent by
`metadata.name` and is merged onto it. The merge is field by field:

- `metadata` — the child's `name` wins; `description`, `duration`,
  `difficulty`, `version`, and `author` fall back to the parent wherever the
  child leaves them empty; `tags` are unioned.
- `spec.vms` — merged by name. A child VM whose name matches a parent's
  replaces it; any other is added.
- `spec.network.segments` — merged by name the same way. If the child declares
  no segments at all, the parent's are kept whole.
- `spec.checkpoints` — the child's config replaces the parent's outright when
  present.
- `spec.objectives` — the child's are appended to the parent's, never
  replacing them.

Variable substitution runs on each template before the merge, so a child's
`variables` apply only to the child's own fields; they do not retroactively
change anything it inherits.

Two fields do not survive the merge at all: the merged result carries neither
`metadata.instructions` nor `metadata.variables`. A lab that needs a
student-facing walkthrough should be a standalone template rather than a child.

```yaml
apiVersion: v1
kind: LabTemplate
metadata:
  name: network-security-intro
  description: Introduction to network security, extending base-linux-lab.
  extends: base-linux-lab
  duration: 120m
  difficulty: intermediate
  version: "1.0.0"
  tags:
    - security
    - networking
  variables:
    LAB_SUBNET: "192.168.100.0/24"
    LAB_GATEWAY: "192.168.100.1"
spec:
  # student-vm is inherited from the parent; this VM is added to it.
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
```

Because a fragment is incomplete on its own, `spec.network` and `spec.vms` are
not required of it, its VMs may attach to the parent's segments, and its
triggers may target the parent's VMs. Python knows this and skips the
cross-reference checks for a fragment.

Go does not. Its loader validates before resolving inheritance, and the
validator has no `extends` awareness, so a child produces spurious complaints:
the server-side validator reports the inherited VM as an undefined trigger
target, and would also demand at least one VM from a child that declares none.
`labctl lab validate` adds a complaint about the inherited network segment.
All of these are warnings — the template loads and merges correctly.

## Fields the parsers ignore

An earlier revision of this page documented a different set of field names.
Templates written against it parse without error but lose data, because both
parsers discard unknown keys silently. If you have such a template, this is the
mapping:

| Was documented as | Use instead | Notes |
|---|---|---|
| `metadata.estimatedMinutes: 60` | `metadata.duration: "60m"` | |
| `metadata.maxPoints` | — | Derived: the sum of `objectives[].points`. |
| `metadata.passThreshold` | `spec.checkpoints.pass_threshold` | |
| `metadata.prerequisites` | `metadata.instructions.prerequisites` | |
| `spec.instructions` | `metadata.instructions` | |
| `spec.networks` (a list) | `spec.network.segments` | |
| `spec.platformOptions` | — | Not implemented. |
| `spec.snapshots` | `vms[].snapshots` | Snapshots are per-VM. |
| `spec.checkpoints` (a list of graded items) | `spec.objectives` | `spec.checkpoints` is the config object. |
| VM `cpu` / `memory` / `disk` | `vms[].resources.cpu` / `.memory` / `.disk` | |
| VM `networks[].network` | `vms[].networks[].segment` | |
| VM `autoStart` | `vms[].startOnCreate` | |
| VM `wazuhAgent` as an object | `wazuhAgent: true` plus `agentConfig` | |
| VM `displayName`, `description`, `startOrder`, `startDelay`, `credentials`, `sshKey` | — | Not implemented. |
| Segment `vlan: auto` | an integer, or omit the field | |
| Segment `dhcp` as an object | `dhcp: true` or `dhcp: false` | |
| Segment `displayName`, `isolated`, `internet` | — | Not implemented. Pods are isolated by platform configuration, not per-segment. |
| Objective `name` | `description` | |
| Objective `dependsOn` | `depends_on` | |
| Objective `hints` as strings | objects with `level` / `text` / `penalty` | |
| Objective `successMessage`, `failureHint`, `partialCredit`, `partialThreshold` | — | Not implemented. |
| `trigger:` (singular, with `params:`) | `triggers:` (a list, with `match:`) | |
| Trigger `type: manual` | `type: custom` | |
| Trigger `params.vm` | `target` | |
| Trigger `params.host` | `match.destination` | |
| Trigger `params.command` | `match.pattern` | |
| Trigger `params.service` plus `params.enabled` | `match.name` plus `match.state` | |
| Trigger `params.sourceVm` | — | Not implemented. A trigger is evaluated on one `target`. |
| Instruction step `hints` | — | Hints belong to objectives. |
| `spec.variables` with double-brace paths | `metadata.variables` with `${NAME}` | See above. |

Two more keys appear in shipped templates and are also discarded:
`metadata.category` (in 28 of them) and `vms[].additionalDisks` (in one).
Neither is on `LabMetadata`/`VMSpec` in Go or on the Python models. In the
`category` case there *is* a `category` column on the database record, but
nothing maps the YAML key onto it, so the value is lost between the file and
the table.

## Validating a template

```bash
# Go — build labctl, then validate a file
cd api && go build -o bin/labctl ./cmd/labctl
./bin/labctl lab validate ../templates/examples/simple-linux-intro.yaml
```

```bash
# Python — the stricter parser, and the one CI runs
cd python && pip install -r requirements.txt
python -c "
from lab_templates.parser import parse_template_file
parse_template_file('../templates/examples/simple-linux-intro.yaml')
print('ok')
"
```

CI parses every lab template under `templates/` with the Python parser, so that
command is the closest local equivalent to the gate a change has to pass.

There is no `labctl template` command group and no `mage template:` namespace.
`labctl` accepts `serve`, `migrate`, `version`, `lab`, `pod`, `snapshot`, and
`session`; validation lives under `lab`.

Note that `labctl lab validate` runs a different set of checks from the
server's loader. It verifies required fields, VM and segment uniqueness,
network and dependency references, trigger targets, and that `cpu` and `memory`
are positive. It does **not** check the metadata format rules (name pattern,
duration format, difficulty enum), address formats, or the resource ceilings —
those belong to the server-side validator. A template that passes
`labctl lab validate` can still produce warnings at load, and the reverse is
also possible.

## Related documentation

- [Example Templates](./examples.md) — complete, validated templates to start from
- [Wazuh Integration](../admin/wazuh-integration.md) — how file, command, and package events reach the checkpoint evaluator
- [Template Creation Guide](./template-creation-guide.md) — building the Proxmox VM images that `vms[].template` refers to
