-- Migration: Add enrollment and module links to lab_sessions
-- This enables automatic pathway progress tracking when sessions complete

-- Add enrollment_id to link sessions to pathway enrollments
ALTER TABLE lab_sessions ADD COLUMN IF NOT EXISTS enrollment_id UUID REFERENCES pathway_enrollments(id) ON DELETE SET NULL;

-- Add module_id to track which module this session belongs to
ALTER TABLE lab_sessions ADD COLUMN IF NOT EXISTS module_id UUID REFERENCES pathway_modules(id) ON DELETE SET NULL;

-- Index for efficient lookups
CREATE INDEX IF NOT EXISTS idx_lab_sessions_enrollment ON lab_sessions(enrollment_id) WHERE enrollment_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_lab_sessions_module ON lab_sessions(module_id) WHERE module_id IS NOT NULL;

-- Composite index for finding sessions by enrollment and module
CREATE INDEX IF NOT EXISTS idx_lab_sessions_enrollment_module ON lab_sessions(enrollment_id, module_id) WHERE enrollment_id IS NOT NULL;

COMMENT ON COLUMN lab_sessions.enrollment_id IS 'Reference to pathway enrollment if session is part of a pathway';
COMMENT ON COLUMN lab_sessions.module_id IS 'Reference to pathway module if session is part of a pathway';
