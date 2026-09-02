-- Migration 006: Optimize update_session_progress trigger
-- Reduces from 3 separate subqueries to a single CTE-based query

CREATE OR REPLACE FUNCTION update_session_progress()
RETURNS TRIGGER AS $$
BEGIN
    WITH checkpoint_stats AS (
        SELECT COALESCE(SUM(earned_points), 0) AS total_earned
        FROM checkpoint_progress
        WHERE session_id = NEW.session_id
    )
    UPDATE lab_sessions ls
    SET
        earned_points = cs.total_earned,
        percentage = CASE
            WHEN ls.max_points > 0 THEN
                ROUND((cs.total_earned::DECIMAL / ls.max_points) * 100, 2)
            ELSE 0
        END,
        passed = CASE
            WHEN ls.max_points > 0 THEN
                (cs.total_earned::DECIMAL / ls.max_points) * 100 >=
                (SELECT pass_threshold FROM lab_templates WHERE id = ls.lab_template_id)
            ELSE false
        END
    FROM checkpoint_stats cs
    WHERE ls.id = NEW.session_id;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION update_session_progress() IS
'Optimized trigger function that updates lab_sessions progress using a single CTE query instead of 3 separate subqueries';
