-- Migration: Add password authentication support to users table
-- This migration adds password hashing and related fields to enable
-- local password-based authentication alongside external identity providers.

-- Add password support columns to users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash VARCHAR(60);
ALTER TABLE users ADD COLUMN IF NOT EXISTS must_change_password BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_updated_at TIMESTAMPTZ;

-- Add comments for documentation
COMMENT ON COLUMN users.password_hash IS 'bcrypt hashed password (60 chars), nullable for external auth users';
COMMENT ON COLUMN users.must_change_password IS 'If true, user must change password on next login';
COMMENT ON COLUMN users.password_updated_at IS 'Timestamp of last password change';

-- Set a default password for the demo user (password: "Demo123!")
-- This allows testing with demo mode disabled
UPDATE users
SET password_hash = '$2a$12$LQv3c1yqBWVHxkd0LHAkCOYz6TtxMQJqhN8/X4.F1oJAo1xq4qpAa',
    password_updated_at = NOW()
WHERE id = '00000000-0000-0000-0000-000000000001';
