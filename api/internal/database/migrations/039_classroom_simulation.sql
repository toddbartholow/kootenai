-- Classroom simulation tables for AI student orchestration
-- Enterprise feature: ai_classroom

CREATE TABLE classroom_simulations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',  -- pending|running|paused|completed|failed
    canvas_course_id TEXT,
    pathway_id UUID REFERENCES pathways(id),
    config JSONB NOT NULL DEFAULT '{}',
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE ai_students (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    simulation_id UUID NOT NULL REFERENCES classroom_simulations(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id),
    canvas_user_id TEXT,
    name TEXT NOT NULL,
    personality TEXT NOT NULL,  -- high_performer|struggling|industry_professional
    traits JSONB NOT NULL DEFAULT '{}',
    tech_skills JSONB NOT NULL DEFAULT '{}',
    behavioral_config JSONB NOT NULL DEFAULT '{}',
    state JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE ai_student_activities (
    id BIGSERIAL PRIMARY KEY,
    simulation_id UUID NOT NULL REFERENCES classroom_simulations(id) ON DELETE CASCADE,
    student_id UUID NOT NULL REFERENCES ai_students(id) ON DELETE CASCADE,
    activity_type TEXT NOT NULL,  -- lab_start|lab_complete|assignment_submit|discussion_post|quiz_attempt
    target_id TEXT,
    target_name TEXT,
    status TEXT NOT NULL DEFAULT 'completed',
    content TEXT,
    score INTEGER,
    max_score INTEGER,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ai_activities_sim ON ai_student_activities(simulation_id);
CREATE INDEX idx_ai_activities_student ON ai_student_activities(student_id);
CREATE INDEX idx_ai_students_sim ON ai_students(simulation_id);
