-- Migration: Enable time-based partitioning for events table
-- This migration converts the events table to use PostgreSQL native range partitioning
-- by month, improving query performance for high-volume event data.
--
-- IMPORTANT: This migration requires PostgreSQL 11+ for declarative partitioning.
-- The migration is designed to be run during low-traffic periods.

-- Step 1: Create the new partitioned events table
CREATE TABLE events_partitioned (
    id BIGSERIAL,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Context
    pod_id UUID NOT NULL,
    session_id UUID,
    vm_name VARCHAR(64) NOT NULL,
    agent_id VARCHAR(128) NOT NULL,

    -- Event details
    event_type VARCHAR(64) NOT NULL,
    rule_id INT,
    rule_level INT,
    description TEXT,
    location VARCHAR(255),

    -- Flexible payload
    data JSONB NOT NULL DEFAULT '{}'::jsonb,

    -- Processing
    processed BOOLEAN NOT NULL DEFAULT false,
    matched_checkpoints TEXT[],

    PRIMARY KEY (timestamp, id)
) PARTITION BY RANGE (timestamp);

-- Step 2: Create initial partitions for the next 12 months
-- We'll create monthly partitions starting from the current month
DO $$
DECLARE
    start_date DATE := DATE_TRUNC('month', CURRENT_DATE);
    end_date DATE;
    partition_name TEXT;
    i INT;
BEGIN
    -- Create partitions for 12 months ahead
    FOR i IN 0..11 LOOP
        end_date := start_date + INTERVAL '1 month';
        partition_name := 'events_p' || TO_CHAR(start_date, 'YYYY_MM');

        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS %I PARTITION OF events_partitioned
             FOR VALUES FROM (%L) TO (%L)',
            partition_name, start_date, end_date
        );

        start_date := end_date;
    END LOOP;

    -- Create a default partition for any data outside the defined ranges
    -- This catches both old data and future data beyond our partitions
    IF NOT EXISTS (SELECT 1 FROM pg_class WHERE relname = 'events_p_default') THEN
        CREATE TABLE events_p_default PARTITION OF events_partitioned DEFAULT;
    END IF;
END $$;

-- Step 3: Migrate existing data from old table to new partitioned table
INSERT INTO events_partitioned
SELECT * FROM events;

-- Step 4: Rename tables to swap them
ALTER TABLE events RENAME TO events_old;
ALTER TABLE events_partitioned RENAME TO events;

-- Step 5: Recreate indexes on the new partitioned table
-- Indexes are automatically created on each partition
CREATE INDEX IF NOT EXISTS idx_events_pod ON events(pod_id);
CREATE INDEX IF NOT EXISTS idx_events_session ON events(session_id) WHERE session_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_events_unprocessed ON events(processed) WHERE processed = false;
CREATE INDEX IF NOT EXISTS idx_events_data ON events USING GIN(data);
CREATE INDEX IF NOT EXISTS idx_events_type ON events(event_type);
CREATE INDEX IF NOT EXISTS idx_events_timestamp ON events(timestamp);

-- Step 6: Create function to automatically create new partitions
CREATE OR REPLACE FUNCTION create_events_partition_if_not_exists()
RETURNS TRIGGER AS $$
DECLARE
    partition_name TEXT;
    start_date DATE;
    end_date DATE;
BEGIN
    start_date := DATE_TRUNC('month', NEW.timestamp);
    end_date := start_date + INTERVAL '1 month';
    partition_name := 'events_p' || TO_CHAR(start_date, 'YYYY_MM');

    -- Check if partition exists, create if not
    IF NOT EXISTS (
        SELECT 1 FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE c.relname = partition_name AND n.nspname = 'public'
    ) THEN
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS %I PARTITION OF events
             FOR VALUES FROM (%L) TO (%L)',
            partition_name, start_date, end_date
        );
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Note: The trigger is not attached by default because it adds overhead.
-- For production, consider creating partitions ahead of time via a cron job.
-- Uncomment below to enable automatic partition creation:
--
-- CREATE TRIGGER events_partition_trigger
--     BEFORE INSERT ON events
--     FOR EACH ROW
--     EXECUTE FUNCTION create_events_partition_if_not_exists();

-- Step 7: Create a function for partition maintenance
-- This can be called periodically to create future partitions
CREATE OR REPLACE FUNCTION maintain_events_partitions(months_ahead INT DEFAULT 3)
RETURNS void AS $$
DECLARE
    start_date DATE := DATE_TRUNC('month', CURRENT_DATE);
    end_date DATE;
    partition_name TEXT;
    i INT;
BEGIN
    FOR i IN 0..months_ahead LOOP
        end_date := start_date + INTERVAL '1 month';
        partition_name := 'events_p' || TO_CHAR(start_date, 'YYYY_MM');

        -- Create partition if it doesn't exist
        IF NOT EXISTS (
            SELECT 1 FROM pg_class c
            JOIN pg_namespace n ON n.oid = c.relnamespace
            WHERE c.relname = partition_name AND n.nspname = 'public'
        ) THEN
            EXECUTE format(
                'CREATE TABLE IF NOT EXISTS %I PARTITION OF events
                 FOR VALUES FROM (%L) TO (%L)',
                partition_name, start_date, end_date
            );
            RAISE NOTICE 'Created partition: %', partition_name;
        END IF;

        start_date := end_date;
    END LOOP;
END;
$$ LANGUAGE plpgsql;

-- Add comment for documentation
COMMENT ON TABLE events IS 'Partitioned events table - stores high-volume Wazuh/OSSEC events, partitioned by month for improved query performance';
COMMENT ON FUNCTION maintain_events_partitions(INT) IS 'Call periodically (e.g., monthly) to create future partitions: SELECT maintain_events_partitions(3);';

-- Note: The old table (events_old) is kept for safety.
-- After verifying the migration, drop it with:
-- DROP TABLE events_old;
