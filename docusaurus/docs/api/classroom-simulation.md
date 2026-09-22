---
title: Classroom Simulation API
description: API reference for AI classroom simulation endpoints
tags:
  - api
  - enterprise
  - simulation
---

# Classroom Simulation API

:::info Enterprise Feature
All classroom simulation endpoints require the `ai_classroom` enterprise feature. Requests without an enterprise license return `403 Forbidden`.
:::

**Base path**: `/api/v1/simulation/classrooms`

**Authentication**: All endpoints require a valid JWT token.

---

## Simulations

### POST /api/v1/simulation/classrooms

Create a new classroom simulation with auto-generated AI students.

**Request**:
```json
{
  "name": "Fall 2026 - Intro to Cybersecurity",
  "pathwayId": "uuid",
  "canvasCourseId": "12345",
  "config": {
    "studentCount": 25,
    "personalityMix": {
      "high_performer": 6,
      "struggling": 5,
      "industry_professional": 14
    },
    "enableCanvas": true,
    "enableVmLabs": true,
    "speedMultiplier": 5.0
  }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Simulation name |
| `pathwayId` | UUID | No | Learning pathway to follow |
| `canvasCourseId` | string | No | Canvas course ID for LMS integration |
| `config.studentCount` | int | No | Total students (default: 25, used if no mix) |
| `config.personalityMix` | object | No | Per-personality counts (overrides studentCount) |
| `config.enableCanvas` | bool | No | Enable Canvas LMS submissions |
| `config.enableVmLabs` | bool | No | Enable VM lab execution via labtest |
| `config.speedMultiplier` | float | No | Timing compression (default: 1.0) |

**Response** (201 Created):
```json
{
  "id": "uuid",
  "name": "Fall 2026 - Intro to Cybersecurity",
  "status": "pending",
  "pathwayId": "uuid",
  "config": { ... },
  "students": [
    {
      "id": "uuid",
      "simulationId": "uuid",
      "name": "Alex Chen",
      "personality": "high_performer",
      "traits": {
        "motivation": 9,
        "conscientiousness": 9,
        "confidence": 8,
        "anxiety": 3
      },
      "techSkills": {
        "linux": 8,
        "networking": 7,
        "security": 8,
        "scripting": 7,
        "cloud_ops": 6
      },
      "behavioralConfig": {
        "llm_temperature": 0.4,
        "quiz_score_min": 85,
        "quiz_score_max": 95,
        "assignment_quality": "thorough",
        "discussion_style": "insightful",
        "submit_timing": "early",
        "error_rate": 0.05
      },
      "state": {},
      "createdAt": "2026-01-26T12:00:00Z"
    }
  ],
  "createdAt": "2026-01-26T12:00:00Z",
  "updatedAt": "2026-01-26T12:00:00Z"
}
```

---

### GET /api/v1/simulation/classrooms

List all classroom simulations.

**Response** (200 OK):
```json
{
  "simulations": [
    {
      "id": "uuid",
      "name": "Fall 2026 - Intro to Cybersecurity",
      "status": "running",
      "pathwayId": "uuid",
      "config": { ... },
      "startedAt": "2026-01-26T12:05:00Z",
      "createdAt": "2026-01-26T12:00:00Z",
      "updatedAt": "2026-01-26T12:05:00Z"
    }
  ],
  "count": 1
}
```

---

### GET /api/v1/simulation/classrooms/\{id\}

Get simulation details including all students.

**Response** (200 OK): Same as create response, with current status and student states.

**Errors**:
- `404` — Simulation not found

---

### DELETE /api/v1/simulation/classrooms/\{id\}

Delete a simulation and all associated students and activities (cascading).

**Response** (200 OK):
```json
{
  "message": "simulation deleted",
  "id": "uuid"
}
```

---

## Simulation Control

### POST /api/v1/simulation/classrooms/\{id\}/start

Start a pending or paused simulation. Launches one goroutine per student.

**Response** (200 OK):
```json
{
  "message": "simulation started",
  "id": "uuid",
  "status": "running"
}
```

**Errors**:
- `500` — Simulation is already running/completed/failed

---

### POST /api/v1/simulation/classrooms/\{id\}/pause

Pause a running simulation. Students stop at their current activity.

**Response** (200 OK):
```json
{
  "message": "simulation paused",
  "id": "uuid",
  "status": "paused"
}
```

---

### POST /api/v1/simulation/classrooms/\{id\}/stop

Stop a running or paused simulation. Marks as completed.

**Response** (200 OK):
```json
{
  "message": "simulation stopped",
  "id": "uuid",
  "status": "completed"
}
```

---

## Activities & Students

### GET /api/v1/simulation/classrooms/\{id\}/activities

Get paginated activity feed for a simulation.

**Query Parameters**:

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | int | 50 | Results per page |
| `offset` | int | 0 | Pagination offset |

**Response** (200 OK):
```json
{
  "activities": [
    {
      "id": 42,
      "simulationId": "uuid",
      "studentId": "uuid",
      "activityType": "lab_complete",
      "targetName": "Linux Foundations Lab 1",
      "status": "completed",
      "score": 88,
      "maxScore": 100,
      "metadata": {},
      "createdAt": "2026-01-26T12:15:00Z"
    }
  ],
  "total": 150
}
```

**Activity Types**:

| Type | Description |
|------|-------------|
| `lab_start` | Student started a lab |
| `lab_complete` | Student completed a lab |
| `assignment_submit` | Canvas assignment submitted |
| `discussion_post` | Canvas discussion post created |
| `quiz_attempt` | Canvas quiz attempted |

---

### GET /api/v1/simulation/classrooms/\{id\}/students/\{studentId\}

Get detailed information about a specific AI student.

**Response** (200 OK):
```json
{
  "id": "uuid",
  "simulationId": "uuid",
  "name": "Jordan Patel",
  "personality": "struggling",
  "traits": {
    "motivation": 5,
    "conscientiousness": 4,
    "confidence": 3,
    "anxiety": 8
  },
  "techSkills": {
    "linux": 3,
    "networking": 2,
    "security": 2,
    "scripting": 2,
    "cloud_ops": 1
  },
  "behavioralConfig": {
    "llm_temperature": 0.8,
    "quiz_score_min": 55,
    "quiz_score_max": 70,
    "assignment_quality": "incomplete",
    "discussion_style": "vague",
    "submit_timing": "last_minute",
    "error_rate": 0.18
  },
  "state": {
    "energy": 0.45,
    "stress": 0.62
  },
  "createdAt": "2026-01-26T12:00:00Z"
}
```

---

## Simulation Status Values

| Status | Description |
|--------|-------------|
| `pending` | Created but not started |
| `running` | Actively executing student activities |
| `paused` | Stopped mid-execution, can resume |
| `completed` | All students finished or manually stopped |
| `failed` | Terminated due to error |

---

## Enterprise Gating

All endpoints return `403 Forbidden` when the `ai_classroom` feature is not enabled:

```json
{
  "error": "This feature requires Kootenai Enterprise Edition",
  "feature": "ai_classroom",
  "edition": "community"
}
```

---

## Related

- [Classroom Simulation Guide](../guides/classroom-simulation.md) — Setup guide with personality details
- [Enterprise Edition](../enterprise.md) — Feature comparison and gating
- [WebSocket API](websocket.md) — Real-time activity updates (Phase 4)
