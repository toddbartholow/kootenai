---
title: Glossary
description: Definitions of key terms used in Kootenai
tags:
  - reference
  - glossary
  - terminology
---

# Glossary

This glossary defines key terms and concepts used throughout the Kootenai platform.

## A

### Achievement
A milestone or accomplishment earned by completing specific criteria, such as finishing labs, earning points, or maintaining streaks. Achievements are displayed on user profiles and contribute to gamification.

### API (Application Programming Interface)
The programmatic interface for interacting with Kootenai. The REST API allows external systems and the web UI to manage labs, pods, sessions, and users.

### Assignment
A Canvas LMS task linked to a Kootenai lab template via LTI. When students launch an assignment, they receive a dedicated pod instance.

## C

### Canvas LMS
A learning management system (LMS) that integrates with Kootenai via LTI 1.3. Instructors create assignments that link to lab templates, and grades sync back automatically. **The LTI integration is gated off in this build** — see [Enterprise Edition](enterprise.md).

### Checkpoint
A specific objective within a lab that students must complete. Checkpoints are detected automatically through file changes, service states, or network conditions monitored by Wazuh agents.

### Clone
A copy of a VM template used to create pod instances. Clones can be full copies or linked clones (sharing base disk with copy-on-write).

### CloudStack
Apache CloudStack, an alternative virtualization platform supported by Kootenai for multi-tenant cloud infrastructure training scenarios.

## D

### Demo Mode
A development/testing mode where authentication is bypassed. Enabled via `AUTH_DEMO_MODE=true` environment variable. Only activates when no Authorization header is present.

### Dependency (Checkpoint)
A relationship where one checkpoint requires another to be completed first. Defined using `dependsOn` in lab templates.

## G

### Grade Passback
The process of sending student scores from Kootenai back to Canvas LMS automatically when checkpoints are completed or sessions end. Implemented via LTI AGS, but [gated off in this build](enterprise.md).

### Group
A collection of users who share access to a pod. Used for team-based labs where multiple students collaborate on the same environment.

## H

### Hint
Progressive help text provided for checkpoints. Students can reveal hints (usually at a point cost) when stuck on an objective.

## I

### Isolated Network
A virtual network segment that prevents communication between different pods. Ensures students can't interfere with each other's lab environments.

## J

### JWT (JSON Web Token)
The authentication token format used by Kootenai. Contains user identity and claims, signed by the server for verification.

## L

### Lab
A complete learning exercise including instructions, VMs, networks, and checkpoints. Labs are instantiated as pods for individual students or groups.

### Lab Session
See [Session](#session).

### Lab Template
A YAML definition file that describes a lab's topology, including VMs, networks, checkpoints, and instructions. Templates are used to create pod instances.

### Leaderboard
A ranking of users or teams based on points earned. Leaderboards can be scoped to specific labs, pathways, organizations, or globally.

### License
An organization-level entitlement that grants access to specific labs or pathways. Licenses can have seat limits and expiration dates.

### Linked Clone
A VM clone that shares the base disk with its template, using copy-on-write for changes. More storage-efficient than full clones but requires the template to remain intact.

### LTI (Learning Tools Interoperability)
A standard for integrating external tools with learning management systems. Kootenai supports LTI 1.3 for Canvas integration.

## M

### Mage
The Go-based build system used by Kootenai. Provides targets for building, testing, deploying, and managing the platform.

### Migration
A database schema change applied incrementally. Migrations are versioned and tracked in the `schema_migrations` table.

### Multi-tenancy
The architecture that allows multiple organizations to share a single Kootenai installation while maintaining data isolation.

## N

### NATS
A messaging system used for real-time communication between Kootenai components. Enables WebSocket updates and async job processing.

### Network (Lab)
A virtual network defined in a lab template. Networks can be isolated, connected to the internet, or configured with specific VLANs and subnets.

## O

### Objective
See [Checkpoint](#checkpoint).

### Organization
A top-level entity that groups users, teams, and resources. Organizations have their own licenses, settings, and data isolation.

## P

### Pathway
A structured sequence of labs designed to teach a specific skill set or curriculum. Pathways track progress across multiple labs and may award certificates upon completion.

### Platform
The virtualization backend (Proxmox or CloudStack) where pod VMs run.

### Pod
An instantiated lab environment for a student or group. Contains cloned VMs, isolated networks, and session state. Pods are the runtime unit of a lab.

### Points
The scoring unit for checkpoints and achievements. Each checkpoint awards points upon completion, contributing to total session and user scores.

### Proxmox VE
The primary virtualization platform for Kootenai. Provides VM management, snapshots, and network isolation.

## Q

### QEMU
The underlying virtualization technology used by Proxmox for running VMs.

## R

### Redis
An in-memory data store used for caching, rate limiting, and session management in Kootenai.

### Required Checkpoint
A checkpoint that must be completed to pass the lab, regardless of total points earned.

### Reset
Reverting a pod's VMs to a previous snapshot state. Allows students to recover from mistakes or start fresh.

## S

### Session
A time-bound period during which a student works on a lab. Sessions track progress, checkpoints completed, time spent, and points earned.

### Snapshot
A saved state of a VM at a specific point in time. Lab templates define named snapshots that students can reset to.

### Streak
Consecutive days of activity on the platform. Maintaining streaks can earn achievements and bonus points.

### Syscheck
The Wazuh component that monitors file system changes for checkpoint detection.

## T

### Team
A group of users within an organization who collaborate on labs. Teams can share pods and appear on leaderboards together.

### Template (Lab)
See [Lab Template](#lab-template).

### Template (VM)
A base VM image in Proxmox used to create clones for pods. VM templates include the operating system, software, and Wazuh agent.

### Topology
The arrangement of VMs and networks in a lab. Defined in the lab template's `spec` section.

### Trigger
The condition that marks a checkpoint as complete. Types include file existence, file content, service state, user creation, and network connectivity.

## U

### User
An individual account in Kootenai. Users can be students, instructors, or administrators with different permission levels.

## V

### Virtual Machine (VM)
A virtualized computer running within Proxmox or CloudStack. Labs consist of one or more VMs connected by virtual networks.

### VLAN (Virtual LAN)
A network isolation technique used to separate pod traffic. Each pod receives a unique VLAN ID.

### VNC (Virtual Network Computing)
The protocol used for console access to VMs. Students connect to VM consoles through the web UI via noVNC.

## W

### Wazuh
A security monitoring platform used by Kootenai for checkpoint detection. Wazuh agents in VMs report file changes, service states, and other events to the central manager.

### WebSocket
A bidirectional communication protocol used for real-time updates in the web UI. Pod status, checkpoint progress, and notifications are pushed via WebSocket.

## Y

### YAML
The file format used for lab templates. YAML (YAML Ain't Markup Language) provides a human-readable syntax for defining lab configurations.

---

## Acronyms

| Acronym | Full Form |
|---------|-----------|
| ADR | Architecture Decision Record |
| API | Application Programming Interface |
| CIDR | Classless Inter-Domain Routing |
| CLI | Command Line Interface |
| CPU | Central Processing Unit |
| CSP | Content Security Policy |
| DHCP | Dynamic Host Configuration Protocol |
| DNS | Domain Name System |
| E2E | End-to-End |
| HA | High Availability |
| HTTP | Hypertext Transfer Protocol |
| HTTPS | HTTP Secure |
| IP | Internet Protocol |
| JSON | JavaScript Object Notation |
| JWT | JSON Web Token |
| LMS | Learning Management System |
| LTI | Learning Tools Interoperability |
| NAT | Network Address Translation |
| NATS | Neural Autonomic Transport System |
| OIDC | OpenID Connect |
| OS | Operating System |
| OWASP | Open Web Application Security Project |
| RAM | Random Access Memory |
| REST | Representational State Transfer |
| RSA | Rivest-Shamir-Adleman (encryption) |
| SQL | Structured Query Language |
| SSH | Secure Shell |
| SSL | Secure Sockets Layer |
| TCP | Transmission Control Protocol |
| TLS | Transport Layer Security |
| UDP | User Datagram Protocol |
| UI | User Interface |
| URL | Uniform Resource Locator |
| UUID | Universally Unique Identifier |
| VLAN | Virtual Local Area Network |
| VM | Virtual Machine |
| VNC | Virtual Network Computing |
| VPN | Virtual Private Network |
| WS | WebSocket |
| YAML | YAML Ain't Markup Language |
