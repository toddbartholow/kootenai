-- Kootenai Platform Database Schema
-- Initial migration: Core tables for lab management and checkpoint tracking

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- -----------------------------------------------------------------------------
-- Enum Types
-- -----------------------------------------------------------------------------

CREATE TYPE platform_type AS ENUM ('proxmox', 'cloudstack', 'any');
CREATE TYPE pod_status AS ENUM ('provisioning', 'running', 'stopped', 'error', 'destroying', 'destroyed');
CREATE TYPE checkpoint_status AS ENUM ('pending', 'passed', 'failed', 'skipped', 'partial');
CREATE TYPE trigger_type AS ENUM (
    'file_exists', 'file_content', 'file_deleted',
    'package', 'service', 'command_executed',
    'user_created', 'permission_changed',
    'network_connection', 'active_check', 'custom'
);

-- -----------------------------------------------------------------------------
-- Core Tables
-- -----------------------------------------------------------------------------

-- Lab Templates (cached from YAML files)
CREATE TABLE lab_templates (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(128) NOT NULL UNIQUE,
    description TEXT,
    version VARCHAR(32) NOT NULL DEFAULT '1.0.0',
    platform platform_type NOT NULL DEFAULT 'proxmox',
    duration_minutes INT,
    difficulty VARCHAR(32),
    max_points INT NOT NULL DEFAULT 0,
    pass_threshold INT NOT NULL DEFAULT 70,
    spec JSONB NOT NULL,  -- Full template spec as JSON
    checkpoints JSONB,    -- Checkpoint definitions
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    is_active BOOLEAN NOT NULL DEFAULT true
);

CREATE INDEX idx_lab_templates_name ON lab_templates(name);
CREATE INDEX idx_lab_templates_platform ON lab_templates(platform);
CREATE INDEX idx_lab_templates_active ON lab_templates(is_active) WHERE is_active = true;

-- Users (synced from FreeIPA)
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    external_id VARCHAR(255) NOT NULL UNIQUE,  -- FreeIPA uid
    username VARCHAR(128) NOT NULL UNIQUE,
    email VARCHAR(255),
    display_name VARCHAR(255),
    role VARCHAR(64) NOT NULL DEFAULT 'student',  -- student, instructor, admin
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_login_at TIMESTAMPTZ,
    is_active BOOLEAN NOT NULL DEFAULT true,
    metadata JSONB DEFAULT '{}'::jsonb
);

CREATE INDEX idx_users_external_id ON users(external_id);
CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_role ON users(role);

-- Pods (instantiated labs)
CREATE TABLE pods (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    lab_template_id UUID NOT NULL REFERENCES lab_templates(id),
    owner_id UUID NOT NULL REFERENCES users(id),
    platform platform_type NOT NULL,
    status pod_status NOT NULL DEFAULT 'provisioning',
    vms JSONB NOT NULL DEFAULT '[]'::jsonb,       -- Array of VM details
    networks JSONB NOT NULL DEFAULT '[]'::jsonb,  -- Array of network details
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ,
    destroyed_at TIMESTAMPTZ,
    metadata JSONB DEFAULT '{}'::jsonb
);

CREATE INDEX idx_pods_owner ON pods(owner_id);
CREATE INDEX idx_pods_template ON pods(lab_template_id);
CREATE INDEX idx_pods_status ON pods(status);
CREATE INDEX idx_pods_expires ON pods(expires_at) WHERE expires_at IS NOT NULL;

-- -----------------------------------------------------------------------------
-- Lab Sessions and Progress
-- -----------------------------------------------------------------------------

-- Lab Sessions (ties a user's work to a pod and optionally Canvas)
CREATE TABLE lab_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    pod_id UUID NOT NULL REFERENCES pods(id),
    user_id UUID NOT NULL REFERENCES users(id),
    lab_template_id UUID NOT NULL REFERENCES lab_templates(id),

    -- Canvas LTI integration
    canvas_course_id VARCHAR(128),
    canvas_assignment_id VARCHAR(128),
    canvas_submission_id VARCHAR(128),
    canvas_user_id VARCHAR(128),

    -- Timing
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at TIMESTAMPTZ,
    due_at TIMESTAMPTZ,

    -- Scoring
    max_points INT NOT NULL DEFAULT 0,
    earned_points INT NOT NULL DEFAULT 0,
    percentage DECIMAL(5,2) NOT NULL DEFAULT 0.00,
    passed BOOLEAN NOT NULL DEFAULT false,

    -- Grade sync
    grade_synced_at TIMESTAMPTZ,
    grade_sync_error TEXT,

    metadata JSONB DEFAULT '{}'::jsonb
);

CREATE INDEX idx_sessions_pod ON lab_sessions(pod_id);
CREATE INDEX idx_sessions_user ON lab_sessions(user_id);
CREATE INDEX idx_sessions_template ON lab_sessions(lab_template_id);
CREATE INDEX idx_sessions_canvas ON lab_sessions(canvas_assignment_id) WHERE canvas_assignment_id IS NOT NULL;
CREATE INDEX idx_sessions_active ON lab_sessions(ended_at) WHERE ended_at IS NULL;

-- Checkpoint Progress (per-session checkpoint state)
CREATE TABLE checkpoint_progress (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id UUID NOT NULL REFERENCES lab_sessions(id) ON DELETE CASCADE,
    checkpoint_id VARCHAR(64) NOT NULL,  -- References checkpoint ID in template

    status checkpoint_status NOT NULL DEFAULT 'pending',
    points INT NOT NULL DEFAULT 0,
    earned_points INT NOT NULL DEFAULT 0,

    passed_at TIMESTAMPTZ,
    triggered_by_event_id BIGINT,  -- References events table

    attempt_count INT NOT NULL DEFAULT 0,
    last_attempt_at TIMESTAMPTZ,
    feedback TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(session_id, checkpoint_id)
);

CREATE INDEX idx_checkpoint_progress_session ON checkpoint_progress(session_id);
CREATE INDEX idx_checkpoint_progress_status ON checkpoint_progress(status);

-- -----------------------------------------------------------------------------
-- Event Storage (Wazuh events via NATS)
-- -----------------------------------------------------------------------------

-- Events table (append-only, high-volume)
CREATE TABLE events (
    id BIGSERIAL,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Context
    pod_id UUID NOT NULL,
    session_id UUID,
    vm_name VARCHAR(64) NOT NULL,
    agent_id VARCHAR(128) NOT NULL,

    -- Event details
    event_type VARCHAR(64) NOT NULL,
    rule_id INT,
    rule_level INT,
    description TEXT,
    location VARCHAR(255),

    -- Flexible payload
    data JSONB NOT NULL DEFAULT '{}'::jsonb,

    -- Processing
    processed BOOLEAN NOT NULL DEFAULT false,
    matched_checkpoints TEXT[],  -- Array of checkpoint IDs this event triggered

    PRIMARY KEY (timestamp, id)
);

CREATE INDEX idx_events_pod ON events(pod_id);
CREATE INDEX idx_events_session ON events(session_id) WHERE session_id IS NOT NULL;
CREATE INDEX idx_events_agent ON events(agent_id);
CREATE INDEX idx_events_type ON events(event_type);
CREATE INDEX idx_events_unprocessed ON events(processed) WHERE processed = false;
CREATE INDEX idx_events_data ON events USING GIN(data);

-- -----------------------------------------------------------------------------
-- Wazuh Agent Registry
-- -----------------------------------------------------------------------------

CREATE TABLE wazuh_agents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id VARCHAR(128) NOT NULL UNIQUE,  -- Wazuh agent ID
    agent_name VARCHAR(128) NOT NULL,
    pod_id UUID REFERENCES pods(id) ON DELETE SET NULL,
    vm_name VARCHAR(64),
    ip_address INET,
    os_name VARCHAR(128),
    os_version VARCHAR(64),
    status VARCHAR(32) NOT NULL DEFAULT 'pending',  -- pending, active, disconnected
    registered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ,
    config JSONB DEFAULT '{}'::jsonb
);

CREATE INDEX idx_wazuh_agents_pod ON wazuh_agents(pod_id);
CREATE INDEX idx_wazuh_agents_status ON wazuh_agents(status);

-- -----------------------------------------------------------------------------
-- Audit and History
-- -----------------------------------------------------------------------------

-- Grade history (for compliance and debugging)
CREATE TABLE grade_history (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id UUID NOT NULL REFERENCES lab_sessions(id),
    earned_points INT NOT NULL,
    max_points INT NOT NULL,
    percentage DECIMAL(5,2) NOT NULL,
    passed BOOLEAN NOT NULL,
    checkpoint_snapshot JSONB NOT NULL,  -- Snapshot of checkpoint states
    synced_to_canvas BOOLEAN NOT NULL DEFAULT false,
    canvas_response JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by VARCHAR(128)  -- user or system
);

CREATE INDEX idx_grade_history_session ON grade_history(session_id);
CREATE INDEX idx_grade_history_created ON grade_history(created_at);

-- System audit log
CREATE TABLE audit_log (
    id BIGSERIAL PRIMARY KEY,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    actor_id UUID REFERENCES users(id),
    actor_type VARCHAR(32) NOT NULL,  -- user, system, api
    action VARCHAR(64) NOT NULL,
    resource_type VARCHAR(64) NOT NULL,
    resource_id UUID,
    details JSONB,
    ip_address INET,
    user_agent TEXT
);

CREATE INDEX idx_audit_log_actor ON audit_log(actor_id);
CREATE INDEX idx_audit_log_action ON audit_log(action);
CREATE INDEX idx_audit_log_resource ON audit_log(resource_type, resource_id);
CREATE INDEX idx_audit_log_timestamp ON audit_log(timestamp);

-- -----------------------------------------------------------------------------
-- Assessment Results (Packet Tracer-style)
-- -----------------------------------------------------------------------------

CREATE TABLE assessment_results (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id UUID NOT NULL REFERENCES lab_sessions(id) ON DELETE CASCADE,

    -- Scoring
    score INT NOT NULL DEFAULT 0,
    max_score INT NOT NULL DEFAULT 0,
    percentage DECIMAL(5,2) NOT NULL DEFAULT 0.00,
    item_count INT NOT NULL DEFAULT 0,
    passed_count INT NOT NULL DEFAULT 0,

    -- Status
    status VARCHAR(32) NOT NULL DEFAULT 'in_progress',  -- in_progress, completed, graded

    -- Timing
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_checked TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,

    -- Detailed results stored as JSON
    components JSONB NOT NULL DEFAULT '[]'::jsonb,  -- ComponentResult array
    devices JSONB NOT NULL DEFAULT '[]'::jsonb,     -- DeviceResult array

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Only one active result per session
    UNIQUE(session_id)
);

CREATE INDEX idx_assessment_results_session ON assessment_results(session_id);
CREATE INDEX idx_assessment_results_status ON assessment_results(status);
CREATE INDEX idx_assessment_results_updated ON assessment_results(updated_at);

-- -----------------------------------------------------------------------------
-- Helper Functions
-- -----------------------------------------------------------------------------

-- Update session progress when checkpoint changes
CREATE OR REPLACE FUNCTION update_session_progress()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE lab_sessions
    SET
        earned_points = (
            SELECT COALESCE(SUM(earned_points), 0)
            FROM checkpoint_progress
            WHERE session_id = NEW.session_id
        ),
        percentage = (
            SELECT CASE
                WHEN max_points > 0 THEN
                    ROUND((COALESCE(SUM(earned_points), 0)::DECIMAL / max_points) * 100, 2)
                ELSE 0
            END
            FROM checkpoint_progress
            WHERE session_id = NEW.session_id
        ),
        passed = (
            SELECT CASE
                WHEN max_points > 0 THEN
                    (COALESCE(SUM(earned_points), 0)::DECIMAL / max_points) * 100 >=
                    (SELECT pass_threshold FROM lab_templates lt
                     JOIN lab_sessions ls ON ls.lab_template_id = lt.id
                     WHERE ls.id = NEW.session_id)
                ELSE false
            END
            FROM checkpoint_progress
            WHERE session_id = NEW.session_id
        )
    WHERE id = NEW.session_id;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_session_progress
AFTER INSERT OR UPDATE ON checkpoint_progress
FOR EACH ROW
EXECUTE FUNCTION update_session_progress();

-- Auto-update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_lab_templates_updated
BEFORE UPDATE ON lab_templates
FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER trigger_users_updated
BEFORE UPDATE ON users
FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER trigger_checkpoint_progress_updated
BEFORE UPDATE ON checkpoint_progress
FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER trigger_assessment_results_updated
BEFORE UPDATE ON assessment_results
FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- -----------------------------------------------------------------------------
-- Initial Data
-- -----------------------------------------------------------------------------

-- Create system user for automated actions
INSERT INTO users (id, external_id, username, email, display_name, role)
VALUES (
    '00000000-0000-0000-0000-000000000000',
    'system',
    'system',
    'system@irqstudio.com',
    'System',
    'admin'
);

RAISE NOTICE 'Database schema initialized successfully';
