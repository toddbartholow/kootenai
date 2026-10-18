-- 014_certificates.sql
-- Persistent storage for pathway completion certificates
-- Enables verification lookup and certificate history

-- +goose Up

CREATE TABLE certificates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    enrollment_id UUID NOT NULL REFERENCES pathway_enrollments(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    pathway_id UUID NOT NULL REFERENCES pathways(id) ON DELETE CASCADE,
    verification_code VARCHAR(32) NOT NULL,
    issued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    revocation_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT certificates_enrollment_id_unique UNIQUE(enrollment_id),
    CONSTRAINT certificates_verification_code_unique UNIQUE(verification_code)
);

-- Index for fast user lookups
CREATE INDEX idx_certificates_user_id ON certificates(user_id);

-- Index for verification code lookups (already has unique constraint but explicit index helps)
CREATE INDEX idx_certificates_verification_code ON certificates(verification_code);

-- Index for pathway lookups
CREATE INDEX idx_certificates_pathway_id ON certificates(pathway_id);

-- Comment on table
COMMENT ON TABLE certificates IS 'Pathway completion certificates with verification codes';
COMMENT ON COLUMN certificates.verification_code IS 'Unique code for public certificate verification';
COMMENT ON COLUMN certificates.revoked_at IS 'When the certificate was revoked (null if valid)';
COMMENT ON COLUMN certificates.revocation_reason IS 'Reason for certificate revocation';

-- +goose Down

DROP TABLE IF EXISTS certificates;
