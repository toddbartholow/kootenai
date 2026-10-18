-- Migration 004: Schema Improvements
-- High-priority fixes identified by SQL analysis

-- -----------------------------------------------------------------------------
-- 1. Add updated_at column to pods table (was missing)
-- -----------------------------------------------------------------------------
ALTER TABLE pods ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE TRIGGER trigger_pods_updated
BEFORE UPDATE ON pods
FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- -----------------------------------------------------------------------------
-- 2. Remove redundant indexes (unique constraints already create indexes)
-- -----------------------------------------------------------------------------
DROP INDEX IF EXISTS idx_lab_templates_name;  -- redundant with lab_templates_name_key
DROP INDEX IF EXISTS idx_users_external_id;   -- redundant with users_external_id_key
DROP INDEX IF EXISTS idx_users_username;      -- redundant with users_username_key

-- -----------------------------------------------------------------------------
-- 3. Add composite index for Canvas assignment lookups (performance)
-- -----------------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS idx_sessions_canvas_assignment_user
ON lab_sessions(canvas_assignment_id, canvas_user_id)
WHERE canvas_assignment_id IS NOT NULL;

-- -----------------------------------------------------------------------------
-- 4. Add composite index for active pods by owner (common query pattern)
-- -----------------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS idx_pods_owner_active
ON pods(owner_id, status)
WHERE status NOT IN ('destroyed', 'destroying');

-- -----------------------------------------------------------------------------
-- 5. Add index for grade sync retry queries
-- -----------------------------------------------------------------------------
CREATE INDEX IF NOT EXISTS idx_grade_sync_queue_retry
ON grade_sync_queue(status, attempts, created_at)
WHERE status IN ('failed', 'pending');
