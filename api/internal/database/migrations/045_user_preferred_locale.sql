-- Migration 045: Add preferred_locale column to users
--
-- Nullable BCP-47 tag (e.g. "en", "es", "es-MX"). NULL means "fall back to
-- Accept-Language". The CHECK constraint keeps rogue values out without
-- hard-coding the supported set here; app-level validation narrows further
-- to the catalogs actually shipped.

ALTER TABLE users ADD COLUMN preferred_locale VARCHAR(16);

ALTER TABLE users ADD CONSTRAINT users_preferred_locale_format CHECK (
    preferred_locale IS NULL
    OR preferred_locale ~ '^[a-z]{2,3}(-[A-Z]{2})?$'
);
