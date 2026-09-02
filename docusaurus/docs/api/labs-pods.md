---
title: Labs & Pods API
description: API endpoints for lab templates and pod management
tags:
  - api
  - labs
  - pods
  - lifecycle
---

# Labs & Pods API

This section covers API endpoints for managing lab templates and pod instances.

## Labs Overview

**Lab templates** define the structure of a virtual environment. **Pods** are instances of these templates assigned to users.

```mermaid
flowchart LR
    T[Lab Template] -->|instantiate| P1[Pod 1]
    T -->|instantiate| P2[Pod 2]
    T -->|instantiate| P3[Pod 3]
```

---

## Lab Endpoints

### List Labs

<span class="api-method get">GET</span> `/api/v1/labs`

Get all available lab templates.

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `platform` | string | Filter by platform (proxmox, cloudstack) |
| `active` | boolean | Filter by active status (default: true) |
| `difficulty` | string | Filter by difficulty level |

**Response (200 OK):**

```json
{
  "labs": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "name": "Linux Foundations",
      "description": "Introduction to Linux commands and system administration",
      "version": "1.0.0",
      "platform": "proxmox",
      "durationMinutes": 60,
      "difficulty": "beginner",
      "maxPoints": 100,
      "passThreshold": 70,
      "isActive": true,
      "createdAt": "2025-01-01T00:00:00Z"
    }
  ],
  "count": 1
}
```

---

### Get Lab

<span class="api-method get">GET</span> `/api/v1/labs/{labId}`

Get a specific lab template.

**Path Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `labId` | uuid | Lab template ID |

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `include_spec` | boolean | Include full YAML spec (default: false) |

**Response (200 OK):**

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "Linux Foundations",
  "description": "Introduction to Linux commands",
  "version": "1.0.0",
  "platform": "proxmox",
  "durationMinutes": 60,
  "difficulty": "beginner",
  "maxPoints": 100,
  "passThreshold": 70,
  "isActive": true,
  "spec": {
    "vms": [...],
    "networks": [...]
  },
  "checkpoints": [
    {
      "id": "create-user",
      "name": "Create lab user",
      "description": "Create a user named 'labuser'",
      "points": 10
    }
  ]
}
```

---

### Get Lab Instructions

<span class="api-method get">GET</span> `/api/v1/labs/{labId}/instructions`

Get detailed instructions for a lab.

**Response (200 OK):**

```json
{
  "labId": "550e8400-e29b-41d4-a716-446655440000",
  "labName": "Linux Foundations",
  "overview": "In this lab, you will learn fundamental Linux commands...",
  "learningObjectives": [
    "Navigate the Linux filesystem",
    "Manage users and permissions",
    "Install and configure software"
  ],
  "prerequisites": [
    "Basic computer skills",
    "Understanding of command line concepts"
  ],
  "steps": [
    {
      "title": "Step 1: Connect to the VM",
      "content": "Use the console to connect to your Linux VM...",
      "hints": ["Use the default credentials", "Try the whoami command"]
    }
  ],
  "summary": "You learned how to...",
  "tips": ["Use tab completion", "Check command history with 'history'"],
  "resources": [
    {
      "title": "Linux Command Reference",
      "url": "https://linux.die.net/man/"
    }
  ]
}
```

---

### Create Lab (Admin)

<span class="api-method post">POST</span> `/api/v1/labs`

Create a new lab template. Requires admin or instructor role.

**Request Body:**

```json
{
  "name": "Network Security Basics",
  "description": "Learn firewall configuration and network security",
  "version": "1.0.0",
  "platform": "proxmox",
  "durationMinutes": 90,
  "difficulty": "intermediate",
  "maxPoints": 100,
  "passThreshold": 70,
  "spec": "apiVersion: v1\nkind: LabTemplate\n...",
  "isActive": true,
  "visibility": "global"
}
```

**Response (201 Created):**

```json
{
  "id": "new-lab-uuid",
  "name": "Network Security Basics",
  "slug": "network-security-basics",
  "createdAt": "2025-01-15T10:00:00Z"
}
```

---

### Update Lab (Admin)

<span class="api-method put">PUT</span> `/api/v1/labs/{labId}`

Update an existing lab template.

**Request Body:**

```json
{
  "name": "Updated Lab Name",
  "description": "Updated description",
  "isActive": false
}
```

---

### Delete Lab (Admin)

<span class="api-method delete">DELETE</span> `/api/v1/labs/{labId}`

Delete a lab template. Cannot delete if pods exist.

**Response (200 OK):**

```json
{
  "message": "Lab template deleted successfully"
}
```

---

## Pod Endpoints

### List Pods

<span class="api-method get">GET</span> `/api/v1/pods`

Get all pods for the current user.

**Response (200 OK):**

```json
{
  "pods": [
    {
      "id": "pod-abc123",
      "name": "linux-foundations-abc123",
      "labTemplateId": "lab-uuid",
      "labName": "Linux Foundations",
      "status": "running",
      "platform": "proxmox",
      "vms": [
        {
          "name": "student-vm",
          "vmid": 100,
          "status": "running",
          "ip": "10.0.100.10"
        }
      ],
      "createdAt": "2025-01-15T10:00:00Z",
      "expiresAt": "2025-01-15T14:00:00Z"
    }
  ]
}
```

---

### Create Pod

<span class="api-method post">POST</span> `/api/v1/pods`

Create a new pod from a lab template.

**Request Body:**

```json
{
  "labTemplateId": "550e8400-e29b-41d4-a716-446655440000",
  "name": "my-linux-lab"
}
```

**Response (201 Created):**

```json
{
  "id": "pod-xyz789",
  "name": "my-linux-lab",
  "status": "provisioning",
  "createdAt": "2025-01-15T10:00:00Z"
}
```

!!! note "Async Provisioning"
    Pod creation is asynchronous. Poll the pod status or use WebSocket for updates.

---

### Get Pod

<span class="api-method get">GET</span> `/api/v1/pods/{podId}`

Get detailed information about a pod.

**Response (200 OK):**

```json
{
  "id": "pod-xyz789",
  "name": "my-linux-lab",
  "labTemplateId": "lab-uuid",
  "labName": "Linux Foundations",
  "status": "running",
  "platform": "proxmox",
  "vms": [
    {
      "name": "student-vm",
      "vmid": 100,
      "status": "running",
      "ip": "10.0.100.10",
      "memory": 2048,
      "cores": 2,
      "template": "ubuntu-22.04"
    }
  ],
  "network": {
    "vlan": 100,
    "subnet": "10.0.100.0/24",
    "gateway": "10.0.100.1"
  },
  "createdAt": "2025-01-15T10:00:00Z",
  "expiresAt": "2025-01-15T14:00:00Z"
}
```

---

### Delete Pod

<span class="api-method delete">DELETE</span> `/api/v1/pods/{podId}`

Delete a pod and all its VMs.

**Response (200 OK):**

```json
{
  "message": "Pod deletion initiated",
  "status": "destroying"
}
```

---

### Pod Lifecycle Operations

#### Start Pod

<span class="api-method post">POST</span> `/api/v1/pods/{podId}/start`

Start all VMs in the pod.

#### Stop Pod

<span class="api-method post">POST</span> `/api/v1/pods/{podId}/stop`

Stop all VMs in the pod.

#### Reset Pod

<span class="api-method post">POST</span> `/api/v1/pods/{podId}/reset`

Reset pod to initial snapshot state.

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `snapshot` | string | Snapshot name (default: "initial") |

---

### VM Operations

#### Start VM

<span class="api-method post">POST</span> `/api/v1/pods/{podId}/vms/{vmName}/start`

Start a specific VM.

#### Stop VM

<span class="api-method post">POST</span> `/api/v1/pods/{podId}/vms/{vmName}/stop`

Stop a specific VM.

#### Reset VM

<span class="api-method post">POST</span> `/api/v1/pods/{podId}/vms/{vmName}/reset`

Reset a specific VM to snapshot.

---

### VM Console

<span class="api-method get">GET</span> `/api/v1/pods/{podId}/vms/{vmName}/console`

Get VNC console connection information.

**Response (200 OK):**

```json
{
  "type": "vnc",
  "host": "proxmox.local",
  "port": 5900,
  "ticket": "vnc-ticket-abc123",
  "websocketUrl": "wss://api.example.com/api/v1/pods/pod-xyz/vms/student-vm/vnc"
}
```

#### WebSocket VNC Proxy

<span class="api-method get">GET</span> `/api/v1/pods/{podId}/vms/{vmName}/vnc`

WebSocket endpoint for VNC proxy. Use with noVNC client.

---

### Pod Topology

<span class="api-method get">GET</span> `/api/v1/pods/{podId}/topology`

Get network topology visualization data.

**Response (200 OK):**

```json
{
  "podId": "pod-xyz789",
  "labTemplate": "Linux Foundations",
  "segments": [
    {
      "name": "internal",
      "vlan": 100,
      "subnet": "10.0.100.0/24",
      "gateway": "10.0.100.1",
      "dhcp": true
    }
  ],
  "vms": [
    {
      "name": "student-vm",
      "platformId": "vm-100",
      "status": "running",
      "ipAddress": "10.0.100.10",
      "template": "ubuntu-22.04",
      "networks": [
        {
          "segment": "internal",
          "ip": "10.0.100.10"
        }
      ],
      "resources": {
        "cpu": 2,
        "memory": 2048,
        "disk": 32
      }
    }
  ]
}
```

---

## Error Responses

| Status | Error | Meaning |
|--------|-------|---------|
| 400 | `invalid_request` | Bad request parameters |
| 401 | `unauthorized` | Authentication required |
| 403 | `forbidden` | Not allowed for this resource |
| 404 | `not_found` | Lab or pod not found |
| 409 | `conflict` | Pod already exists or in use |
| 503 | `service_unavailable` | Proxmox unavailable |
