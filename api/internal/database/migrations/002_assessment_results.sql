-- Assessment Results Schema
-- Migration: Store Packet Tracer-style assessment results

-- -----------------------------------------------------------------------------
-- Assessment Results Table
-- -----------------------------------------------------------------------------

-- Stores the results of network assessment checks for sessions
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

-- Trigger for auto-updating updated_at
CREATE TRIGGER trigger_assessment_results_updated
BEFORE UPDATE ON assessment_results
FOR EACH ROW EXECUTE FUNCTION update_updated_at();
