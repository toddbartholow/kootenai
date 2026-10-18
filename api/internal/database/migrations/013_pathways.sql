-- Pathways Schema
-- Learning pathways/tracks containing modules of labs
-- Supports enrollment, progress tracking, and achievements

-- =============================================================================
-- EXTEND LAB_TEMPLATES TABLE
-- =============================================================================

-- Add slug column for URL-friendly identifiers
ALTER TABLE lab_templates ADD COLUMN IF NOT EXISTS slug VARCHAR(128);

-- Populate slug from name for existing records
UPDATE lab_templates SET slug = LOWER(REPLACE(name, ' ', '-')) WHERE slug IS NULL;

-- Make slug unique and not null
ALTER TABLE lab_templates ALTER COLUMN slug SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_lab_templates_slug ON lab_templates(slug);

-- Add tags for categorization and filtering
ALTER TABLE lab_templates ADD COLUMN IF NOT EXISTS tags TEXT[];
CREATE INDEX IF NOT EXISTS idx_lab_templates_tags ON lab_templates USING GIN(tags);

-- Add visibility for multi-tenancy
ALTER TABLE lab_templates ADD COLUMN IF NOT EXISTS visibility lab_visibility NOT NULL DEFAULT 'global';
CREATE INDEX IF NOT EXISTS idx_lab_templates_visibility ON lab_templates(visibility);

-- =============================================================================
-- ENUM TYPES
-- =============================================================================

CREATE TYPE pathway_status AS ENUM ('draft', 'published', 'archived');
CREATE TYPE enrollment_status AS ENUM ('enrolled', 'in_progress', 'completed', 'abandoned');
CREATE TYPE module_status AS ENUM ('locked', 'unlocked', 'in_progress', 'completed');
CREATE TYPE unlock_type AS ENUM ('sequential', 'all_previous', 'manual', 'always');

-- =============================================================================
-- PATHWAYS (Learning Tracks)
-- =============================================================================

CREATE TABLE pathways (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(128) NOT NULL UNIQUE,
    description TEXT,
    short_description VARCHAR(500),

    -- Classification
    difficulty VARCHAR(32),  -- beginner, intermediate, advanced, mixed
    estimated_hours INT,

    -- Display ordering
    display_order INT NOT NULL DEFAULT 0,

    -- Status flags
    status pathway_status NOT NULL DEFAULT 'draft',
    is_active BOOLEAN NOT NULL DEFAULT true,
    is_featured BOOLEAN NOT NULL DEFAULT false,

    -- Multi-tenancy
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    visibility lab_visibility NOT NULL DEFAULT 'global',
    created_by UUID REFERENCES users(id),

    -- Prerequisites (array of pathway IDs that must be completed first)
    prerequisites JSONB DEFAULT '[]'::jsonb,

    -- Tags for filtering/search
    tags TEXT[],

    -- Metadata
    icon VARCHAR(64),  -- icon class name
    color VARCHAR(32), -- theme color
    cover_image_url VARCHAR(512),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_pathways_slug ON pathways(slug);
CREATE INDEX idx_pathways_status ON pathways(status);
CREATE INDEX idx_pathways_active ON pathways(is_active) WHERE is_active = true;
CREATE INDEX idx_pathways_featured ON pathways(is_featured) WHERE is_featured = true;
CREATE INDEX idx_pathways_org ON pathways(organization_id) WHERE organization_id IS NOT NULL;
CREATE INDEX idx_pathways_visibility ON pathways(visibility);
CREATE INDEX idx_pathways_display_order ON pathways(display_order);
CREATE INDEX idx_pathways_tags ON pathways USING GIN(tags);

CREATE TRIGGER trigger_pathways_updated
BEFORE UPDATE ON pathways
FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- =============================================================================
-- PATHWAY MODULES (Grouped Sections)
-- =============================================================================

CREATE TABLE pathway_modules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    pathway_id UUID NOT NULL REFERENCES pathways(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(128) NOT NULL,
    description TEXT,

    -- Ordering within pathway
    display_order INT NOT NULL DEFAULT 0,

    -- How this module unlocks
    unlock_type unlock_type NOT NULL DEFAULT 'sequential',

    -- Optional: specific modules required (for 'all_previous' type)
    required_module_ids UUID[],

    -- Status
    is_active BOOLEAN NOT NULL DEFAULT true,

    -- Metadata
    icon VARCHAR(64),
    estimated_minutes INT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(pathway_id, slug)
);

CREATE INDEX idx_pathway_modules_pathway ON pathway_modules(pathway_id);
CREATE INDEX idx_pathway_modules_order ON pathway_modules(pathway_id, display_order);
CREATE INDEX idx_pathway_modules_active ON pathway_modules(is_active) WHERE is_active = true;

-- =============================================================================
-- MODULE LABS (Junction Table)
-- =============================================================================

CREATE TABLE module_labs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    module_id UUID NOT NULL REFERENCES pathway_modules(id) ON DELETE CASCADE,
    lab_template_id UUID NOT NULL REFERENCES lab_templates(id) ON DELETE CASCADE,

    -- Ordering within module
    display_order INT NOT NULL DEFAULT 0,

    -- Whether this lab is required to complete the module
    is_required BOOLEAN NOT NULL DEFAULT true,

    -- Optional: override pass threshold for this specific context
    pass_threshold_override INT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(module_id, lab_template_id)
);

CREATE INDEX idx_module_labs_module ON module_labs(module_id);
CREATE INDEX idx_module_labs_template ON module_labs(lab_template_id);
CREATE INDEX idx_module_labs_order ON module_labs(module_id, display_order);

-- =============================================================================
-- PATHWAY ENROLLMENTS (User Enrollment)
-- =============================================================================

CREATE TABLE pathway_enrollments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    pathway_id UUID NOT NULL REFERENCES pathways(id) ON DELETE CASCADE,

    -- Overall status
    status enrollment_status NOT NULL DEFAULT 'enrolled',

    -- Progress tracking (denormalized for quick access)
    completed_modules INT NOT NULL DEFAULT 0,
    total_modules INT NOT NULL DEFAULT 0,

    -- Points tracking
    earned_points INT NOT NULL DEFAULT 0,
    max_points INT NOT NULL DEFAULT 0,
    percentage DECIMAL(5,2) NOT NULL DEFAULT 0.00,

    -- Timing
    enrolled_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    last_activity_at TIMESTAMPTZ,

    -- Multi-tenancy context
    organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,

    -- Certificate tracking
    certificate_issued BOOLEAN NOT NULL DEFAULT false,
    certificate_url VARCHAR(512),

    UNIQUE(user_id, pathway_id)
);

CREATE INDEX idx_pathway_enrollments_user ON pathway_enrollments(user_id);
CREATE INDEX idx_pathway_enrollments_pathway ON pathway_enrollments(pathway_id);
CREATE INDEX idx_pathway_enrollments_status ON pathway_enrollments(status);
CREATE INDEX idx_pathway_enrollments_org ON pathway_enrollments(organization_id)
    WHERE organization_id IS NOT NULL;
CREATE INDEX idx_pathway_enrollments_completed ON pathway_enrollments(completed_at)
    WHERE completed_at IS NOT NULL;

-- =============================================================================
-- MODULE PROGRESS (Per-Module Tracking)
-- =============================================================================

CREATE TABLE module_progress (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    enrollment_id UUID NOT NULL REFERENCES pathway_enrollments(id) ON DELETE CASCADE,
    module_id UUID NOT NULL REFERENCES pathway_modules(id) ON DELETE CASCADE,

    -- Status
    status module_status NOT NULL DEFAULT 'locked',

    -- Progress tracking
    completed_labs INT NOT NULL DEFAULT 0,
    total_labs INT NOT NULL DEFAULT 0,

    -- Points tracking
    earned_points INT NOT NULL DEFAULT 0,
    max_points INT NOT NULL DEFAULT 0,

    -- Timing
    unlocked_at TIMESTAMPTZ,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,

    UNIQUE(enrollment_id, module_id)
);

CREATE INDEX idx_module_progress_enrollment ON module_progress(enrollment_id);
CREATE INDEX idx_module_progress_module ON module_progress(module_id);
CREATE INDEX idx_module_progress_status ON module_progress(status);

-- =============================================================================
-- LAB PROGRESS (Per-Lab Tracking within Pathway)
-- =============================================================================

CREATE TABLE lab_progress (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    enrollment_id UUID NOT NULL REFERENCES pathway_enrollments(id) ON DELETE CASCADE,
    module_id UUID NOT NULL REFERENCES pathway_modules(id) ON DELETE CASCADE,
    lab_template_id UUID NOT NULL REFERENCES lab_templates(id) ON DELETE CASCADE,

    -- Link to best session
    best_session_id UUID REFERENCES lab_sessions(id) ON DELETE SET NULL,

    -- Attempt tracking
    attempt_count INT NOT NULL DEFAULT 0,

    -- Score tracking (from best session)
    best_score INT NOT NULL DEFAULT 0,
    max_points INT NOT NULL DEFAULT 0,

    -- Completion
    passed BOOLEAN NOT NULL DEFAULT false,

    -- Timing
    first_attempt_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,

    UNIQUE(enrollment_id, lab_template_id)
);

CREATE INDEX idx_lab_progress_enrollment ON lab_progress(enrollment_id);
CREATE INDEX idx_lab_progress_module ON lab_progress(module_id);
CREATE INDEX idx_lab_progress_template ON lab_progress(lab_template_id);
CREATE INDEX idx_lab_progress_session ON lab_progress(best_session_id)
    WHERE best_session_id IS NOT NULL;
CREATE INDEX idx_lab_progress_passed ON lab_progress(passed) WHERE passed = true;

-- =============================================================================
-- HELPER FUNCTIONS
-- =============================================================================

-- Calculate and update enrollment progress when module progress changes
CREATE OR REPLACE FUNCTION update_enrollment_progress()
RETURNS TRIGGER AS $$
DECLARE
    v_enrollment_id UUID;
    v_completed_modules INT;
    v_total_modules INT;
    v_earned_points INT;
    v_max_points INT;
    v_percentage DECIMAL(5,2);
    v_all_completed BOOLEAN;
BEGIN
    v_enrollment_id := COALESCE(NEW.enrollment_id, OLD.enrollment_id);

    -- Calculate totals from module_progress
    SELECT
        COUNT(*) FILTER (WHERE status = 'completed'),
        COUNT(*),
        COALESCE(SUM(earned_points), 0),
        COALESCE(SUM(max_points), 0)
    INTO v_completed_modules, v_total_modules, v_earned_points, v_max_points
    FROM module_progress
    WHERE enrollment_id = v_enrollment_id;

    -- Calculate percentage
    IF v_max_points > 0 THEN
        v_percentage := ROUND((v_earned_points::DECIMAL / v_max_points) * 100, 2);
    ELSE
        v_percentage := 0;
    END IF;

    -- Check if all modules are completed
    v_all_completed := (v_completed_modules = v_total_modules AND v_total_modules > 0);

    -- Update enrollment
    UPDATE pathway_enrollments
    SET
        completed_modules = v_completed_modules,
        total_modules = v_total_modules,
        earned_points = v_earned_points,
        max_points = v_max_points,
        percentage = v_percentage,
        status = CASE
            WHEN v_all_completed THEN 'completed'::enrollment_status
            WHEN v_completed_modules > 0 OR EXISTS (
                SELECT 1 FROM module_progress
                WHERE enrollment_id = v_enrollment_id AND status = 'in_progress'
            ) THEN 'in_progress'::enrollment_status
            ELSE status
        END,
        completed_at = CASE
            WHEN v_all_completed AND completed_at IS NULL THEN NOW()
            ELSE completed_at
        END,
        last_activity_at = NOW()
    WHERE id = v_enrollment_id;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_enrollment_progress
AFTER INSERT OR UPDATE OR DELETE ON module_progress
FOR EACH ROW
EXECUTE FUNCTION update_enrollment_progress();

-- Calculate and update module progress when lab progress changes
CREATE OR REPLACE FUNCTION update_module_progress()
RETURNS TRIGGER AS $$
DECLARE
    v_enrollment_id UUID;
    v_module_id UUID;
    v_completed_labs INT;
    v_total_labs INT;
    v_earned_points INT;
    v_max_points INT;
    v_all_required_passed BOOLEAN;
BEGIN
    v_enrollment_id := COALESCE(NEW.enrollment_id, OLD.enrollment_id);
    v_module_id := COALESCE(NEW.module_id, OLD.module_id);

    -- Calculate totals from lab_progress for this module
    SELECT
        COUNT(*) FILTER (WHERE lp.passed = true),
        COUNT(*),
        COALESCE(SUM(lp.best_score), 0),
        COALESCE(SUM(lp.max_points), 0)
    INTO v_completed_labs, v_total_labs, v_earned_points, v_max_points
    FROM lab_progress lp
    WHERE lp.enrollment_id = v_enrollment_id
    AND lp.module_id = v_module_id;

    -- Check if all required labs are passed
    SELECT NOT EXISTS (
        SELECT 1
        FROM module_labs ml
        LEFT JOIN lab_progress lp ON lp.lab_template_id = ml.lab_template_id
            AND lp.enrollment_id = v_enrollment_id
        WHERE ml.module_id = v_module_id
        AND ml.is_required = true
        AND (lp.passed IS NULL OR lp.passed = false)
    ) INTO v_all_required_passed;

    -- Update or insert module progress
    INSERT INTO module_progress (
        enrollment_id, module_id, status, completed_labs, total_labs,
        earned_points, max_points, started_at, completed_at
    )
    VALUES (
        v_enrollment_id, v_module_id,
        CASE
            WHEN v_all_required_passed AND v_total_labs > 0 THEN 'completed'::module_status
            WHEN v_completed_labs > 0 THEN 'in_progress'::module_status
            ELSE 'unlocked'::module_status
        END,
        v_completed_labs, v_total_labs,
        v_earned_points, v_max_points,
        CASE WHEN v_completed_labs > 0 THEN NOW() ELSE NULL END,
        CASE WHEN v_all_required_passed AND v_total_labs > 0 THEN NOW() ELSE NULL END
    )
    ON CONFLICT (enrollment_id, module_id)
    DO UPDATE SET
        completed_labs = EXCLUDED.completed_labs,
        total_labs = EXCLUDED.total_labs,
        earned_points = EXCLUDED.earned_points,
        max_points = EXCLUDED.max_points,
        status = CASE
            WHEN v_all_required_passed AND v_total_labs > 0 THEN 'completed'::module_status
            WHEN v_completed_labs > 0 THEN 'in_progress'::module_status
            ELSE module_progress.status
        END,
        started_at = COALESCE(module_progress.started_at,
            CASE WHEN v_completed_labs > 0 THEN NOW() ELSE NULL END),
        completed_at = CASE
            WHEN v_all_required_passed AND v_total_labs > 0 AND module_progress.completed_at IS NULL
            THEN NOW()
            ELSE module_progress.completed_at
        END;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_module_progress
AFTER INSERT OR UPDATE OR DELETE ON lab_progress
FOR EACH ROW
EXECUTE FUNCTION update_module_progress();

-- Check if a user is enrolled in a pathway
CREATE OR REPLACE FUNCTION is_enrolled_in_pathway(p_user_id UUID, p_pathway_id UUID)
RETURNS BOOLEAN AS $$
BEGIN
    RETURN EXISTS (
        SELECT 1 FROM pathway_enrollments
        WHERE user_id = p_user_id
        AND pathway_id = p_pathway_id
    );
END;
$$ LANGUAGE plpgsql STABLE;

-- Get pathway progress percentage for a user
CREATE OR REPLACE FUNCTION get_pathway_progress(p_user_id UUID, p_pathway_id UUID)
RETURNS DECIMAL(5,2) AS $$
DECLARE
    v_percentage DECIMAL(5,2);
BEGIN
    SELECT percentage INTO v_percentage
    FROM pathway_enrollments
    WHERE user_id = p_user_id
    AND pathway_id = p_pathway_id;

    RETURN COALESCE(v_percentage, 0);
END;
$$ LANGUAGE plpgsql STABLE;

-- =============================================================================
-- VIEWS
-- =============================================================================

-- Pathway summary view with stats
CREATE OR REPLACE VIEW pathway_stats AS
SELECT
    p.id,
    p.name,
    p.slug,
    p.difficulty,
    p.status,
    p.is_featured,
    COUNT(DISTINCT pm.id) AS module_count,
    COUNT(DISTINCT ml.lab_template_id) AS lab_count,
    COALESCE(SUM(lt.max_points), 0) AS total_points,
    COALESCE(SUM(lt.duration_minutes), 0) AS total_duration_minutes,
    COUNT(DISTINCT pe.id) AS enrollment_count,
    COUNT(DISTINCT pe.id) FILTER (WHERE pe.status = 'completed') AS completion_count
FROM pathways p
LEFT JOIN pathway_modules pm ON pm.pathway_id = p.id AND pm.is_active = true
LEFT JOIN module_labs ml ON ml.module_id = pm.id
LEFT JOIN lab_templates lt ON lt.id = ml.lab_template_id AND lt.is_active = true
LEFT JOIN pathway_enrollments pe ON pe.pathway_id = p.id
GROUP BY p.id;
