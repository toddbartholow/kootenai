# TimescaleDB Events Table Partitioning Analysis

## Current State

The codebase **already has time-based partitioning** implemented via native PostgreSQL declarative partitioning (migration 018). The events table is partitioned by month with:
- 12 monthly partitions pre-created
- A default partition for overflow
- Maintenance functions for creating new partitions

The question is: **Should you replace native PostgreSQL partitioning with TimescaleDB hypertables?**

---

## Benefits of TimescaleDB

### 1. **Automatic Partition Management**
- **Native PG**: Requires manual partition creation (monthly cron job or trigger)
- **TimescaleDB**: Automatically creates "chunks" (partitions) as data arrives
- **Impact**: No maintenance overhead, no risk of data going to wrong partition

### 2. **Better Compression** (TimescaleDB native)
- **Native PG**: No built-in compression for time-series
- **TimescaleDB**: 90-95% compression ratios on older data
- **Impact**: Significant storage savings for long-term event retention

### 3. **Time-Series Optimized Queries**
- **Native PG**: Standard B-tree indexes, manual query tuning
- **TimescaleDB**: Optimized chunk exclusion, time-bucket functions, continuous aggregates
- **Impact**: Faster queries like "events in last hour" or "daily event counts"

### 4. **Continuous Aggregates**
- Pre-computed rollups (hourly/daily stats) that auto-update
- Example: Real-time dashboard showing "events per hour by type"
- **Impact**: Much faster analytics without complex materialized view management

### 5. **Data Retention Policies**
- Built-in `add_retention_policy()` to automatically drop old chunks
- Example: `SELECT add_retention_policy('events', INTERVAL '90 days');`
- **Impact**: Automatic cleanup vs manual DELETE operations

### 6. **Better Index Management**
- Indexes automatically created per-chunk (smaller, faster)
- Parallel index creation on chunks
- **Impact**: Faster writes, faster index maintenance

---

## Downsides of TimescaleDB

### 1. **Additional Dependency**
- Requires installing TimescaleDB extension on PostgreSQL
- Not available on all managed PostgreSQL providers (check your hosting)
- Docker: Need `timescale/timescaledb` image instead of plain `postgres`

### 2. **Migration Complexity**
- Must migrate existing partitioned data to hypertable
- Existing events table would need to be converted
- Foreign key from `checkpoint_progress.triggered_by_event_id → events.id` complicates migration

### 3. **Learning Curve**
- New concepts: chunks, continuous aggregates, compression policies
- Different query patterns for best performance
- Team needs to understand TimescaleDB-specific features

### 4. **Licensing Considerations**
- TimescaleDB Community: Apache 2.0 (free, open source)
- TimescaleDB Enterprise: Some advanced features are paid
- Compression is now in Community edition (was previously Enterprise-only)

### 5. **Resource Usage**
- Background workers for compression, retention, continuous aggregates
- Additional memory/CPU for TimescaleDB processes
- May be overkill for low-volume deployments

### 6. **Foreign Key Limitations**
- TimescaleDB hypertables have restrictions on foreign keys
- Your `checkpoint_progress.triggered_by_event_id` references `events.id`
- Would need to handle this relationship differently (application-level or remove FK)

---

## Recommendation

### When TimescaleDB Makes Sense:
- **High volume**: Thousands of events per minute
- **Long retention**: Need to keep months/years of events
- **Analytics**: Dashboard queries on event trends, aggregations
- **Storage concerns**: Events table growing too large

### When Native PG Partitioning is Sufficient:
- **Moderate volume**: Hundreds of events per minute
- **Short retention**: Events are deleted after days/weeks
- **Simple queries**: Mostly "get events by session/pod"
- **Minimal ops**: Don't want to manage another extension

### For This Project:
Given that:
1. You already have monthly partitioning implemented
2. The foreign key to `checkpoint_progress` exists
3. The primary use case is checkpoint matching (not analytics)

**Recommendation**: **Keep native PostgreSQL partitioning for now** unless you experience:
- Storage growth issues (consider adding compression via pg_cron + pg_compress)
- Performance issues with event queries
- Need for real-time analytics dashboards

If you do need TimescaleDB later, the migration path would be:
1. Install TimescaleDB extension
2. Create new hypertable for events
3. Remove or restructure the foreign key constraint
4. Migrate historical data
5. Update application code for TimescaleDB-specific features

---

## Summary Table

| Aspect | Native PG Partitioning | TimescaleDB |
|--------|----------------------|-------------|
| Auto partition creation | Manual (cron/trigger) | Automatic |
| Compression | None built-in | 90-95% ratio |
| Time-series queries | Standard | Optimized |
| Continuous aggregates | Manual materialized views | Built-in |
| Data retention | Manual DELETE | Built-in policies |
| Foreign keys | Full support | Limited |
| Dependency | None | Extension required |
| Your current state | Already implemented | Would need migration |
