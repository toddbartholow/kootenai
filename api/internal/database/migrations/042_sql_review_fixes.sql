-- SQL review fixes: race condition, redundant indexes, missing indexes, trigger optimization, FK

-- 1. Fix race condition in get_next_version_number() with advisory lock
CREATE OR REPLACE FUNCTION get_next_version_number(p_template_id UUID)
RETURNS INTEGER AS $$
DECLARE
    next_num INTEGER;
BEGIN
    -- Serialize version number assignment per template to prevent race conditions
    PERFORM pg_advisory_xact_lock(hashtext(p_template_id::text));
    SELECT COALESCE(MAX(version_number), 0) + 1
    INTO next_num
    FROM lab_template_versions
    WHERE lab_template_id = p_template_id;
    RETURN next_num;
END;
$$ LANGUAGE plpgsql;

-- 2. Drop redundant index (covered by UNIQUE constraint on (lab_template_id, version_number))
DROP INDEX IF EXISTS idx_lab_template_versions_template_id;

-- 3. Drop redundant created_at index (not used by current queries)
DROP INDEX IF EXISTS idx_lab_template_versions_created_at;

-- 4. Add missing index on lab_templates.slug
CREATE INDEX IF NOT EXISTS idx_lab_templates_slug ON lab_templates(slug);

-- 5. Drop redundant name index (covered by UNIQUE constraint on name)
DROP INDEX IF EXISTS idx_lab_templates_name;

-- 6. Add foreign key on created_by_user_id
-- Use ON DELETE SET NULL so versions survive user deletion
ALTER TABLE lab_template_versions
    ADD CONSTRAINT fk_lab_template_versions_created_by_user
    FOREIGN KEY (created_by_user_id) REFERENCES users(id) ON DELETE SET NULL;

-- 7. Optimize update_session_progress() trigger to use single subquery
CREATE OR REPLACE FUNCTION update_session_progress()
RETURNS TRIGGER AS $$
DECLARE
    v_total_earned INTEGER;
    v_max_points INTEGER;
    v_pass_threshold INTEGER;
    v_percentage NUMERIC;
    v_passed BOOLEAN;
BEGIN
    -- Single scan of checkpoint_progress
    SELECT COALESCE(SUM(earned_points), 0)
    INTO v_total_earned
    FROM checkpoint_progress
    WHERE session_id = NEW.session_id;

    -- Get max_points and pass_threshold from session + template
    SELECT ls.max_points, lt.pass_threshold
    INTO v_max_points, v_pass_threshold
    FROM lab_sessions ls
    JOIN lab_templates lt ON lt.id = ls.lab_template_id
    WHERE ls.id = NEW.session_id;

    -- Calculate derived values
    IF v_max_points > 0 THEN
        v_percentage := ROUND((v_total_earned::DECIMAL / v_max_points) * 100, 2);
        v_passed := v_percentage >= v_pass_threshold;
    ELSE
        v_percentage := 0;
        v_passed := false;
    END IF;

    UPDATE lab_sessions
    SET earned_points = v_total_earned,
        percentage = v_percentage,
        passed = v_passed
    WHERE id = NEW.session_id;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
