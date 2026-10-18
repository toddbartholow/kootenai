-- Migration: Performance optimization indexes
-- Adds indexes for common query patterns identified in API usage

-- =============================================================================
-- USER QUERIES
-- =============================================================================

-- Email lookups (login by email)
CREATE INDEX IF NOT EXISTS idx_users_email_active ON users(email) WHERE is_active = true;

-- Last login ordering for admin views
CREATE INDEX IF NOT EXISTS idx_users_last_login ON users(last_login_at DESC NULLS LAST) WHERE is_active = true;

-- =============================================================================
-- SESSION QUERIES
-- =============================================================================

-- User's recent sessions (dashboard, history)
CREATE INDEX IF NOT EXISTS idx_sessions_user_recent ON lab_sessions(user_id, started_at DESC);

-- Active sessions by template (admin monitoring)
CREATE INDEX IF NOT EXISTS idx_sessions_template_active ON lab_sessions(lab_template_id, started_at DESC) WHERE ended_at IS NULL;

-- Sessions by organization for reporting
CREATE INDEX IF NOT EXISTS idx_sessions_org_started ON lab_sessions(organization_id, started_at DESC) WHERE organization_id IS NOT NULL;

-- Graded sessions for analytics
CREATE INDEX IF NOT EXISTS idx_sessions_graded ON lab_sessions(user_id, percentage DESC) WHERE ended_at IS NOT NULL;

-- =============================================================================
-- POD QUERIES
-- =============================================================================

-- Active pods by owner (most common query)
CREATE INDEX IF NOT EXISTS idx_pods_owner_active ON pods(owner_id, created_at DESC) WHERE status IN ('provisioning', 'running');

-- Pods expiring soon (cleanup job)
CREATE INDEX IF NOT EXISTS idx_pods_expiring ON pods(expires_at) WHERE status = 'running' AND expires_at IS NOT NULL;

-- =============================================================================
-- ACHIEVEMENT QUERIES
-- =============================================================================

-- User's earned achievements (profile page)
CREATE INDEX IF NOT EXISTS idx_user_achievements_user_earned ON user_achievements(user_id, earned_at DESC);

-- Recent achievements for feed
CREATE INDEX IF NOT EXISTS idx_user_achievements_recent ON user_achievements(earned_at DESC) WHERE notified = false;

-- =============================================================================
-- ENROLLMENT QUERIES
-- =============================================================================

-- User's active enrollments
CREATE INDEX IF NOT EXISTS idx_enrollments_user_active ON pathway_enrollments(user_id, last_activity_at DESC NULLS LAST) WHERE status IN ('enrolled', 'in_progress');

-- Pathway completions for analytics
CREATE INDEX IF NOT EXISTS idx_enrollments_pathway_completed ON pathway_enrollments(pathway_id, completed_at DESC) WHERE status = 'completed';

-- =============================================================================
-- EVENT QUERIES (HIGH VOLUME)
-- =============================================================================

-- Events by pod and time range (checkpoint evaluation)
CREATE INDEX IF NOT EXISTS idx_events_pod_timestamp ON events(pod_id, timestamp DESC);

-- Unprocessed events (event processing queue)
CREATE INDEX IF NOT EXISTS idx_events_unprocessed_type ON events(event_type, timestamp ASC) WHERE processed = false;

-- =============================================================================
-- CHECKPOINT PROGRESS
-- =============================================================================

-- Checkpoint progress by session (live progress view)
CREATE INDEX IF NOT EXISTS idx_checkpoint_progress_session_status ON checkpoint_progress(session_id, status);

-- =============================================================================
-- AUDIT LOG
-- =============================================================================

-- Audit by resource (compliance queries)
CREATE INDEX IF NOT EXISTS idx_audit_log_resource_timestamp ON audit_log(resource_type, resource_id, timestamp DESC);

-- =============================================================================
-- COMPOSITE INDEXES FOR COMMON JOINS
-- =============================================================================

-- Sessions with user info (admin list view)
CREATE INDEX IF NOT EXISTS idx_sessions_composite ON lab_sessions(user_id, lab_template_id, started_at DESC);

-- Lab progress lookup
CREATE INDEX IF NOT EXISTS idx_lab_progress_composite ON lab_progress(enrollment_id, module_id, passed);

-- =============================================================================
-- STATISTICS UPDATE
-- =============================================================================

-- Update statistics for the optimizer
ANALYZE users;
ANALYZE lab_sessions;
ANALYZE pods;
ANALYZE achievements;
ANALYZE user_achievements;
ANALYZE pathway_enrollments;
ANALYZE checkpoint_progress;
ANALYZE events;
