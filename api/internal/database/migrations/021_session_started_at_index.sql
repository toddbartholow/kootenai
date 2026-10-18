-- Migration: Add index on lab_sessions.started_at
-- This improves query performance for:
--   - Listing sessions ordered by start time
--   - Finding sessions within date ranges
--   - Recent session queries
--
-- Note: Using regular CREATE INDEX instead of CONCURRENTLY because
-- CONCURRENTLY cannot run inside a transaction block (which the
-- migration system uses). For production with large tables, consider
-- running these manually outside of the migration system.

-- Index for ordering by started_at (DESC for recent-first queries)
CREATE INDEX IF NOT EXISTS idx_sessions_started_at
ON lab_sessions(started_at DESC);

-- Composite index for user sessions ordered by start time (common query pattern)
CREATE INDEX IF NOT EXISTS idx_sessions_user_started
ON lab_sessions(user_id, started_at DESC);

-- Composite index for template sessions ordered by start time
CREATE INDEX IF NOT EXISTS idx_sessions_template_started
ON lab_sessions(lab_template_id, started_at DESC);
