-- Migration: 026_question_responses.sql
-- Description: Add question_responses table for question-based lab assessments
-- This complements the existing checkpoint-based objectives with manual Q&A

-- Question response status enum (create if not exists)
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'question_response_status') THEN
        CREATE TYPE question_response_status AS ENUM ('pending', 'correct', 'incorrect', 'partial');
    END IF;
END$$;

-- Question responses table (stores student answers per session)
CREATE TABLE IF NOT EXISTS question_responses (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id UUID NOT NULL REFERENCES lab_sessions(id) ON DELETE CASCADE,
    question_id VARCHAR(64) NOT NULL,  -- References question ID in lab template spec.questions[]

    -- Question metadata (denormalized from template for audit trail)
    question_type VARCHAR(32) NOT NULL,  -- 'text' or 'multiple_choice'
    points INT NOT NULL DEFAULT 0,

    -- Response data
    response_text TEXT,                    -- For text questions
    selected_options JSONB,                -- For multiple choice: ["A", "C"]

    -- Validation result
    status question_response_status NOT NULL DEFAULT 'pending',
    earned_points INT NOT NULL DEFAULT 0,
    is_correct BOOLEAN NOT NULL DEFAULT false,

    -- Attempt tracking
    attempt_count INT NOT NULL DEFAULT 0,
    first_attempt_at TIMESTAMPTZ,
    last_attempt_at TIMESTAMPTZ,
    correct_at TIMESTAMPTZ,

    -- Feedback and hints
    feedback TEXT,
    hint_shown BOOLEAN NOT NULL DEFAULT false,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(session_id, question_id)
);

-- Indexes for common queries (create if not exist)
CREATE INDEX IF NOT EXISTS idx_question_responses_session ON question_responses(session_id);
CREATE INDEX IF NOT EXISTS idx_question_responses_status ON question_responses(status);
CREATE INDEX IF NOT EXISTS idx_question_responses_session_correct ON question_responses(session_id) WHERE is_correct = true;

-- Trigger to auto-update updated_at timestamp
DROP TRIGGER IF EXISTS trigger_question_responses_updated ON question_responses;
CREATE TRIGGER trigger_question_responses_updated
    BEFORE UPDATE ON question_responses
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

-- Function to recalculate session progress including question points
-- This updates lab_sessions.earned_points, percentage, and passed
CREATE OR REPLACE FUNCTION update_session_progress_with_questions()
RETURNS TRIGGER AS $$
DECLARE
    v_checkpoint_points INT;
    v_question_points INT;
    v_total_earned INT;
    v_max_points INT;
    v_pass_threshold INT;
    v_new_percentage DECIMAL(5,2);
BEGIN
    -- Get checkpoint earned points
    SELECT COALESCE(SUM(earned_points), 0) INTO v_checkpoint_points
    FROM checkpoint_progress
    WHERE session_id = NEW.session_id;

    -- Get question earned points
    SELECT COALESCE(SUM(earned_points), 0) INTO v_question_points
    FROM question_responses
    WHERE session_id = NEW.session_id;

    -- Calculate total
    v_total_earned := v_checkpoint_points + v_question_points;

    -- Get session max_points and pass_threshold
    SELECT ls.max_points, lt.pass_threshold
    INTO v_max_points, v_pass_threshold
    FROM lab_sessions ls
    JOIN lab_templates lt ON ls.lab_template_id = lt.id
    WHERE ls.id = NEW.session_id;

    -- Calculate percentage
    IF v_max_points > 0 THEN
        v_new_percentage := ROUND((v_total_earned::DECIMAL / v_max_points) * 100, 2);
    ELSE
        v_new_percentage := 0;
    END IF;

    -- Update session
    UPDATE lab_sessions
    SET
        earned_points = v_total_earned,
        percentage = v_new_percentage,
        passed = (v_new_percentage >= COALESCE(v_pass_threshold, 70))
    WHERE id = NEW.session_id;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to update session progress when question response changes
DROP TRIGGER IF EXISTS trigger_update_session_progress_questions ON question_responses;
CREATE TRIGGER trigger_update_session_progress_questions
    AFTER INSERT OR UPDATE OF earned_points, is_correct ON question_responses
    FOR EACH ROW
    EXECUTE FUNCTION update_session_progress_with_questions();

-- Add comment explaining the table
COMMENT ON TABLE question_responses IS 'Stores student responses to manual questions in lab sessions';
COMMENT ON COLUMN question_responses.question_id IS 'References question ID in lab template spec.questions[] array';
COMMENT ON COLUMN question_responses.selected_options IS 'JSON array of selected option IDs for multiple choice questions';
COMMENT ON COLUMN question_responses.hint_shown IS 'Whether the student has revealed the hint for this question';
