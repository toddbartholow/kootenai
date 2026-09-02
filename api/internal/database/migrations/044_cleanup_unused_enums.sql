-- Migration 044: Drop unused ENUM types
-- These ENUMs were created in 007_schema_constraints.sql but never applied to columns.
-- Dropping them to avoid confusion.

DROP TYPE IF EXISTS wazuh_agent_status;
DROP TYPE IF EXISTS grade_sync_status;
DROP TYPE IF EXISTS assessment_status;
