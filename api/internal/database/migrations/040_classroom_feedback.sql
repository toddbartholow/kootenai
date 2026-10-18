-- +migrate Up
-- Classroom feedback table for AI student lab execution feedback

CREATE TABLE IF NOT EXISTS classroom_feedback (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    simulation_id UUID NOT NULL REFERENCES classroom_simulations(id) ON DELETE CASCADE,
    student_id UUID NOT NULL REFERENCES ai_students(id) ON DELETE CASCADE,
    lab_template_id UUID,
    pathway_id UUID,
    session_id TEXT,
    feedback_type TEXT NOT NULL,
    summary TEXT NOT NULL,
    rating INTEGER CHECK (rating BETWEEN 1 AND 5),
    details JSONB NOT NULL DEFAULT '{}',
    execution_duration_ms INTEGER,
    checkpoints_passed INTEGER,
    checkpoints_total INTEGER,
    score INTEGER,
    max_score INTEGER,
    execution_log TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_classroom_feedback_sim ON classroom_feedback(simulation_id);
CREATE INDEX idx_classroom_feedback_lab ON classroom_feedback(lab_template_id);
CREATE INDEX idx_classroom_feedback_type ON classroom_feedback(feedback_type);
CREATE INDEX idx_classroom_feedback_student ON classroom_feedback(student_id);

-- +migrate Down
DROP TABLE IF EXISTS classroom_feedback;
