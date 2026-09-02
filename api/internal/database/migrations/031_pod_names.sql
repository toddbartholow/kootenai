-- Migration: Add human-readable names to pods
-- Names follow adjective-verb-noun pattern like "cryptic-waddling-rain"

-- Add name column for human-readable pod names
ALTER TABLE pods ADD COLUMN IF NOT EXISTS name VARCHAR(64);

-- Create unique index for name lookups (partial - allows NULL for existing pods)
CREATE UNIQUE INDEX IF NOT EXISTS idx_pods_name ON pods(name) WHERE name IS NOT NULL;

-- Add index for name lookups
CREATE INDEX IF NOT EXISTS idx_pods_name_lookup ON pods(name) WHERE name IS NOT NULL;
