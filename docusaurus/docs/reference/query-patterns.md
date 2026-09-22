---
title: Query Patterns
description: Common database query patterns and optimization tips
tags:
  - database
  - queries
  - performance
---

# Query Patterns

This document covers common database query patterns used in Kootenai and optimization tips.

## Common Queries

### User Queries

#### Get User by ID

```sql
SELECT id, email, display_name, role, created_at, last_login_at
FROM users
WHERE id = $1;
```

#### Get User by Email

```sql
SELECT id, email, display_name, role, password_hash, created_at
FROM users
WHERE email = $1;
```

#### List Active Users

```sql
SELECT id, email, display_name, role, last_login_at
FROM users
WHERE is_active = true
ORDER BY last_login_at DESC
LIMIT $1 OFFSET $2;
```

### Lab Queries

#### List Active Labs

```sql
SELECT id, name, description, platform, difficulty, duration_minutes,
       max_points, pass_threshold, created_at
FROM lab_templates
WHERE is_active = true
ORDER BY name;
```

**Index used:** `idx_lab_templates_active`

#### Get Lab with Checkpoints

```sql
SELECT
    lt.id, lt.name, lt.description, lt.spec, lt.checkpoints,
    COUNT(p.id) as pod_count
FROM lab_templates lt
LEFT JOIN pods p ON p.lab_template_id = lt.id
    AND p.status NOT IN ('destroyed', 'destroying')
WHERE lt.id = $1
GROUP BY lt.id;
```

### Pod Queries

#### Get User's Active Pods

```sql
SELECT
    p.id, p.status, p.vms, p.networks, p.created_at, p.expires_at,
    lt.name as lab_name, lt.id as lab_template_id
FROM pods p
JOIN lab_templates lt ON lt.id = p.lab_template_id
WHERE p.owner_id = $1
    AND p.status NOT IN ('destroyed', 'destroying')
ORDER BY p.created_at DESC;
```

**Index used:** `idx_pods_owner_active`

#### Get Expiring Pods

```sql
SELECT id, owner_id, expires_at, status
FROM pods
WHERE expires_at < NOW() + INTERVAL '1 hour'
    AND status = 'running'
ORDER BY expires_at;
```

**Index used:** `idx_pods_expires`

### Session Queries

#### Get Active Session for Pod

```sql
SELECT
    s.id, s.started_at, s.earned_points, s.max_points,
    s.canvas_course_id, s.canvas_assignment_id
FROM lab_sessions s
WHERE s.pod_id = $1
    AND s.ended_at IS NULL
LIMIT 1;
```

**Index used:** `idx_sessions_active`

#### Get Session Progress

```sql
SELECT
    s.id, s.earned_points, s.max_points, s.percentage, s.passed,
    json_agg(json_build_object(
        'id', cp.checkpoint_id,
        'status', cp.status,
        'points', cp.points,
        'earned_points', cp.earned_points,
        'passed_at', cp.passed_at
    )) as checkpoints
FROM lab_sessions s
LEFT JOIN checkpoint_progress cp ON cp.session_id = s.id
WHERE s.id = $1
GROUP BY s.id;
```

#### Canvas Assignment Sessions

```sql
SELECT
    s.id, s.user_id, s.earned_points, s.max_points, s.percentage,
    s.grade_synced_at, s.grade_sync_error,
    u.email, u.display_name
FROM lab_sessions s
JOIN users u ON u.id = s.user_id
WHERE s.canvas_assignment_id = $1
    AND s.canvas_user_id = $2
ORDER BY s.started_at DESC
LIMIT 1;
```

**Index used:** `idx_sessions_canvas_assignment_user`

### Achievement Queries

#### Get User Achievements

```sql
SELECT
    a.id, a.name, a.description, a.tier, a.points, a.icon,
    ua.earned_at
FROM achievements a
JOIN user_achievements ua ON ua.achievement_id = a.id
WHERE ua.user_id = $1
ORDER BY ua.earned_at DESC;
```

#### Achievement Progress

```sql
WITH user_stats AS (
    SELECT
        COUNT(DISTINCT ls.id) FILTER (WHERE ls.passed = true) as labs_completed,
        COUNT(DISTINCT ls.id) FILTER (WHERE ls.percentage = 100) as perfect_scores,
        MAX(ls.ended_at) as last_completion
    FROM lab_sessions ls
    WHERE ls.user_id = $1 AND ls.ended_at IS NOT NULL
)
SELECT
    a.id, a.name, a.criteria,
    us.labs_completed,
    us.perfect_scores
FROM achievements a
CROSS JOIN user_stats us
WHERE a.id NOT IN (SELECT achievement_id FROM user_achievements WHERE user_id = $1);
```

### Event Queries

#### Unprocessed Events for Pod

```sql
SELECT id, timestamp, event_type, vm_name, description, data
FROM events
WHERE pod_id = $1
    AND processed = false
ORDER BY timestamp
LIMIT 100;
```

**Index used:** `idx_events_unprocessed`

#### Search Events

```sql
SELECT id, timestamp, event_type, vm_name, description
FROM events
WHERE pod_id = $1
    AND timestamp BETWEEN $2 AND $3
    AND ($4 IS NULL OR event_type = $4)
ORDER BY timestamp DESC
LIMIT $5;
```

### Organization Queries

#### Get User's Organizations

```sql
SELECT
    o.id, o.name, o.slug, o.type, o.edition,
    om.role as member_role,
    COUNT(DISTINCT om2.user_id) as member_count
FROM organizations o
JOIN organization_members om ON om.organization_id = o.id AND om.user_id = $1
LEFT JOIN organization_members om2 ON om2.organization_id = o.id
WHERE o.is_active = true
GROUP BY o.id, om.role;
```

#### Team Members with Progress

```sql
SELECT
    u.id, u.email, u.display_name,
    COUNT(DISTINCT ls.id) as labs_completed,
    AVG(ls.percentage) as avg_score
FROM users u
JOIN team_members tm ON tm.user_id = u.id
LEFT JOIN lab_sessions ls ON ls.user_id = u.id AND ls.passed = true
WHERE tm.team_id = $1
GROUP BY u.id
ORDER BY labs_completed DESC;
```

## Optimization Tips

### Index Usage

Verify index usage with `EXPLAIN ANALYZE`:

```sql
EXPLAIN ANALYZE
SELECT * FROM pods
WHERE owner_id = '...' AND status NOT IN ('destroyed', 'destroying');
```

Good output shows `Index Scan using idx_pods_owner_active`.

### Common Optimizations

#### Use Partial Indexes

For queries that always filter on a condition:

```sql
-- Instead of full index
CREATE INDEX idx_pods_status ON pods(status);

-- Use partial index
CREATE INDEX idx_pods_active ON pods(owner_id)
WHERE status NOT IN ('destroyed', 'destroying');
```

#### Avoid SELECT *

Only select needed columns:

```sql
-- Bad
SELECT * FROM lab_sessions WHERE user_id = $1;

-- Good
SELECT id, status, earned_points, max_points FROM lab_sessions WHERE user_id = $1;
```

#### Use EXISTS Instead of COUNT

For existence checks:

```sql
-- Bad
SELECT COUNT(*) > 0 FROM pods WHERE owner_id = $1 AND status = 'running';

-- Good
SELECT EXISTS(SELECT 1 FROM pods WHERE owner_id = $1 AND status = 'running');
```

#### Batch Operations

For multiple inserts:

```sql
-- Bad: Multiple round trips
INSERT INTO checkpoint_progress (session_id, checkpoint_id) VALUES ($1, $2);
INSERT INTO checkpoint_progress (session_id, checkpoint_id) VALUES ($1, $3);

-- Good: Single statement
INSERT INTO checkpoint_progress (session_id, checkpoint_id) VALUES
    ($1, $2),
    ($1, $3),
    ($1, $4);
```

### Query Analysis

#### Slow Query Log

Enable in PostgreSQL:

```sql
ALTER SYSTEM SET log_min_duration_statement = 100; -- Log queries > 100ms
SELECT pg_reload_conf();
```

#### pg_stat_statements

Find slow queries:

```sql
SELECT
    query,
    calls,
    mean_time,
    total_time,
    rows
FROM pg_stat_statements
ORDER BY mean_time DESC
LIMIT 20;
```

### Connection Pooling

For high concurrency, use PgBouncer:

```
[databases]
virtuallab = host=postgres port=5432 dbname=virtuallab

[pgbouncer]
pool_mode = transaction
max_client_conn = 1000
default_pool_size = 20
```

## N+1 Query Prevention

### Problem

```go
// Bad: N+1 queries
pods := db.GetPods(userID)
for _, pod := range pods {
    lab := db.GetLab(pod.LabTemplateID) // N additional queries
}
```

### Solution

```go
// Good: Single query with JOIN
pods := db.GetPodsWithLabs(userID)

// Or batch fetch
labIDs := extractLabIDs(pods)
labs := db.GetLabsByIDs(labIDs)
labMap := indexByID(labs)
```

SQL with JOIN:

```sql
SELECT p.*, lt.name as lab_name, lt.description as lab_description
FROM pods p
JOIN lab_templates lt ON lt.id = p.lab_template_id
WHERE p.owner_id = $1;
```

## Related Documentation

- [Schema Overview](database-schema.md) - Database schema
- [Migrations](migrations.md) - Migration management
- [Scaling Guide](../admin/scaling.md) - Database scaling
