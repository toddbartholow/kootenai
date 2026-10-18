-- Add demo user for development/demo mode
-- This user is used when AUTH_DEMO_MODE=true

INSERT INTO users (id, external_id, username, email, display_name, role)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    'demo',
    'demo',
    'demo@example.com',
    'Demo User',
    'student'
)
ON CONFLICT (id) DO NOTHING;
