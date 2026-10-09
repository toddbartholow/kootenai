---
title: AI Classroom Simulation
description: Simulate an entire class of AI students with distinct personalities completing labs and interacting with Canvas LMS
tags:
  - enterprise
  - simulation
  - ai
  - classroom
---

# AI Classroom Simulation

:::info Enterprise Feature
AI Classroom Simulation requires **Kootenai Enterprise Edition**. Community Edition users will see a 403 error with upgrade information.
:::

The AI Classroom Simulation feature creates a realistic virtual class of AI-driven students, each with a distinct personality, who progress through learning pathways, complete labs on VMs, and interact with Canvas LMS (assignments, discussions, quizzes).

## Overview

Use classroom simulation to:

- **Demo the platform** with realistic student activity before a live rollout
- **Load-test** your lab infrastructure with concurrent student sessions
- **Train instructors** on grading workflows with realistic submissions
- **Validate Canvas integration** with automated assignment/discussion/quiz submissions

## Architecture

```
┌──────────────────────────────────────────────────────────┐
│                  Classroom Runner                         │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐                  │
│  │Student 1│  │Student 2│  │Student N│  (goroutine each) │
│  │HP       │  │Struggle │  │IndyPro  │                   │
│  └────┬────┘  └────┬────┘  └────┬────┘                  │
│       │            │            │                         │
│       ▼            ▼            ▼                         │
│  ┌─────────────────────────────────────┐                 │
│  │         Behavior Engine             │                 │
│  │  Energy / Stress / Quality / Timing │                 │
│  └──────────────┬──────────────────────┘                 │
│                 │                                         │
│    ┌────────────┼────────────┐                           │
│    ▼            ▼            ▼                            │
│ ┌──────┐  ┌─────────┐  ┌──────────┐                     │
│ │ LLM  │  │ Canvas  │  │ labtest  │                      │
│ │Client│  │  REST   │  │simulator │                      │
│ └──────┘  └─────────┘  └──────────┘                     │
└──────────────────────────────────────────────────────────┘
```

### Components

| Component | Purpose |
|-----------|---------|
| **Classroom Runner** | Goroutine-per-student orchestration with start/pause/stop |
| **Behavior Engine** | Per-student energy, stress, quality modifiers, skip decisions |
| **LLM Client** | Anthropic API for generating persona-appropriate content |
| **Canvas Client** | REST API for user creation, enrollment, submissions |
| **Lab Executor** | Invokes `labtest simulate` subprocess for VM SSH actions |
| **Scheduler** | Personality-based timing delays with speed multiplier |

## Personality Types

Each AI student is assigned one of three personality archetypes that govern all their behavior:

### High Performer

- **Traits**: Motivation 9, Conscientiousness 9, Confidence 8, Anxiety 3
- **Behavior**: Submits 1-2 days early, thorough assignments, insightful discussion posts
- **Quiz scores**: 85-95%
- **LLM temperature**: 0.4 (consistent, focused responses)
- **Error rate**: 5%

### Struggling Student

- **Traits**: Motivation 5, Conscientiousness 4, Confidence 3, Anxiety 8
- **Behavior**: Submits last minute, incomplete assignments, short/vague discussion posts
- **Quiz scores**: 55-70%
- **LLM temperature**: 0.8 (variable, uncertain responses)
- **Error rate**: 18%

### Industry Professional

- **Traits**: Motivation 7, Conscientiousness 8, Confidence 7, Anxiety 4
- **Behavior**: Submits on time (evenings), practical assignments, real-world examples in discussions
- **Quiz scores**: 75-85%
- **LLM temperature**: 0.5 (balanced, practical responses)
- **Error rate**: 8%

## Behavior Engine

The behavior engine models each student's runtime state:

- **Energy** (0.0-1.0): Decreases per activity, resets daily. Low energy increases delays and skip chance.
- **Stress** (0.0-1.0): Increases near deadlines, decreases after completions. High stress degrades quality.

**Quality formula**: `base_quality × (1 - stress × 0.2) × energy`

**Skip logic**: When energy < 0.3 and conscientiousness < 5, there's a 30% chance the student skips an activity.

## Getting Started

### 1. Create a Simulation

Navigate to **Simulation > Classroom** in the web UI, or use the API:

```bash
curl -X POST http://localhost:8080/api/v1/simulation/classrooms \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Fall 2026 - Intro to Cybersecurity",
    "pathwayId": "uuid-of-pathway",
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
  }'
```

This auto-generates 25 AI students with randomized names and personality-appropriate traits.

### 2. Start the Simulation

```bash
curl -X POST http://localhost:8080/api/v1/simulation/classrooms/{id}/start \
  -H "Authorization: Bearer $TOKEN"
```

Each student begins progressing through the pathway in their own goroutine, with personality-driven timing and quality.

### 3. Monitor Progress

Use the activity feed to watch students in real-time:

```bash
curl http://localhost:8080/api/v1/simulation/classrooms/{id}/activities?limit=20 \
  -H "Authorization: Bearer $TOKEN"
```

### 4. Pause or Stop

```bash
# Pause (can resume later)
curl -X POST http://localhost:8080/api/v1/simulation/classrooms/{id}/pause

# Stop (marks as completed)
curl -X POST http://localhost:8080/api/v1/simulation/classrooms/{id}/stop
```

## Configuration

### Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `ANTHROPIC_API_KEY` | Anthropic API key for LLM-generated content | No (falls back to canned responses) |
| `CLASSROOM_SIM_ENABLED` | Enable classroom simulation feature | No (default: true if enterprise) |
| `CANVAS_API_TOKEN` | Canvas admin token for user creation/enrollment | No (only for Canvas integration) |

### Speed Multiplier

The `speedMultiplier` config compresses all timing:

| Value | Effect |
|-------|--------|
| 1.0 | Real-time delays (seconds between activities) |
| 5.0 | 5x faster |
| 10.0 | 10x faster (good for demos) |

### Personality Mix

If `personalityMix` is omitted, students are distributed evenly across the three personality types. You can also specify exact counts:

```json
{
  "personalityMix": {
    "high_performer": 8,
    "struggling": 4,
    "industry_professional": 13
  }
}
```

## Database Tables

The simulation uses three tables (migration `039_classroom_simulation.sql`):

| Table | Purpose |
|-------|---------|
| `classroom_simulations` | Simulation metadata, status, config |
| `ai_students` | Per-student personality, traits, skills, runtime state |
| `ai_student_activities` | Activity log (lab starts, completions, submissions, posts) |

## Web UI

The Classroom Simulation view (`/simulation/classroom`) has three panels:

1. **Setup** — Name, pathway, student count, personality mix sliders, Canvas/VM toggles, speed multiplier
2. **Simulation List** — All simulations with status badges, click to view details
3. **Detail View** — Student grid with personality badges, action buttons (Start/Pause/Stop/Delete)

## LLM Integration

When an `ANTHROPIC_API_KEY` is configured, the LLM client generates persona-appropriate content:

- **Assignments**: System prompt includes personality traits, quality level, and tone
- **Discussions**: References existing posts, varies depth by personality
- **Quizzes**: Generates answers at the student's skill level (some intentionally wrong)

Without an API key, the system falls back to canned placeholder responses.

## Related Documentation

- [Enterprise Edition](../enterprise.md) — Feature gating and edition comparison
- [API Reference: Classroom Simulation](../api/classroom-simulation.md) — Full API endpoint reference
- [Database Migrations](../reference/migrations.md) — Migration 039 details
