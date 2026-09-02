# Why PostgreSQL for Kootenai Platform

PostgreSQL's strengths align well with this project's specific requirements.

## 1. JSONB for Flexible Lab Configurations

Lab templates have complex, variable structures (VMs, networks, checkpoints). JSONB allows:

```sql
-- Store entire template spec without rigid schema
spec JSONB NOT NULL,
checkpoints JSONB,

-- Query nested data efficiently
SELECT * FROM lab_templates
WHERE spec->'vms' @> '[{"template": "security-onion"}]';

-- Index JSON paths for fast lookups
CREATE INDEX idx_events_data ON events USING GIN(data);
```

MariaDB's JSON is stored as text and parsed on every query - no binary storage, no GIN indexes for fast containment queries.

## 2. High-Volume Event Ingestion

Wazuh generates thousands of security events per minute. PostgreSQL handles this with:

- **Partial indexes** - Only index unprocessed events, keeping index small:
  ```sql
  CREATE INDEX idx_events_unprocessed ON events(processed) WHERE processed = false;
  ```

- **Composite primary key for time-series** - Efficient for append-only workloads:
  ```sql
  PRIMARY KEY (timestamp, id)
  ```

- **TimescaleDB compatibility** - Can add time-series extension for automatic partitioning without code changes

## 3. Complex Checkpoint Evaluation

The trigger system automatically recalculates scores when checkpoints change:

```sql
CREATE FUNCTION update_session_progress() RETURNS TRIGGER AS $$
BEGIN
    UPDATE lab_sessions SET
        earned_points = (SELECT SUM(earned_points) FROM checkpoint_progress WHERE session_id = NEW.session_id),
        percentage = ROUND((earned_points::DECIMAL / max_points) * 100, 2)
    WHERE id = NEW.session_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
```

This keeps scoring logic in the database, ensuring consistency even if the API crashes mid-operation.

## 4. Array Types for Event Matching

Events can trigger multiple checkpoints simultaneously:

```sql
matched_checkpoints TEXT[]  -- ['checkpoint-1', 'checkpoint-3', 'checkpoint-7']
```

Query which events matched a specific checkpoint:
```sql
SELECT * FROM events WHERE 'firewall-config' = ANY(matched_checkpoints);
```

MariaDB would require a junction table or JSON parsing for this.

## 5. INET Type for Network Lab Data

Labs involve IP addresses, subnets, and network configurations:

```sql
ip_address INET,

-- Native network operations
SELECT * FROM wazuh_agents WHERE ip_address << '10.0.16.0/20';  -- Subnet containment
```

MariaDB stores IPs as strings with no validation or network-aware queries.

## 6. Strong Data Integrity

ENUMs prevent invalid states:
```sql
CREATE TYPE pod_status AS ENUM ('provisioning', 'running', 'stopped', 'error', 'destroying', 'destroyed');
```

The database rejects invalid values rather than silently accepting bad data.

## 7. RETURNING Clause for Efficiency

Single round-trip for insert + get generated values:
```sql
INSERT INTO lab_templates (...) VALUES (...)
RETURNING id, created_at, updated_at;
```

MariaDB requires two queries: `INSERT` then `SELECT LAST_INSERT_ID()`.

## Summary

| Project Need | PostgreSQL Solution |
|--------------|-------------------|
| Variable lab configs | JSONB with GIN indexes |
| High event throughput | Partial indexes, TimescaleDB-ready |
| Auto score calculation | PL/pgSQL triggers |
| Multi-checkpoint matching | Native arrays |
| Network topology data | INET type |
| State validation | Custom ENUMs |
| Efficient inserts | RETURNING clause |

PostgreSQL isn't just "better" generically - its features directly solve problems this platform faces. MariaDB would work, but you'd be reimplementing these capabilities in application code with worse performance.

## PostgreSQL-Specific Features Used

| Feature | PostgreSQL | MariaDB Equivalent |
|---------|------------|-------------------|
| **Extensions** | `uuid-ossp`, `pgcrypto` | No direct equivalent |
| **UUID generation** | `uuid_generate_v4()` | `UUID()` (MariaDB 10.7+) |
| **JSONB** | Native binary JSON with indexing | `JSON` (less performant, no GIN indexes) |
| **Custom ENUMs** | `CREATE TYPE ... AS ENUM` | Inline `ENUM()` in column definition |
| **Array types** | `TEXT[]` | Not supported natively |
| **INET type** | Native IP address type | `VARCHAR` (no validation) |
| **TIMESTAMPTZ** | Timestamp with time zone | `TIMESTAMP` (no TZ awareness) |
| **Partial indexes** | `WHERE is_active = true` | Not supported |
| **GIN indexes** | `USING GIN(data)` for JSONB | Not supported |
| **PL/pgSQL** | Stored procedures/triggers | Need to rewrite in MariaDB syntax |
| **`::` cast syntax** | `value::DECIMAL` | `CAST(value AS DECIMAL)` |
| **`RETURNING`** | `INSERT ... RETURNING id` | Not supported (need `LAST_INSERT_ID()`) |
| **`$1, $2` params** | Positional parameters | `?` placeholders |
