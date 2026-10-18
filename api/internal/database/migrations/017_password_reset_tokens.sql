-- Password reset tokens table
-- Stores secure tokens for password recovery flow

CREATE TABLE IF NOT EXISTS password_reset_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(64) NOT NULL, -- SHA256 hash of the actual token
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for fast token lookup (we hash the token and look up by hash)
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_hash ON password_reset_tokens(token_hash);

-- Index for cleanup of expired tokens
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_expires ON password_reset_tokens(expires_at);

-- Index for finding tokens by user (to invalidate old tokens)
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user ON password_reset_tokens(user_id);

-- Only allow one unused token per user at a time
-- This prevents token spam (expired tokens are cleaned up separately)
CREATE UNIQUE INDEX IF NOT EXISTS idx_password_reset_tokens_active
ON password_reset_tokens(user_id)
WHERE used_at IS NULL;
