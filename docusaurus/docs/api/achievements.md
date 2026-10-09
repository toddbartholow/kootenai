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

The achievement system rewards users for completing labs, perfect scores, streaks, speed and
pathway progress.

:::caution This page was substantially wrong before
Its criteria keys (`count`, `requirePass`, `maxMinutes`, `days`, `category`) do not exist on
`AchievementCriteria`, so any achievement authored against the old version of this page was
**permanently unearnable** — the evaluator never sees a field it recognises. It also listed
four tiers where the enum has five. Everything below is taken from
`api/internal/models/achievement.go` and `api/internal/server/achievement_handlers.go`.

The sibling page [Achievement System](../achievement-system.md) describes the same model and
is broadly accurate.
:::

## Achievement Tiers

Five tiers (`api/internal/models/achievement.go:27-33`). Point values are per-achievement,
set by the `points` field; they are not fixed per tier:

| Tier | Value |
|------|-------|
| Bronze | `bronze` |
| Silver | `silver` |
| Gold | `gold` |
| Platinum | `platinum` |
| Diamond | `diamond` |

## Achievement Types

`type` is a **top-level field on the achievement**, not a field inside `criteria`
(`achievement.go:12-22`):

`lab_completion`, `perfect_score`, `streak`, `speed`, `category`, `milestone`, `special`,
`pathway`, `pathway_completion`

## Achievement Criteria

The complete set of `AchievementCriteria` fields (`achievement.go:52-84`). All are
`omitempty`, so an achievement sets only the ones it needs:

| Field | Type | Meaning |
|-------|------|---------|
| `labTemplateId` | string | A specific lab to complete |
| `labCategory` | string | Complete all labs in a category |
| `labTags` | string[] | Labs carrying specific tags |
| `minScore` | int | Minimum percentage, 0-100 |
| `requirePerfect` | bool | Must score 100% |
| `maxDurationMins` | int | Complete within N minutes |
| `streakCount` | int | Consecutive completions |
| `streakType` | string | `daily`, `weekly`, `any` |
| `totalLabs` | int | Total labs completed |
| `totalPoints` | int | Total points earned |
| `minDifficulty` | string | `beginner`, `intermediate`, `advanced`, `expert` |
| `pathwayId` | string | A specific pathway to complete |
| `pathwaySlug` | string | Pathway slug, as an alternative to the id |
| `modulesRequired` | int | Number of modules to complete |
| `type` | string | `pathway_modules`, `pathway_complete`, `lab_perfect_score` |
| `customRule` | string | Custom rule identifier |

Renames to watch for, if you are working from older notes or the previous version of this
page:

| Was documented as | Actually |
|-------------------|----------|
| `count` | `totalLabs` (milestones) or `streakCount` (streaks) |
| `requirePass` | `requirePerfect` (or `minScore` for a pass threshold) |
| `maxMinutes` | `maxDurationMins` |
| `days` | `streakCount` with `streakType: "daily"` |
| `category` | `labCategory` |

---

## Achievement Endpoints

Five routes, all `GET`, all read-only
(`api/internal/server/achievement_handlers.go:46-59`). There is no endpoint to create,
update or delete an achievement.

### List All Achievements

<span class="api-method get">GET</span> `/api/v1/achievements`

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `type` | string | Filter by achievement type |
| `tier` | string | Filter by tier |
| `active` | bool | Pass `true` to return only active achievements |

**Response (200 OK):**

```json
{
  "achievements": [
    {
      "id": "first-lab",
      "name": "First Steps",
      "description": "Complete your first lab",
      "type": "milestone",
      "tier": "bronze",
      "iconUrl": "/icons/star.svg",
      "points": 10,
      "isSecret": false,
      "isActive": true,
      "criteria": { "totalLabs": 1 },
      "createdAt": "2025-01-01T00:00:00Z",
      "updatedAt": "2025-01-01T00:00:00Z"
    },
    {
      "id": "perfect-score",
      "name": "Perfectionist",
      "description": "Get 100% on any lab",
      "type": "perfect_score",
      "tier": "silver",
      "iconUrl": "/icons/check-circle.svg",
      "points": 25,
      "isSecret": false,
      "isActive": true,
      "criteria": { "requirePerfect": true },
      "createdAt": "2025-01-01T00:00:00Z",
      "updatedAt": "2025-01-01T00:00:00Z"
    },
    {
      "id": "speed-demon",
      "name": "Speed Demon",
      "description": "Complete a lab in under 30 minutes",
      "type": "speed",
      "tier": "silver",
      "iconUrl": "/icons/bolt.svg",
      "points": 25,
      "isSecret": false,
      "isActive": true,
      "criteria": { "maxDurationMins": 30 },
      "createdAt": "2025-01-01T00:00:00Z",
      "updatedAt": "2025-01-01T00:00:00Z"
    }
  ],
  "count": 3
}
```

The icon field is `iconUrl`, and there is no `category` field on an achievement — category
targeting lives in `criteria.labCategory`.

---

### Get Achievement

<span class="api-method get">GET</span> `/api/v1/achievements/{achievementID}`

**Response (200 OK):** the bare achievement object, with no wrapper and no `count`:

```json
{
  "id": "lab-master",
  "name": "Lab Master",
  "description": "Complete 10 labs",
  "type": "milestone",
  "tier": "gold",
  "iconUrl": "/icons/trophy.svg",
  "points": 50,
  "isSecret": false,
  "isActive": true,
  "criteria": { "totalLabs": 10, "minScore": 70 },
  "createdAt": "2025-01-01T00:00:00Z",
  "updatedAt": "2025-01-01T00:00:00Z"
}
```

There are no `earnedCount` or `rarity` fields.

**Other statuses:** 404 if not found, or if the achievement repository is not wired up.

---

### Get Recent Achievements

<span class="api-method get">GET</span> `/api/v1/achievements/recent`

Recently earned achievements across all users.

:::note The limit is fixed
`limit` is hardcoded to 10 in the handler (`achievement_handlers.go:194`). A `?limit=`
query parameter is accepted by the URL but ignored.
:::

**Response (200 OK):** `{ "achievements": [ …UserAchievement… ], "count": N }`, where each
entry is a `UserAchievement` (`achievement.go:87-97`):

```json
{
  "achievements": [
    {
      "id": "ua-123",
      "userId": "user-uuid-1",
      "achievementId": "speed-demon",
      "achievement": { "id": "speed-demon", "name": "Speed Demon", "tier": "silver" },
      "earnedAt": "2025-01-15T10:30:00Z",
      "sessionId": "sess-xyz789",
      "notified": true,
      "createdAt": "2025-01-15T10:30:00Z"
    }
  ],
  "count": 1
}
```

---

## User Achievement Endpoints

### Get User Achievements

<span class="api-method get">GET</span> `/api/v1/users/{userID}/achievements`

Callers may read their own achievements; admins and instructors may read anyone's.

**Response (200 OK):**

A flat `achievements` array with a `count` — **not** the `{earned, progress}` split this
page used to show:

```json
{
  "achievements": [
    {
      "id": "ua-123",
      "userId": "user-uuid",
      "achievementId": "first-lab",
      "achievement": {
        "id": "first-lab",
        "name": "First Steps",
        "type": "milestone",
        "tier": "bronze",
        "points": 10
      },
      "earnedAt": "2025-01-10T14:30:00Z",
      "progress": 100,
      "notified": true,
      "createdAt": "2025-01-10T14:30:00Z"
    }
  ],
  "count": 1
}
```

**Other statuses:** 401 unauthenticated, 403 reading another user's achievements without
instructor or admin role.

---

### Get Achievement Summary

<span class="api-method get">GET</span> `/api/v1/users/{userID}/achievements/summary`

**Response (200 OK):** a bare `UserAchievementSummary` (`achievement.go:111-122`). The
breakdown keys are `rarityBreakdown` and `typeBreakdown` — there is no `byTier`,
`byCategory` or `nextMilestones`:

```json
{
  "userId": "user-uuid",
  "totalEarned": 5,
  "totalPoints": 120,
  "totalAvailable": 25,
  "completionPercent": 20.0,
  "recentAchievements": [
    {
      "id": "ua-124",
      "userId": "user-uuid",
      "achievementId": "perfect-score",
      "earnedAt": "2025-01-12T16:45:00Z",
      "notified": true,
      "createdAt": "2025-01-12T16:45:00Z"
    }
  ],
  "rarityBreakdown": { "bronze": 3, "silver": 1, "gold": 1, "platinum": 0, "diamond": 0 },
  "typeBreakdown": { "milestone": 2, "perfect_score": 1, "speed": 1, "streak": 1 }
}
```

**Other statuses:** 401, 403 as above; 404 if the achievement service is not wired up.

---

## Leaderboard

### Get Points Leaderboard

<span class="api-method get">GET</span> `/api/v1/leaderboard`

Served by the dashboard manager (`api/internal/server/dashboards/manager.go:64`), not the
achievement manager.

**Query Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `limit` | int | Number of entries (default 10, max 100; out-of-range values fall back to the default) |

There is no `teamId` filter.

**Response (200 OK):**

`dashboard.LeaderboardData` (`api/internal/dashboard/service.go:198-213`):

```json
{
  "entries": [
    {
      "rank": 1,
      "userId": "user-uuid-1",
      "displayName": "Jane Doe",
      "totalPoints": 1500,
      "achievementCount": 15,
      "labsCompleted": 25,
      "isCurrentUser": false
    },
    {
      "rank": 2,
      "userId": "user-uuid-2",
      "displayName": "John Smith",
      "totalPoints": 1200,
      "achievementCount": 12,
      "labsCompleted": 20,
      "isCurrentUser": true
    }
  ],
  "currentUser": {
    "rank": 2,
    "userId": "user-uuid-2",
    "displayName": "John Smith",
    "totalPoints": 1200,
    "achievementCount": 12,
    "labsCompleted": 20,
    "isCurrentUser": true
  },
  "totalUsers": 150
}
```

`currentUser` is a **complete entry**, not the `{rank, totalPoints}` pair this page used to
show, and it is present only when the caller appears in the returned page of entries. Each
entry also carries `isCurrentUser`.

---

## WebSocket Events

:::danger No achievement events are broadcast
This page used to document an `achievement.earned` WebSocket event. **No such event
exists.** The hub broadcasts exactly seven types — `checkpoint`, `session`, `grade`,
`assessment`, `pod_provisioning`, `hint_nudge`, `monitoring_event`
(`api/internal/websocket/hub.go:327-415`) — and none of them concerns achievements. A UI
wanting achievement notifications has to poll
`GET /api/v1/users/{userID}/achievements`.

`AchievementNotification` (`api/internal/models/achievement.go:133-138`) exists as a type,
but nothing routes it to the WebSocket hub.
:::

See [WebSocket Events](websocket.md) for the events that do exist.

---

## Error Responses

| Status | Error | Meaning |
|--------|-------|---------|
| 401 | `unauthorized` | Auth required |
| 403 | `forbidden` | Cannot view other user's achievements |
| 404 | `not_found` | Achievement or user not found |
