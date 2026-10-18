-- Migration: Add achievements and badges system
-- This migration creates tables for tracking achievements, user achievements, and progress

-- Achievements table - defines available achievements
CREATE TABLE IF NOT EXISTS achievements (
    id VARCHAR(128) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    type VARCHAR(50) NOT NULL, -- lab_completion, perfect_score, streak, speed, category, milestone, special
    tier VARCHAR(50) NOT NULL DEFAULT 'bronze', -- bronze, silver, gold, platinum, diamond
    icon_url VARCHAR(512),
    points INT NOT NULL DEFAULT 0,
    is_secret BOOLEAN NOT NULL DEFAULT false,
    is_active BOOLEAN NOT NULL DEFAULT true,
    criteria JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_achievements_type ON achievements(type);
CREATE INDEX idx_achievements_tier ON achievements(tier);
CREATE INDEX idx_achievements_active ON achievements(is_active) WHERE is_active = true;

-- User achievements table - tracks which users have earned which achievements
CREATE TABLE IF NOT EXISTS user_achievements (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    achievement_id VARCHAR(128) NOT NULL REFERENCES achievements(id) ON DELETE CASCADE,
    earned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    session_id VARCHAR(128), -- Optional: which lab session earned this
    progress DECIMAL(5,2) DEFAULT 100.00, -- Usually 100 when earned, but can track partial
    notified BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(user_id, achievement_id) -- Can only earn each achievement once
);

CREATE INDEX idx_user_achievements_user ON user_achievements(user_id);
CREATE INDEX idx_user_achievements_achievement ON user_achievements(achievement_id);
CREATE INDEX idx_user_achievements_earned ON user_achievements(earned_at DESC);
CREATE INDEX idx_user_achievements_notified ON user_achievements(notified) WHERE notified = false;

-- Achievement progress table - tracks user progress towards achievements
CREATE TABLE IF NOT EXISTS achievement_progress (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    achievement_id VARCHAR(128) NOT NULL REFERENCES achievements(id) ON DELETE CASCADE,
    progress DECIMAL(5,2) NOT NULL DEFAULT 0.00, -- 0-100
    current_value INT NOT NULL DEFAULT 0,
    required_value INT NOT NULL DEFAULT 1,
    metadata JSONB DEFAULT '{}'::jsonb, -- Additional tracking data
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY(user_id, achievement_id)
);

CREATE INDEX idx_achievement_progress_updated ON achievement_progress(updated_at DESC);

-- Seed some initial achievements
INSERT INTO achievements (id, name, description, type, tier, points, criteria) VALUES
    -- Lab completion achievements
    ('first-lab', 'First Steps', 'Complete your first lab', 'lab_completion', 'bronze', 10,
     '{"totalLabs": 1}'::jsonb),

    ('lab-veteran', 'Lab Veteran', 'Complete 10 labs', 'milestone', 'silver', 50,
     '{"totalLabs": 10}'::jsonb),

    ('lab-master', 'Lab Master', 'Complete 25 labs', 'milestone', 'gold', 150,
     '{"totalLabs": 25}'::jsonb),

    ('lab-legend', 'Lab Legend', 'Complete 50 labs', 'milestone', 'platinum', 300,
     '{"totalLabs": 50}'::jsonb),

    -- Perfect score achievements
    ('perfectionist', 'Perfectionist', 'Earn a perfect score on any lab', 'perfect_score', 'silver', 25,
     '{"requirePerfect": true}'::jsonb),

    ('flawless-five', 'Flawless Five', 'Earn perfect scores on 5 different labs', 'perfect_score', 'gold', 100,
     '{"requirePerfect": true, "totalLabs": 5}'::jsonb),

    -- Speed achievements
    ('speed-demon', 'Speed Demon', 'Complete a lab in record time', 'speed', 'gold', 75,
     '{"maxDurationMins": 15}'::jsonb),

    -- Streak achievements
    ('on-fire', 'On Fire!', 'Complete 3 labs in a row', 'streak', 'bronze', 30,
     '{"streakCount": 3, "streakType": "any"}'::jsonb),

    ('unstoppable', 'Unstoppable', 'Complete 7 labs in a row', 'streak', 'silver', 75,
     '{"streakCount": 7, "streakType": "any"}'::jsonb),

    -- Category achievements
    ('security-expert', 'Security Expert', 'Complete all security labs', 'category', 'gold', 100,
     '{"labCategory": "security"}'::jsonb),

    ('network-ninja', 'Network Ninja', 'Complete all networking labs', 'category', 'gold', 100,
     '{"labCategory": "networking"}'::jsonb),

    -- Difficulty achievements
    ('brave-beginner', 'Brave Beginner', 'Complete a beginner lab', 'lab_completion', 'bronze', 5,
     '{"minDifficulty": "beginner", "totalLabs": 1}'::jsonb),

    ('advanced-achiever', 'Advanced Achiever', 'Complete an advanced lab', 'lab_completion', 'silver', 40,
     '{"minDifficulty": "advanced", "totalLabs": 1}'::jsonb),

    ('expert-elite', 'Expert Elite', 'Complete an expert-level lab', 'lab_completion', 'gold', 100,
     '{"minDifficulty": "expert", "totalLabs": 1}'::jsonb),

    -- Special achievements
    ('early-bird', 'Early Bird', 'Complete a lab before 6 AM', 'special', 'bronze', 20,
     '{"customRule": "early_morning"}'::jsonb),

    ('night-owl', 'Night Owl', 'Complete a lab after 10 PM', 'special', 'bronze', 20,
     '{"customRule": "late_night"}'::jsonb)
ON CONFLICT (id) DO NOTHING;

-- Add a trigger to update the updated_at timestamp
CREATE OR REPLACE FUNCTION update_achievement_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER achievements_updated_at
    BEFORE UPDATE ON achievements
    FOR EACH ROW
    EXECUTE FUNCTION update_achievement_timestamp();

CREATE TRIGGER achievement_progress_updated_at
    BEFORE UPDATE ON achievement_progress
    FOR EACH ROW
    EXECUTE FUNCTION update_achievement_timestamp();
