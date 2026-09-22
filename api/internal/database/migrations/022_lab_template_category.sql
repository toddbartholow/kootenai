-- Migration: Add category field to lab_templates
-- This enables category-based achievement tracking
--
-- Note: Using regular CREATE INDEX instead of CONCURRENTLY because
-- CONCURRENTLY cannot run inside a transaction block.

-- Add category column to lab_templates
ALTER TABLE lab_templates ADD COLUMN IF NOT EXISTS category VARCHAR(64);

-- Add tags column for flexible categorization (JSONB array)
ALTER TABLE lab_templates ADD COLUMN IF NOT EXISTS tags JSONB DEFAULT '[]'::jsonb;

-- Create index for category queries
CREATE INDEX IF NOT EXISTS idx_lab_templates_category
ON lab_templates(category) WHERE category IS NOT NULL;

-- Create GIN index for tags array queries
CREATE INDEX IF NOT EXISTS idx_lab_templates_tags
ON lab_templates USING GIN (tags);

-- Common categories for reference:
-- 'linux-fundamentals', 'networking', 'security', 'cloud', 'devops',
-- 'programming', 'databases', 'containers', 'monitoring', 'incident-response'
