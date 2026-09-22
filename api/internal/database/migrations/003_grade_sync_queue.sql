-- Grade Sync Queue table for Canvas LTI grade synchronization
-- Enables async grade submission with retry capability

CREATE TABLE grade_sync_queue (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id UUID NOT NULL REFERENCES lab_sessions(id) ON DELETE CASCADE,

    -- Sync status
    status VARCHAR(32) NOT NULL DEFAULT 'pending',  -- pending, processing, completed, failed

    -- Grade data
    earned_points INT NOT NULL,
    max_points INT NOT NULL,
    percentage DECIMAL(5,2) NOT NULL,

    -- Canvas identifiers
    canvas_course_id VARCHAR(128) NOT NULL,
    canvas_assignment_id VARCHAR(128) NOT NULL,
    canvas_user_id VARCHAR(128) NOT NULL,

    -- Retry tracking
    attempts INT NOT NULL DEFAULT 0,
    last_attempt_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    error_message TEXT,

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_grade_sync_queue_status ON grade_sync_queue(status);
CREATE INDEX idx_grade_sync_queue_session ON grade_sync_queue(session_id);
CREATE INDEX idx_grade_sync_queue_pending ON grade_sync_queue(created_at) WHERE status = 'pending';
