-- Migration 007: Schema Constraints and Improvements
-- Based on SQL review recommendations

-- =============================================================================
-- 1. ENUM TYPES for status columns (for consistency)
-- =============================================================================

-- Create ENUM for wazuh_agents.status
DO $$ BEGIN
    CREATE TYPE wazuh_agent_status AS ENUM ('pending', 'active', 'disconnected');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- Create ENUM for grade_sync_queue.status
DO $$ BEGIN
    CREATE TYPE grade_sync_status AS ENUM ('pending', 'processing', 'completed', 'failed');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- Create ENUM for assessment_results.status
DO $$ BEGIN
    CREATE TYPE assessment_status AS ENUM ('in_progress', 'completed', 'graded');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- Note: Converting existing VARCHAR columns to ENUM would require data migration
-- These ENUMs are created for documentation and future use
-- To migrate existing columns, run:
--   ALTER TABLE wazuh_agents ALTER COLUMN status TYPE wazuh_agent_status USING status::wazuh_agent_status;
--   ALTER TABLE grade_sync_queue ALTER COLUMN status TYPE grade_sync_status USING status::grade_sync_status;
--   ALTER TABLE assessment_results ALTER COLUMN status TYPE assessment_status USING status::assessment_status;

-- =============================================================================
-- 2. CHECK CONSTRAINTS for data validation
-- =============================================================================

-- lab_templates: max_points must be non-negative
ALTER TABLE lab_templates
    DROP CONSTRAINT IF EXISTS chk_lab_templates_max_points_positive;
ALTER TABLE lab_templates
    ADD CONSTRAINT chk_lab_templates_max_points_positive
    CHECK (max_points >= 0);

-- lab_templates: pass_threshold must be 0-100
ALTER TABLE lab_templates
    DROP CONSTRAINT IF EXISTS chk_lab_templates_pass_threshold_range;
ALTER TABLE lab_templates
    ADD CONSTRAINT chk_lab_templates_pass_threshold_range
    CHECK (pass_threshold >= 0 AND pass_threshold <= 100);

-- lab_sessions: percentage must be 0-100
ALTER TABLE lab_sessions
    DROP CONSTRAINT IF EXISTS chk_lab_sessions_percentage_range;
ALTER TABLE lab_sessions
    ADD CONSTRAINT chk_lab_sessions_percentage_range
    CHECK (percentage >= 0 AND percentage <= 100);

-- lab_sessions: earned_points must be non-negative
ALTER TABLE lab_sessions
    DROP CONSTRAINT IF EXISTS chk_lab_sessions_earned_points_positive;
ALTER TABLE lab_sessions
    ADD CONSTRAINT chk_lab_sessions_earned_points_positive
    CHECK (earned_points >= 0);

-- lab_sessions: max_points must be non-negative
ALTER TABLE lab_sessions
    DROP CONSTRAINT IF EXISTS chk_lab_sessions_max_points_positive;
ALTER TABLE lab_sessions
    ADD CONSTRAINT chk_lab_sessions_max_points_positive
    CHECK (max_points >= 0);

-- checkpoint_progress: points must be non-negative
ALTER TABLE checkpoint_progress
    DROP CONSTRAINT IF EXISTS chk_checkpoint_progress_points_positive;
ALTER TABLE checkpoint_progress
    ADD CONSTRAINT chk_checkpoint_progress_points_positive
    CHECK (points >= 0);

-- checkpoint_progress: earned_points must be non-negative
ALTER TABLE checkpoint_progress
    DROP CONSTRAINT IF EXISTS chk_checkpoint_progress_earned_points_positive;
ALTER TABLE checkpoint_progress
    ADD CONSTRAINT chk_checkpoint_progress_earned_points_positive
    CHECK (earned_points >= 0);

-- checkpoint_progress: attempt_count must be non-negative
ALTER TABLE checkpoint_progress
    DROP CONSTRAINT IF EXISTS chk_checkpoint_progress_attempt_count_positive;
ALTER TABLE checkpoint_progress
    ADD CONSTRAINT chk_checkpoint_progress_attempt_count_positive
    CHECK (attempt_count >= 0);

-- assessment_results: percentage must be 0-100
ALTER TABLE assessment_results
    DROP CONSTRAINT IF EXISTS chk_assessment_results_percentage_range;
ALTER TABLE assessment_results
    ADD CONSTRAINT chk_assessment_results_percentage_range
    CHECK (percentage >= 0 AND percentage <= 100);

-- assessment_results: score must be non-negative
ALTER TABLE assessment_results
    DROP CONSTRAINT IF EXISTS chk_assessment_results_score_positive;
ALTER TABLE assessment_results
    ADD CONSTRAINT chk_assessment_results_score_positive
    CHECK (score >= 0);

-- assessment_results: max_score must be non-negative
ALTER TABLE assessment_results
    DROP CONSTRAINT IF EXISTS chk_assessment_results_max_score_positive;
ALTER TABLE assessment_results
    ADD CONSTRAINT chk_assessment_results_max_score_positive
    CHECK (max_score >= 0);

-- grade_history: percentage must be 0-100
ALTER TABLE grade_history
    DROP CONSTRAINT IF EXISTS chk_grade_history_percentage_range;
ALTER TABLE grade_history
    ADD CONSTRAINT chk_grade_history_percentage_range
    CHECK (percentage >= 0 AND percentage <= 100);

-- grade_sync_queue: percentage must be 0-100
ALTER TABLE grade_sync_queue
    DROP CONSTRAINT IF EXISTS chk_grade_sync_queue_percentage_range;
ALTER TABLE grade_sync_queue
    ADD CONSTRAINT chk_grade_sync_queue_percentage_range
    CHECK (percentage >= 0 AND percentage <= 100);

-- grade_sync_queue: attempts must be non-negative
ALTER TABLE grade_sync_queue
    DROP CONSTRAINT IF EXISTS chk_grade_sync_queue_attempts_positive;
ALTER TABLE grade_sync_queue
    ADD CONSTRAINT chk_grade_sync_queue_attempts_positive
    CHECK (attempts >= 0);

-- =============================================================================
-- 3. ON DELETE behaviors for key relationships
-- =============================================================================

-- Note: Changing FK constraints requires dropping and recreating them
-- These changes prevent orphaned records

-- pods.lab_template_id: RESTRICT deletion of templates with active pods
ALTER TABLE pods
    DROP CONSTRAINT IF EXISTS pods_lab_template_id_fkey;
ALTER TABLE pods
    ADD CONSTRAINT pods_lab_template_id_fkey
    FOREIGN KEY (lab_template_id) REFERENCES lab_templates(id) ON DELETE RESTRICT;

-- pods.owner_id: RESTRICT deletion of users with pods
ALTER TABLE pods
    DROP CONSTRAINT IF EXISTS pods_owner_id_fkey;
ALTER TABLE pods
    ADD CONSTRAINT pods_owner_id_fkey
    FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE RESTRICT;

-- lab_sessions.pod_id: CASCADE when pod is deleted
ALTER TABLE lab_sessions
    DROP CONSTRAINT IF EXISTS lab_sessions_pod_id_fkey;
ALTER TABLE lab_sessions
    ADD CONSTRAINT lab_sessions_pod_id_fkey
    FOREIGN KEY (pod_id) REFERENCES pods(id) ON DELETE CASCADE;

-- lab_sessions.user_id: RESTRICT deletion of users with sessions
ALTER TABLE lab_sessions
    DROP CONSTRAINT IF EXISTS lab_sessions_user_id_fkey;
ALTER TABLE lab_sessions
    ADD CONSTRAINT lab_sessions_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE RESTRICT;

-- lab_sessions.lab_template_id: RESTRICT deletion of templates with sessions
ALTER TABLE lab_sessions
    DROP CONSTRAINT IF EXISTS lab_sessions_lab_template_id_fkey;
ALTER TABLE lab_sessions
    ADD CONSTRAINT lab_sessions_lab_template_id_fkey
    FOREIGN KEY (lab_template_id) REFERENCES lab_templates(id) ON DELETE RESTRICT;

-- grade_history.session_id: CASCADE when session is deleted
ALTER TABLE grade_history
    DROP CONSTRAINT IF EXISTS grade_history_session_id_fkey;
ALTER TABLE grade_history
    ADD CONSTRAINT grade_history_session_id_fkey
    FOREIGN KEY (session_id) REFERENCES lab_sessions(id) ON DELETE CASCADE;

-- =============================================================================
-- 4. Index on events.pod_id for FK-like behavior (no actual FK due to composite PK)
-- =============================================================================

-- Note: events table has composite PK (timestamp, id) which makes standard FK difficult
-- We rely on application-level referential integrity for events
-- The existing idx_events_pod index provides query performance

-- Add comment documenting the design decision
COMMENT ON TABLE events IS 'Event storage table with composite PK (timestamp, id) for TimescaleDB compatibility. pod_id and session_id have no FK constraints due to composite PK complexity - referential integrity is enforced at application level.';
