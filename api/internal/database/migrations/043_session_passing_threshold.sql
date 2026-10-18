-- +migrate Up
-- Add passing_threshold column to lab_sessions.
-- Previously the field existed on the Go model but was never persisted,
-- so the hardcoded default of 70 always applied.
ALTER TABLE lab_sessions ADD COLUMN IF NOT EXISTS passing_threshold INTEGER NOT NULL DEFAULT 70;

-- +migrate Down
ALTER TABLE lab_sessions DROP COLUMN IF EXISTS passing_threshold;
