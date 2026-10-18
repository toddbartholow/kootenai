# Creating Pods and Pathways

This guide explains how to create and manage pods (lab instances) and pathways (learning tracks) in the Kootenai platform.

## Overview

The Kootenai platform uses three core concepts:

- **Lab Template**: A YAML definition describing a lab's VMs, networks, and learning objectives
- **Pod**: A running instance of a lab template assigned to a user/student
- **Pathway**: A structured learning track containing modules with ordered lab assignments

```
Lab Template (YAML) → Pod (Running Instance) → Session (Progress Tracking)
                          ↑
Pathway → Module → ModuleLab ─┘ (references template)
```

## Prerequisites

Before creating pods or pathways:

1. **API Access**: Ensure the Kootenai API is running (default: `http://localhost:8080`)
2. **Authentication**: Obtain a valid JWT token (see [Authentication Guide](../api/authentication.md))
3. **Platform Configuration**: Verify Proxmox or CloudStack backend is configured
4. **Lab Templates**: Have templates loaded into the database

---

## Part 1: Creating Lab Templates

Before creating pods, you need lab templates. Templates can be loaded from YAML files or created via API.

### Template Structure

Lab templates follow this structure:

```yaml
apiVersion: v1
kind: LabTemplate
metadata:
  name: My Lab Name
  description: What this lab teaches
  duration: 60m
  difficulty: beginner  # beginner, intermediate, advanced
  tags:
    - networking
    - security
  version: "1.0"
  author: Your Name
spec:
  platform: proxmox  # proxmox, cloudstack, or any
  network:
    segments:
      - name: lab-network
        vlan: 100
        subnet: 10.10.0.0/24
  vms:
    - name: workstation
      template: ubuntu-22.04-desktop
      resources:
        cpu: 2
        memory: 4096
        disk: 40
      networks:
        - segment: lab-network
          ip: 10.10.0.10
      snapshots:
        - name: initial
          description: Clean state
      startOnCreate: true
  checkpoints:
    enabled: true
    pass_threshold: 70
  objectives:
    - id: obj-1
      description: Complete the first task
      points: 20
      triggers:
        - type: file_exists
          target: workstation
          match:
            path: /home/student/completed.txt
```

### Loading Templates from Files

Place YAML files in the `templates/` directory structure:

```
templates/
├── networking/
│   ├── 01-tcp-ip-basics.yaml
│   └── 02-subnetting.yaml
├── cybersecurity/
│   ├── 01-soc-analyst.yaml
│   └── 02-incident-response.yaml
└── cloud/
    └── 01-cloudstack-basics.yaml
```

Templates are automatically loaded when the API starts, or use the admin API:

```bash
# Reload templates from disk
curl -X POST http://localhost:8080/api/v1/admin/templates/reload \
  -H "Authorization: Bearer $TOKEN"
```

### Listing Available Templates

```bash
curl http://localhost:8080/api/v1/templates \
  -H "Authorization: Bearer $TOKEN"
```

Response:
```json
{
  "templates": [
    {
      "id": "uuid-1",
      "name": "SOC Analyst Fundamentals",
      "description": "Learn SOC analysis and SIEM basics",
      "difficulty": "beginner",
      "platform": "proxmox",
      "vmCount": 2,
      "duration": "90m",
      "tags": ["security", "soc", "siem"]
    }
  ]
}
```

---

## Part 2: Creating Pods

A pod is a running instance of a lab template. Each pod contains:
- Cloned VMs from the template
- Isolated network segments
- Snapshot state for reset capability

### Method 1: Synchronous Pod Creation

Creates a pod and waits for provisioning to complete.

**Endpoint**: `POST /api/v1/pods`

```bash
curl -X POST http://localhost:8080/api/v1/pods \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "labTemplate": "SOC Analyst Fundamentals",
    "owner": "student1"
  }'
```

**Request Body**:
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `labTemplate` | string | Yes | Template name or ID |
| `owner` | string | Yes | Username of pod owner |

**Response** (HTTP 201):
```json
{
  "id": "pod-soc-analyst-student1-abc123",
  "labTemplate": "SOC Analyst Fundamentals",
  "labTemplateId": "uuid-of-template",
  "platform": "proxmox",
  "owner": "student1",
  "ownerId": "user-uuid",
  "status": "running",
  "vms": [
    {
      "name": "analyst-workstation",
      "platformId": "100123",
      "platform": "proxmox",
      "node": "pve",
      "status": "running",
      "ip": "10.20.0.10"
    },
    {
      "name": "siem-server",
      "platformId": "100124",
      "platform": "proxmox",
      "node": "pve",
      "status": "running",
      "ip": "10.20.0.5"
    }
  ],
  "networks": [
    {
      "name": "soc-network",
      "vlan": 200,
      "subnet": "10.20.0.0/24"
    }
  ],
  "createdAt": "2024-01-15T10:30:00Z",
  "expiresAt": "2024-01-15T12:00:00Z"
}
```

### Method 2: Asynchronous Pod Creation

For long-running provisioning, use async creation to avoid timeout issues.

**Endpoint**: `POST /api/v1/pods/async`

```bash
curl -X POST http://localhost:8080/api/v1/pods/async \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "labTemplate": "Complex Network Lab",
    "owner": "student1"
  }'
```

**Response** (HTTP 202):
```json
{
  "podId": "pod-uuid",
  "requestId": "req-abc123",
  "status": "provisioning",
  "subject": "labs.pods.pod-uuid.provision"
}
```

**Monitor progress via NATS**:
Subscribe to the `subject` for real-time status updates:
```
labs.pods.`{podId}`.provision
```

Or poll the pod status:
```bash
curl http://localhost:8080/api/v1/pods.`{podId}` \
  -H "Authorization: Bearer $TOKEN"
```

### Listing User's Pods

```bash
curl http://localhost:8080/api/v1/pods \
  -H "Authorization: Bearer $TOKEN"
```

**Query Parameters**:
| Parameter | Description |
|-----------|-------------|
| `owner` | Filter by owner username |
| `status` | Filter by status (running, stopped, error) |
| `platform` | Filter by platform (proxmox, cloudstack) |

### Pod Lifecycle Operations

#### Start a Pod
```bash
curl -X POST http://localhost:8080/api/v1/pods.`{podId}`/start \
  -H "Authorization: Bearer $TOKEN"
```

#### Stop a Pod
```bash
curl -X POST http://localhost:8080/api/v1/pods.`{podId}`/stop \
  -H "Authorization: Bearer $TOKEN"
```

#### Reset Pod to Snapshot
```bash
curl -X POST http://localhost:8080/api/v1/pods.`{podId}`/reset \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "snapshot": "initial"
  }'
```

#### Delete a Pod
```bash
curl -X DELETE http://localhost:8080/api/v1/pods.`{podId}` \
  -H "Authorization: Bearer $TOKEN"
```

### Individual VM Control

Control specific VMs within a pod:

```bash
# Start VM
curl -X POST http://localhost:8080/api/v1/pods.`{podId}`/vms/{vmName}/start \
  -H "Authorization: Bearer $TOKEN"

# Stop VM
curl -X POST http://localhost:8080/api/v1/pods.`{podId}`/vms/{vmName}/stop \
  -H "Authorization: Bearer $TOKEN"

# Get VM Console (VNC)
curl http://localhost:8080/api/v1/pods.`{podId}`/vms/{vmName}/console?type=vnc \
  -H "Authorization: Bearer $TOKEN"
```

---

## Part 3: Creating Pathways

Pathways are learning tracks that organize labs into structured curricula.

### Pathway Structure

```
Pathway: "Security Fundamentals"
├── Module 1: "Linux Basics" (sequential unlock)
│   ├── Lab: Linux CLI Fundamentals (required)
│   └── Lab: File Permissions (required)
├── Module 2: "Network Basics" (requires Module 1)
│   ├── Lab: TCP/IP Fundamentals (required)
│   └── Lab: Wireshark Introduction (optional)
└── Module 3: "Security Tools" (requires Module 2)
    ├── Lab: SOC Analyst Fundamentals (required)
    └── Lab: Incident Response (required)
```

### Step 1: Create the Pathway

**Endpoint**: `POST /api/v1/pathways`

```bash
curl -X POST http://localhost:8080/api/v1/pathways \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Security Fundamentals",
    "slug": "security-fundamentals",
    "description": "A comprehensive introduction to cybersecurity concepts",
    "difficulty": "beginner",
    "visibility": "public"
  }'
```

**Request Body**:
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Display name |
| `slug` | string | Yes | URL-friendly identifier |
| `description` | string | No | Pathway description |
| `difficulty` | string | No | beginner/intermediate/advanced |
| `visibility` | string | No | public/private/global |

**Response** (HTTP 201):
```json
{
  "id": "pathway-uuid",
  "name": "Security Fundamentals",
  "slug": "security-fundamentals",
  "description": "A comprehensive introduction to cybersecurity concepts",
  "difficulty": "beginner",
  "status": "draft",
  "visibility": "public",
  "isFeatured": false,
  "moduleCount": 0,
  "labCount": 0,
  "totalPoints": 0,
  "totalDuration": 0,
  "createdAt": "2024-01-15T10:30:00Z"
}
```

### Step 2: Create Modules

Modules group related labs within a pathway.

**Endpoint**: `POST /api/v1/pathways/{pathwayId}/modules`

```bash
curl -X POST http://localhost:8080/api/v1/pathways/{pathwayId}/modules \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Linux Basics",
    "slug": "linux-basics",
    "displayOrder": 1,
    "unlockType": "always",
    "isActive": true
  }'
```

**Unlock Types**:
| Type | Behavior |
|------|----------|
| `always` | Module is always available |
| `sequential` | Requires previous module completed |
| `all_previous` | Requires all previous modules completed |
| `manual` | Requires instructor approval to unlock |

**Create additional modules**:
```bash
# Module 2: Requires Module 1
curl -X POST http://localhost:8080/api/v1/pathways/{pathwayId}/modules \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Network Basics",
    "slug": "network-basics",
    "displayOrder": 2,
    "unlockType": "sequential",
    "isActive": true
  }'

# Module 3: Requires Module 2
curl -X POST http://localhost:8080/api/v1/pathways/{pathwayId}/modules \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Security Tools",
    "slug": "security-tools",
    "displayOrder": 3,
    "unlockType": "sequential",
    "isActive": true
  }'
```

### Step 3: Add Labs to Modules

Associate lab templates with modules.

**Endpoint**: `POST /api/v1/modules/{moduleId}/labs`

```bash
# Add required lab to Module 1
curl -X POST http://localhost:8080/api/v1/modules/{moduleId}/labs \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "labTemplateId": "uuid-of-linux-cli-lab",
    "displayOrder": 1,
    "isRequired": true
  }'

# Add optional lab
curl -X POST http://localhost:8080/api/v1/modules/{moduleId}/labs \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "labTemplateId": "uuid-of-bonus-lab",
    "displayOrder": 2,
    "isRequired": false,
    "passThresholdOverride": 60
  }'
```

**Request Body**:
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `labTemplateId` | string | Yes | UUID of the lab template |
| `displayOrder` | int | No | Order within module |
| `isRequired` | bool | No | Must complete to finish module |
| `passThresholdOverride` | int | No | Override default passing score (0-100) |

### Step 4: Publish the Pathway

Pathways start in `draft` status. Publish when ready for students.

**Endpoint**: `POST /api/v1/pathways/{pathwayId}/publish`

```bash
curl -X POST http://localhost:8080/api/v1/pathways/{pathwayId}/publish \
  -H "Authorization: Bearer $TOKEN"
```

### Listing Pathways

```bash
# List all published pathways
curl http://localhost:8080/api/v1/pathways?status=published \
  -H "Authorization: Bearer $TOKEN"

# Get pathway with modules
curl http://localhost:8080/api/v1/pathways/{pathwayId} \
  -H "Authorization: Bearer $TOKEN"
```

### Get Pathway with Full Details

```bash
curl http://localhost:8080/api/v1/pathways/{pathwayId}?include=modules,labs \
  -H "Authorization: Bearer $TOKEN"
```

Response includes nested modules and labs:
```json
{
  "id": "pathway-uuid",
  "name": "Security Fundamentals",
  "status": "published",
  "modules": [
    {
      "id": "module-1-uuid",
      "name": "Linux Basics",
      "displayOrder": 1,
      "unlockType": "always",
      "labs": [
        {
          "id": "lab-1-uuid",
          "labTemplateId": "template-uuid",
          "labName": "Linux CLI Fundamentals",
          "displayOrder": 1,
          "isRequired": true,
          "labMaxPoints": 100
        }
      ]
    }
  ],
  "moduleCount": 3,
  "labCount": 6,
  "totalPoints": 600,
  "totalDuration": 360
}
```

---

## Part 4: Student Enrollment

Students enroll in pathways to track their progress.

### Enroll in a Pathway

**Endpoint**: `POST /api/v1/pathways/{pathwayId}/enroll`

```bash
curl -X POST http://localhost:8080/api/v1/pathways/{pathwayId}/enroll \
  -H "Authorization: Bearer $TOKEN"
```

**Response** (HTTP 201):
```json
{
  "id": "enrollment-uuid",
  "userId": "user-uuid",
  "pathwayId": "pathway-uuid",
  "status": "enrolled",
  "completedModules": 0,
  "totalModules": 3,
  "earnedPoints": 0,
  "maxPoints": 600,
  "enrolledAt": "2024-01-15T10:30:00Z"
}
```

### Check Progress

**Endpoint**: `GET /api/v1/enrollments/{enrollmentId}/progress`

```bash
curl http://localhost:8080/api/v1/enrollments/{enrollmentId}/progress \
  -H "Authorization: Bearer $TOKEN"
```

**Response**:
```json
{
  "enrollmentId": "enrollment-uuid",
  "status": "in_progress",
  "completedModules": 1,
  "totalModules": 3,
  "earnedPoints": 200,
  "maxPoints": 600,
  "percentComplete": 33,
  "moduleProgress": [
    {
      "moduleId": "module-1-uuid",
      "moduleName": "Linux Basics",
      "status": "completed",
      "earnedPoints": 200,
      "maxPoints": 200,
      "completedLabs": 2,
      "totalLabs": 2
    },
    {
      "moduleId": "module-2-uuid",
      "moduleName": "Network Basics",
      "status": "unlocked",
      "earnedPoints": 0,
      "maxPoints": 200,
      "completedLabs": 0,
      "totalLabs": 2
    },
    {
      "moduleId": "module-3-uuid",
      "moduleName": "Security Tools",
      "status": "locked",
      "earnedPoints": 0,
      "maxPoints": 200,
      "completedLabs": 0,
      "totalLabs": 2
    }
  ]
}
```

### Check Module Unlock Requirements

**Endpoint**: `GET /api/v1/enrollments/{enrollmentId}/modules/{moduleId}/unlock-requirements`

```bash
curl http://localhost:8080/api/v1/enrollments/{enrollmentId}/modules/{moduleId}/unlock-requirements \
  -H "Authorization: Bearer $TOKEN"
```

### Manual Module Unlock (Instructor)

**Endpoint**: `POST /api/v1/enrollments/{enrollmentId}/modules/{moduleId}/unlock`

```bash
curl -X POST http://localhost:8080/api/v1/enrollments/{enrollmentId}/modules/{moduleId}/unlock \
  -H "Authorization: Bearer $TOKEN"
```

---

## Part 5: Complete Workflow Example

Here's a complete example creating a pathway and having a student complete it:

### Admin: Create the Curriculum

```bash
# 1. Create pathway
PATHWAY=$(curl -s -X POST http://localhost:8080/api/v1/pathways \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Intro to Security",
    "slug": "intro-security",
    "difficulty": "beginner"
  }')
PATHWAY_ID=$(echo $PATHWAY | jq -r '.id')

# 2. Create first module
MODULE1=$(curl -s -X POST http://localhost:8080/api/v1/pathways/$PATHWAY_ID/modules \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Getting Started",
    "slug": "getting-started",
    "displayOrder": 1,
    "unlockType": "always"
  }')
MODULE1_ID=$(echo $MODULE1 | jq -r '.id')

# 3. Add lab to module
curl -X POST http://localhost:8080/api/v1/modules/$MODULE1_ID/labs \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "labTemplateId": "'$LAB_TEMPLATE_ID'",
    "isRequired": true
  }'

# 4. Publish pathway
curl -X POST http://localhost:8080/api/v1/pathways/$PATHWAY_ID/publish \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

### Student: Complete the Pathway

```bash
# 1. Enroll in pathway
ENROLLMENT=$(curl -s -X POST http://localhost:8080/api/v1/pathways/$PATHWAY_ID/enroll \
  -H "Authorization: Bearer $STUDENT_TOKEN")
ENROLLMENT_ID=$(echo $ENROLLMENT | jq -r '.id')

# 2. Launch lab pod
POD=$(curl -s -X POST http://localhost:8080/api/v1/pods \
  -H "Authorization: Bearer $STUDENT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "labTemplate": "'$LAB_TEMPLATE_ID'",
    "owner": "student1"
  }')
POD_ID=$(echo $POD | jq -r '.id')

# 3. Create session to track progress
SESSION=$(curl -s -X POST http://localhost:8080/api/v1/sessions \
  -H "Authorization: Bearer $STUDENT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "podId": "'$POD_ID'",
    "labTemplateId": "'$LAB_TEMPLATE_ID'"
  }')
SESSION_ID=$(echo $SESSION | jq -r '.id')

# 4. Work through lab... (student completes objectives)

# 5. Submit session for grading
curl -X POST http://localhost:8080/api/v1/sessions/$SESSION_ID/submit \
  -H "Authorization: Bearer $STUDENT_TOKEN"

# 6. Check pathway progress
curl http://localhost:8080/api/v1/enrollments/$ENROLLMENT_ID/progress \
  -H "Authorization: Bearer $STUDENT_TOKEN"

# 7. Clean up pod when done
curl -X DELETE http://localhost:8080/api/v1/pods/$POD_ID \
  -H "Authorization: Bearer $STUDENT_TOKEN"
```

---

## Troubleshooting

### Common Issues

| Issue | Cause | Solution |
|-------|-------|----------|
| Pod stuck in "provisioning" | Platform timeout | Check Proxmox/CloudStack connectivity |
| "Template not found" | Template not loaded | Reload templates or check template name |
| Module won't unlock | Prerequisites not met | Check previous module completion |
| VM won't start | Resource constraints | Check Proxmox cluster capacity |
| Console connection failed | Firewall blocking VNC | Open ports 5900-5999 |

### Checking Pod Status

```bash
# Get detailed pod status
curl http://localhost:8080/api/v1/pods.`{podId}` \
  -H "Authorization: Bearer $TOKEN"

# Check VM status
curl http://localhost:8080/api/v1/pods.`{podId}`/vms \
  -H "Authorization: Bearer $TOKEN"
```

### Logs

```bash
# API logs (on infra VM)
docker compose logs -f api

# Check for provisioning errors
docker compose logs api | grep -i error
```

### Force Pod Cleanup

If a pod is stuck, admin can force delete:

```bash
curl -X DELETE http://localhost:8080/api/v1/admin/pods.`{podId}`?force=true \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

---

## API Reference

For complete API documentation, see:
- [API Reference](../api/reference.md)
- [OpenAPI Specification](../api/openapi.md)

## Related Documentation

- [Lab Template Schema](../lab-templates/yaml-schema.md)
- [Production Deployment](../admin/production-deployment.md)
- [Canvas LTI Integration](../admin/canvas-lms-integration.md)
