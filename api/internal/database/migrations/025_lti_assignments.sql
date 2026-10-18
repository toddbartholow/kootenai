-- Migration: 025_lti_assignments.sql
-- Description: Add LTI deep linking assignment-to-template mapping
-- Created: 2024-01-09

-- +goose Up

-- LTI Assignments table: Maps Canvas assignments to lab templates via deep linking
CREATE TABLE IF NOT EXISTS lti_assignments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

    -- Canvas identifiers
    canvas_course_id VARCHAR(128) NOT NULL,
    resource_link_id VARCHAR(255) NOT NULL,  -- LTI resource link ID from deep linking

    -- Lab template link
    lab_template_id UUID NOT NULL REFERENCES lab_templates(id) ON DELETE RESTRICT,

    -- Configuration
    custom_title VARCHAR(255),  -- Optional custom name for assignment
    max_points DECIMAL(10,2) NOT NULL DEFAULT 100,

    -- Multi-tenancy
    organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,

    -- Audit fields
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- LTI metadata
    deployment_id VARCHAR(255),
    lti_version VARCHAR(10) DEFAULT '1.3',

    -- Ensure unique mapping per course
    UNIQUE(canvas_course_id, resource_link_id)
);

-- Indexes for common queries
CREATE INDEX idx_lti_assignments_course ON lti_assignments(canvas_course_id);
CREATE INDEX idx_lti_assignments_template ON lti_assignments(lab_template_id);
CREATE INDEX idx_lti_assignments_resource_link ON lti_assignments(resource_link_id);
CREATE INDEX idx_lti_assignments_org ON lti_assignments(organization_id);

-- Add lti_assignment_id to lab_sessions for linking
ALTER TABLE lab_sessions
ADD COLUMN IF NOT EXISTS lti_assignment_id UUID REFERENCES lti_assignments(id) ON DELETE SET NULL;

CREATE INDEX idx_sessions_lti_assignment ON lab_sessions(lti_assignment_id);

-- Update trigger for updated_at
CREATE OR REPLACE FUNCTION update_lti_assignments_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_lti_assignments_updated_at
    BEFORE UPDATE ON lti_assignments
    FOR EACH ROW
    EXECUTE FUNCTION update_lti_assignments_updated_at();

-- +goose Down

DROP TRIGGER IF EXISTS trigger_lti_assignments_updated_at ON lti_assignments;
DROP FUNCTION IF EXISTS update_lti_assignments_updated_at();
DROP INDEX IF EXISTS idx_sessions_lti_assignment;
ALTER TABLE lab_sessions DROP COLUMN IF EXISTS lti_assignment_id;
DROP INDEX IF EXISTS idx_lti_assignments_org;
DROP INDEX IF EXISTS idx_lti_assignments_resource_link;
DROP INDEX IF EXISTS idx_lti_assignments_template;
DROP INDEX IF EXISTS idx_lti_assignments_course;
DROP TABLE IF EXISTS lti_assignments;
