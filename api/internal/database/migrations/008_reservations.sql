-- 008_reservations.sql
-- Reservation and scheduling system

-- +migrate Up

-- Reservation status enum
DO $$ BEGIN
    CREATE TYPE reservation_status AS ENUM (
        'pending',
        'confirmed',
        'active',
        'completed',
        'cancelled',
        'expired'
    );
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- Reservations table
CREATE TABLE IF NOT EXISTS reservations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    lab_template_id UUID NOT NULL REFERENCES lab_templates(id) ON DELETE CASCADE,
    pod_id UUID REFERENCES pods(id) ON DELETE SET NULL,
    status reservation_status NOT NULL DEFAULT 'pending',
    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ NOT NULL,
    duration_minutes INTEGER NOT NULL,
    cpu_cores INTEGER NOT NULL DEFAULT 4,
    memory_gb INTEGER NOT NULL DEFAULT 8,
    storage_gb INTEGER NOT NULL DEFAULT 60,
    notes TEXT,
    cancel_reason TEXT,
    cancelled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Ensure end_time is after start_time
    CONSTRAINT valid_time_range CHECK (end_time > start_time),

    -- Ensure duration matches time range
    CONSTRAINT valid_duration CHECK (
        duration_minutes = EXTRACT(EPOCH FROM (end_time - start_time)) / 60
    )
);

-- Indexes for common queries
CREATE INDEX idx_reservations_user_id ON reservations(user_id);
CREATE INDEX idx_reservations_lab_template_id ON reservations(lab_template_id);
CREATE INDEX idx_reservations_status ON reservations(status);
CREATE INDEX idx_reservations_start_time ON reservations(start_time);
CREATE INDEX idx_reservations_end_time ON reservations(end_time);
CREATE INDEX idx_reservations_time_range ON reservations(start_time, end_time);

-- Index for finding overlapping reservations (conflict detection)
CREATE INDEX idx_reservations_active_time ON reservations(start_time, end_time)
    WHERE status IN ('pending', 'confirmed', 'active');

-- Trigger to update updated_at
CREATE OR REPLACE FUNCTION update_reservation_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_reservation_updated_at
    BEFORE UPDATE ON reservations
    FOR EACH ROW
    EXECUTE FUNCTION update_reservation_updated_at();

-- Resource capacity configuration table (optional, for future use)
CREATE TABLE IF NOT EXISTS resource_capacity (
    id SERIAL PRIMARY KEY,
    platform VARCHAR(50) NOT NULL,
    max_concurrent_pods INTEGER NOT NULL DEFAULT 50,
    max_cpu_cores INTEGER NOT NULL DEFAULT 200,
    max_memory_gb INTEGER NOT NULL DEFAULT 500,
    max_storage_gb INTEGER NOT NULL DEFAULT 5000,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Insert default capacity for Proxmox
INSERT INTO resource_capacity (platform, max_concurrent_pods, max_cpu_cores, max_memory_gb, max_storage_gb)
VALUES ('proxmox', 20, 80, 160, 1000)
ON CONFLICT DO NOTHING;

-- +migrate Down
DROP TRIGGER IF EXISTS trigger_reservation_updated_at ON reservations;
DROP FUNCTION IF EXISTS update_reservation_updated_at();
DROP TABLE IF EXISTS resource_capacity;
DROP TABLE IF EXISTS reservations;
DROP TYPE IF EXISTS reservation_status;
