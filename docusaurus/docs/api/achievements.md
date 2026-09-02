---
title: Achievements API
description: API endpoints for achievements, badges, and leaderboards
tags:
  - api
  - achievements
  - badges
  - gamification
---

# Achievements API

This section covers API endpoints for the achievement and gamification system.

## Overview

The achievement system rewards users for:

- Completing labs
- Perfect scores
- Streaks and consistency
- Speed challenges
- Special accomplishments

## Achievement Tiers

| Tier | Points | Color |
|------|--------|-------|
| Bronze | 10 | Copper |
| Silver | 25 | Silver |
| Gold | 50 | Gold |
| Platinum | 100 | Platinum |

---

## Achievement Endpoints

### List All Achievements

<span class="api-method get">GET</span> `/api/v1/achievements`

Get all available achievements.

**Response (200 OK):**

```json
{
  "achievements": [
    {
      "id": "first-lab",
      "name": "First Steps",
      "description": "Complete your first lab",
      "tier": "bronze",
      "points": 10,
      "icon": "pi-star",
      "category": "milestones",
      "criteria": {
        "type": "lab_completion",
        "count": 1
      }
    },
    {
      "id": "perfect-score",
      "name": "Perfectionist",
      "description": "Get 100% on any lab",
      "tier": "silver",
      "points": 25,
      "icon": "pi-check-circle",
      "category": "excellence",
      "criteria": {
        "type": "perfect_score",
        "count": 1
      }
    },
    {
      "id": "speed-demon",
      "name": "Speed Demon",
      "description": "Complete a lab in under 30 minutes",
      "tier": "silver",
      "points": 25,
      "icon": "pi-bolt",
      "category": "speed",
      "criteria": {
        "type": "time_based",
        "maxMinutes": 30
      }
    }
  ]
}
```

---

### Get Achievement

<span class="api-method get">GET</span> `/api/v1/achievements/{achievementId}`

Get details of a specific achievement.

**Response (200 OK):**

```json
{
  "id": "lab-master",
  "name": "Lab Master",
  "description": "Complete 10 labs with a passing grade",
  "tier": "gold",
  "points": 50,
  "icon": "pi-trophy",
  "category": "milestones",
  "criteria": {
    "type": "lab_completion",
    "count": 10,
    "requirePass": true
  },
  "earnedCount": 156,
  "rarity": "common"
}
```

---

### Get Recent Achievements

<span class="api-method get">GET</span> `/api/v1/achievements/recent`

Get recently earned achievements across all users (leaderboard style).

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `limit` | int | Number of entries (default: 20) |

**Response (200 OK):**

```json
{
  "achievements": [
    {
      "userId": "user-uuid-1",
      "displayName": "Jane Doe",
      "achievement": {
        "id": "speed-demon",
        "name": "Speed Demon",
        "tier": "silver",
        "icon": "pi-bolt"
      },
      "earnedAt": "2025-01-15T10:30:00Z"
    },
    {
      "userId": "user-uuid-2",
      "displayName": "John Smith",
      "achievement": {
        "id": "first-lab",
        "name": "First Steps",
        "tier": "bronze",
        "icon": "pi-star"
      },
      "earnedAt": "2025-01-15T10:25:00Z"
    }
  ]
}
```

---

## User Achievement Endpoints

### Get User Achievements

<span class="api-method get">GET</span> `/api/v1/users/{userId}/achievements`

Get all achievements for a specific user.

**Response (200 OK):**

```json
{
  "earned": [
    {
      "id": "ua-123",
      "achievementId": "first-lab",
      "name": "First Steps",
      "description": "Complete your first lab",
      "tier": "bronze",
      "points": 10,
      "icon": "pi-star",
      "earnedAt": "2025-01-10T14:30:00Z"
    },
    {
      "id": "ua-124",
      "achievementId": "perfect-score",
      "name": "Perfectionist",
      "tier": "silver",
      "points": 25,
      "icon": "pi-check-circle",
      "earnedAt": "2025-01-12T16:45:00Z"
    }
  ],
  "progress": [
    {
      "achievementId": "lab-master",
      "name": "Lab Master",
      "description": "Complete 10 labs with a passing grade",
      "tier": "gold",
      "points": 50,
      "progress": 5,
      "required": 10,
      "percentComplete": 50
    },
    {
      "achievementId": "streak-week",
      "name": "Weekly Warrior",
      "description": "Complete labs 7 days in a row",
      "tier": "silver",
      "points": 25,
      "progress": 3,
      "required": 7,
      "percentComplete": 43
    }
  ]
}
```

---

### Get Achievement Summary

<span class="api-method get">GET</span> `/api/v1/users/{userId}/achievements/summary`

Get summary statistics for a user's achievements.

**Response (200 OK):**

```json
{
  "totalEarned": 5,
  "totalPoints": 120,
  "totalAvailable": 25,
  "byTier": {
    "bronze": 3,
    "silver": 1,
    "gold": 1,
    "platinum": 0
  },
  "byCategory": {
    "milestones": 2,
    "excellence": 1,
    "speed": 1,
    "streaks": 1
  },
  "recentAchievements": [
    {
      "id": "ua-124",
      "name": "Perfectionist",
      "tier": "silver",
      "earnedAt": "2025-01-12T16:45:00Z"
    }
  ],
  "nextMilestones": [
    {
      "achievementId": "lab-master",
      "name": "Lab Master",
      "progress": 50,
      "pointsWorth": 50
    }
  ]
}
```

---

## Leaderboard

### Get Points Leaderboard

<span class="api-method get">GET</span> `/api/v1/leaderboard`

Get the global points leaderboard.

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `limit` | int | Number of entries (default: 10) |
| `teamId` | uuid | Filter by team (optional) |

**Response (200 OK):**

```json
{
  "entries": [
    {
      "rank": 1,
      "userId": "user-uuid-1",
      "displayName": "Jane Doe",
      "totalPoints": 1500,
      "achievementCount": 15,
      "labsCompleted": 25
    },
    {
      "rank": 2,
      "userId": "user-uuid-2",
      "displayName": "John Smith",
      "totalPoints": 1200,
      "achievementCount": 12,
      "labsCompleted": 20
    }
  ],
  "currentUser": {
    "rank": 15,
    "totalPoints": 450
  },
  "totalUsers": 150
}
```

---

## Achievement Categories

| Category | Description | Examples |
|----------|-------------|----------|
| `milestones` | Completion milestones | First lab, 10 labs, 50 labs |
| `excellence` | Quality achievements | Perfect score, high streak |
| `speed` | Speed challenges | Under 30 min, under 15 min |
| `streaks` | Consistency | 3-day streak, 7-day streak |
| `specialty` | Domain-specific | Security expert, Network pro |

---

## Achievement Criteria Types

| Type | Description | Parameters |
|------|-------------|------------|
| `lab_completion` | Complete N labs | count, requirePass |
| `perfect_score` | Get 100% on N labs | count |
| `time_based` | Complete in time limit | maxMinutes |
| `streak` | Consecutive days | days |
| `category_master` | Complete category labs | category, count |
| `first_time` | First accomplishment | action |

---

## WebSocket Events

Achievements emit events via WebSocket:

```json
{
  "type": "achievement.earned",
  "timestamp": "2025-01-15T11:00:05Z",
  "data": {
    "achievementId": "first-lab",
    "name": "First Steps",
    "description": "Complete your first lab",
    "tier": "bronze",
    "points": 10,
    "icon": "pi-star"
  }
}
```

See [WebSocket Events](websocket.md) for subscription details.

---

## Error Responses

| Status | Error | Meaning |
|--------|-------|---------|
| 401 | `unauthorized` | Auth required |
| 403 | `forbidden` | Cannot view other user's achievements |
| 404 | `not_found` | Achievement or user not found |
