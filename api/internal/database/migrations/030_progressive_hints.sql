-- Migration: 030_progressive_hints.sql
-- Description: Add progressive hints support for questions and checkpoints

-- Step 1: Rename hint_shown column to hint_level_shown and change type
-- This tracks the highest hint level revealed (0 = none, 1 = first hint, 2 = second, etc.)
ALTER TABLE question_responses
    ADD COLUMN hint_level_shown INTEGER NOT NULL DEFAULT 0;

-- Migrate existing data: if hint_shown was true, set hint_level_shown to 1
UPDATE question_responses
SET hint_level_shown = 1
WHERE hint_shown = true;

-- Step 2: Add hint penalty tracking column
ALTER TABLE question_responses
    ADD COLUMN hint_penalty_applied INTEGER NOT NULL DEFAULT 0;

-- Step 3: Drop the old hint_shown column
ALTER TABLE question_responses
    DROP COLUMN hint_shown;

-- Step 4: Create checkpoint hint progress tracking table
CREATE TABLE IF NOT EXISTS checkpoint_hint_progress (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id UUID NOT NULL REFERENCES lab_sessions(id) ON DELETE CASCADE,
    checkpoint_id VARCHAR(64) NOT NULL,
    hint_level_shown INTEGER NOT NULL DEFAULT 0,
    hint_penalty_applied INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(session_id, checkpoint_id)
);

-- Create indexes for checkpoint hint progress
CREATE INDEX IF NOT EXISTS idx_checkpoint_hint_progress_session
    ON checkpoint_hint_progress(session_id);

-- Trigger to auto-update updated_at timestamp
DROP TRIGGER IF EXISTS trigger_checkpoint_hint_progress_updated ON checkpoint_hint_progress;
CREATE TRIGGER trigger_checkpoint_hint_progress_updated
    BEFORE UPDATE ON checkpoint_hint_progress
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

-- Add comments
COMMENT ON COLUMN question_responses.hint_level_shown IS 'Highest hint level revealed (0 = none, 1-N = hint levels)';
COMMENT ON COLUMN question_responses.hint_penalty_applied IS 'Total point penalty from revealed hints';
COMMENT ON TABLE checkpoint_hint_progress IS 'Tracks progressive hint usage for checkpoints in lab sessions';
