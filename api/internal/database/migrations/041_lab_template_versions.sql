-- Lab Template Versioning
-- Auto-creates version snapshots when templates are updated

CREATE TABLE IF NOT EXISTS lab_template_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    lab_template_id UUID NOT NULL REFERENCES lab_templates(id) ON DELETE CASCADE,
    version_number INTEGER NOT NULL,
    -- Snapshot of all template fields
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    description TEXT,
    version TEXT NOT NULL,
    platform TEXT NOT NULL,
    duration_minutes INTEGER,
    difficulty TEXT,
    category TEXT,
    tags JSONB,
    max_points INTEGER NOT NULL DEFAULT 0,
    pass_threshold INTEGER NOT NULL DEFAULT 70,
    spec JSONB,
    checkpoints JSONB,
    instructions JSONB,
    is_active BOOLEAN NOT NULL DEFAULT true,
    organization_id UUID,
    visibility TEXT NOT NULL DEFAULT 'global',
    created_by UUID,
    min_edition TEXT,
    -- Version metadata
    change_summary TEXT,
    created_by_user_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (lab_template_id, version_number)
);

CREATE INDEX idx_lab_template_versions_template_id ON lab_template_versions(lab_template_id);
CREATE INDEX idx_lab_template_versions_created_at ON lab_template_versions(created_at);

-- Function to get next version number atomically
CREATE OR REPLACE FUNCTION get_next_version_number(p_template_id UUID)
RETURNS INTEGER AS $$
DECLARE
    next_num INTEGER;
BEGIN
    SELECT COALESCE(MAX(version_number), 0) + 1
    INTO next_num
    FROM lab_template_versions
    WHERE lab_template_id = p_template_id;
    RETURN next_num;
END;
$$ LANGUAGE plpgsql;
