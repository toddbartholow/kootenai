-- Vim Mastery Pathway Seed Data
-- This migration creates the Vim pathway with 8 modules and their associated labs

-- =============================================================================
-- VIM LAB TEMPLATES
-- =============================================================================

INSERT INTO lab_templates (id, name, slug, description, difficulty, platform, duration_minutes, max_points, tags, visibility, is_active, spec, created_at, updated_at)
VALUES
    -- Module 1: Vim Fundamentals
    ('a2000001-0001-0001-0001-000000000001', 'Vim Fundamentals', 'vim-fundamentals',
     'Begin your Vim journey by understanding the modal editing philosophy. Master Normal, Insert, and Command modes, learn basic navigation with hjkl, and discover how to save and quit safely.',
     'beginner', 'proxmox', 30, 100,
     ARRAY['vim', 'editor', 'linux', 'text-editing', 'beginner'],
     'global', true, '{}'::jsonb, NOW(), NOW()),

    -- Module 2: Essential Editing
    ('a2000001-0001-0001-0001-000000000002', 'Essential Editing', 'vim-essential-editing',
     'Learn the core editing commands that make Vim so powerful. Master deletion with d, yanking with y, putting with p, and the change command c. Understand undo/redo and the dot command for repeating actions.',
     'beginner', 'proxmox', 40, 100,
     ARRAY['vim', 'editor', 'linux', 'text-editing', 'beginner', 'editing'],
     'global', true, '{}'::jsonb, NOW(), NOW()),

    -- Module 3: Navigation Mastery
    ('a2000001-0001-0001-0001-000000000003', 'Navigation Mastery', 'vim-navigation-mastery',
     'Level up your Vim navigation skills with advanced movement commands. Learn to jump across screens, search within files, use marks to bookmark locations, and navigate by line numbers.',
     'intermediate', 'proxmox', 35, 120,
     ARRAY['vim', 'editor', 'linux', 'text-editing', 'intermediate', 'navigation'],
     'global', true, '{}'::jsonb, NOW(), NOW()),

    -- Module 4: Visual Mode & Selection
    ('a2000001-0001-0001-0001-000000000004', 'Visual Mode & Selection', 'vim-visual-mode',
     'Master Vim''s Visual modes for selecting and operating on text. Learn character-wise, line-wise, and block-wise selection. Visual Block mode is especially powerful for editing columns of text.',
     'intermediate', 'proxmox', 35, 120,
     ARRAY['vim', 'editor', 'linux', 'text-editing', 'intermediate', 'visual-mode'],
     'global', true, '{}'::jsonb, NOW(), NOW()),

    -- Module 5: Search and Replace
    ('a2000001-0001-0001-0001-000000000005', 'Search and Replace', 'vim-search-replace',
     'Learn Vim''s powerful search and replace capabilities. Master the substitute command, work with regular expressions, and perform global operations across files.',
     'intermediate', 'proxmox', 40, 130,
     ARRAY['vim', 'editor', 'linux', 'text-editing', 'intermediate', 'regex', 'search'],
     'global', true, '{}'::jsonb, NOW(), NOW()),

    -- Module 6: Buffers, Windows & Tabs
    ('a2000001-0001-0001-0001-000000000006', 'Buffers, Windows & Tabs', 'vim-buffers-windows',
     'Learn to work with multiple files efficiently in Vim. Master buffers for managing open files, windows for viewing multiple files simultaneously, and tabs for organizing your workspace.',
     'intermediate', 'proxmox', 35, 120,
     ARRAY['vim', 'editor', 'linux', 'text-editing', 'intermediate', 'workflow'],
     'global', true, '{}'::jsonb, NOW(), NOW()),

    -- Module 7: Configuration & Customization
    ('a2000001-0001-0001-0001-000000000007', 'Configuration & Customization', 'vim-configuration',
     'Make Vim your own by learning to configure it. Create and customize your .vimrc file, set essential options, create custom key mappings, and personalize the appearance with color schemes.',
     'advanced', 'proxmox', 45, 150,
     ARRAY['vim', 'editor', 'linux', 'text-editing', 'advanced', 'configuration'],
     'global', true, '{}'::jsonb, NOW(), NOW()),

    -- Module 8: Advanced Features
    ('a2000001-0001-0001-0001-000000000008', 'Advanced Vim Features', 'vim-advanced-features',
     'Master Vim''s most powerful features: macros for automating repetitive edits, registers for flexible text storage, advanced text objects for precise selection, and folding for managing large files.',
     'advanced', 'proxmox', 50, 160,
     ARRAY['vim', 'editor', 'linux', 'text-editing', 'advanced', 'macros', 'productivity'],
     'global', true, '{}'::jsonb, NOW(), NOW())
ON CONFLICT (slug) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    difficulty = EXCLUDED.difficulty,
    duration_minutes = EXCLUDED.duration_minutes,
    max_points = EXCLUDED.max_points,
    tags = EXCLUDED.tags,
    updated_at = NOW();

-- =============================================================================
-- VIM MASTERY PATHWAY
-- =============================================================================

INSERT INTO pathways (id, name, slug, description, short_description, difficulty, estimated_hours, display_order, status, is_active, is_featured, visibility, prerequisites, tags, icon, color, created_at, updated_at)
VALUES (
    'b2000002-0001-0001-0001-000000000001',
    'Vim Mastery',
    'vim-mastery',
    'Transform your text editing workflow with Vim, the legendary modal editor. This pathway takes you from complete beginner to Vim power user. Learn the philosophy behind modal editing, master efficient navigation and text manipulation, configure Vim to your preferences, and unlock advanced features like macros and registers. By the end, you''ll understand why Vim users are so passionate about their editor.',
    'Master the art of modal editing - from basic navigation to macros and custom configuration.',
    'beginner',
    5, -- Total estimated hours (30+40+35+35+40+35+45+50 = 310 minutes = ~5 hours)
    2, -- Display after Linux pathway
    'published',
    true,
    true,
    'global',
    '["Basic familiarity with terminal/command line", "A text file you want to edit"]'::jsonb,
    ARRAY['vim', 'editor', 'productivity', 'linux', 'development'],
    'pi pi-pencil',
    '#019733',
    NOW(),
    NOW()
)
ON CONFLICT (slug) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    short_description = EXCLUDED.short_description,
    estimated_hours = EXCLUDED.estimated_hours,
    is_featured = EXCLUDED.is_featured,
    status = 'published',
    icon = EXCLUDED.icon,
    color = EXCLUDED.color,
    updated_at = NOW();

-- =============================================================================
-- VIM PATHWAY MODULES
-- =============================================================================

INSERT INTO pathway_modules (id, pathway_id, name, slug, description, display_order, unlock_type, is_active, icon, estimated_minutes, created_at)
VALUES
    -- Module 1: Vim Fundamentals (always unlocked - entry point)
    ('c3000002-0001-0001-0001-000000000001', 'b2000002-0001-0001-0001-000000000001',
     'Vim Fundamentals', 'vim-fundamentals',
     'Understand modal editing and learn the basic commands to navigate and edit text.',
     1, 'always', true, 'pi pi-play', 30, NOW()),

    -- Module 2: Essential Editing (sequential)
    ('c3000002-0001-0001-0001-000000000002', 'b2000002-0001-0001-0001-000000000001',
     'Essential Editing', 'essential-editing',
     'Master the core editing commands: delete, yank, put, change, and the power of the dot command.',
     2, 'sequential', true, 'pi pi-file-edit', 40, NOW()),

    -- Module 3: Navigation Mastery (sequential)
    ('c3000002-0001-0001-0001-000000000003', 'b2000002-0001-0001-0001-000000000001',
     'Navigation Mastery', 'navigation-mastery',
     'Move through files at lightning speed with advanced navigation commands.',
     3, 'sequential', true, 'pi pi-directions', 35, NOW()),

    -- Module 4: Visual Mode (sequential)
    ('c3000002-0001-0001-0001-000000000004', 'b2000002-0001-0001-0001-000000000001',
     'Visual Mode & Selection', 'visual-mode',
     'Select and operate on text with character, line, and block visual modes.',
     4, 'sequential', true, 'pi pi-check-square', 35, NOW()),

    -- Module 5: Search and Replace (sequential)
    ('c3000002-0001-0001-0001-000000000005', 'b2000002-0001-0001-0001-000000000001',
     'Search and Replace', 'search-replace',
     'Find and transform text with powerful search patterns and substitution commands.',
     5, 'sequential', true, 'pi pi-search', 40, NOW()),

    -- Module 6: Buffers, Windows & Tabs (sequential)
    ('c3000002-0001-0001-0001-000000000006', 'b2000002-0001-0001-0001-000000000001',
     'Buffers, Windows & Tabs', 'buffers-windows-tabs',
     'Work with multiple files efficiently using buffers, splits, and tabs.',
     6, 'sequential', true, 'pi pi-clone', 35, NOW()),

    -- Module 7: Configuration (sequential)
    ('c3000002-0001-0001-0001-000000000007', 'b2000002-0001-0001-0001-000000000001',
     'Configuration & Customization', 'configuration',
     'Make Vim your own with .vimrc settings, mappings, and color schemes.',
     7, 'sequential', true, 'pi pi-cog', 45, NOW()),

    -- Module 8: Advanced Features (sequential)
    ('c3000002-0001-0001-0001-000000000008', 'b2000002-0001-0001-0001-000000000001',
     'Advanced Features', 'advanced-features',
     'Unlock Vim''s full power with macros, registers, text objects, and folding.',
     8, 'sequential', true, 'pi pi-bolt', 50, NOW())
ON CONFLICT (pathway_id, slug) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    display_order = EXCLUDED.display_order,
    icon = EXCLUDED.icon,
    estimated_minutes = EXCLUDED.estimated_minutes;

-- =============================================================================
-- LINK LABS TO MODULES
-- =============================================================================

INSERT INTO module_labs (id, module_id, lab_template_id, display_order, is_required)
VALUES
    ('d4000002-0001-0001-0001-000000000001', 'c3000002-0001-0001-0001-000000000001', 'a2000001-0001-0001-0001-000000000001', 1, true),
    ('d4000002-0001-0001-0001-000000000002', 'c3000002-0001-0001-0001-000000000002', 'a2000001-0001-0001-0001-000000000002', 1, true),
    ('d4000002-0001-0001-0001-000000000003', 'c3000002-0001-0001-0001-000000000003', 'a2000001-0001-0001-0001-000000000003', 1, true),
    ('d4000002-0001-0001-0001-000000000004', 'c3000002-0001-0001-0001-000000000004', 'a2000001-0001-0001-0001-000000000004', 1, true),
    ('d4000002-0001-0001-0001-000000000005', 'c3000002-0001-0001-0001-000000000005', 'a2000001-0001-0001-0001-000000000005', 1, true),
    ('d4000002-0001-0001-0001-000000000006', 'c3000002-0001-0001-0001-000000000006', 'a2000001-0001-0001-0001-000000000006', 1, true),
    ('d4000002-0001-0001-0001-000000000007', 'c3000002-0001-0001-0001-000000000007', 'a2000001-0001-0001-0001-000000000007', 1, true),
    ('d4000002-0001-0001-0001-000000000008', 'c3000002-0001-0001-0001-000000000008', 'a2000001-0001-0001-0001-000000000008', 1, true)
ON CONFLICT (id) DO UPDATE SET
    display_order = EXCLUDED.display_order,
    is_required = EXCLUDED.is_required;

-- =============================================================================
-- VIM ACHIEVEMENTS
-- =============================================================================

INSERT INTO achievements (id, name, description, type, tier, icon_url, points, is_secret, is_active, criteria, created_at, updated_at)
VALUES
    -- Progression achievements
    ('vim-novice', 'Vim Novice',
     'Complete the first Vim module and understand modal editing',
     'pathway', 'bronze', '/icons/vim.svg', 50, false, true,
     '{"type": "pathway_modules", "pathway_slug": "vim-mastery", "modules_required": 1}'::jsonb,
     NOW(), NOW()),

    ('vim-apprentice', 'Vim Apprentice',
     'Complete the first four Vim modules (Fundamentals through Visual Mode)',
     'pathway', 'silver', '/icons/vim.svg', 150, false, true,
     '{"type": "pathway_modules", "pathway_slug": "vim-mastery", "modules_required": 4}'::jsonb,
     NOW(), NOW()),

    ('vim-journeyman', 'Vim Journeyman',
     'Complete six Vim modules including Search/Replace and Multi-file editing',
     'pathway', 'gold', '/icons/vim.svg', 250, false, true,
     '{"type": "pathway_modules", "pathway_slug": "vim-mastery", "modules_required": 6}'::jsonb,
     NOW(), NOW()),

    ('vim-master', 'Vim Master',
     'Complete the entire Vim Mastery pathway',
     'pathway', 'platinum', '/icons/trophy.svg', 500, false, true,
     '{"type": "pathway_complete", "pathway_slug": "vim-mastery"}'::jsonb,
     NOW(), NOW()),

    -- Special achievements
    ('vim-perfectionist', 'Vim Perfectionist',
     'Score 100% on any Vim pathway lab',
     'perfect_score', 'gold', '/icons/star.svg', 100, false, true,
     '{"type": "lab_perfect_score", "pathway_slug": "vim-mastery"}'::jsonb,
     NOW(), NOW()),

    ('modal-convert', 'Modal Convert',
     'Complete all 8 Vim labs without using arrow keys (detected via objectives)',
     'special', 'platinum', '/icons/keyboard.svg', 300, true, true,
     '{"type": "custom", "check": "no_arrow_keys", "pathway_slug": "vim-mastery"}'::jsonb,
     NOW(), NOW()),

    ('macro-maestro', 'Macro Maestro',
     'Successfully record and replay a macro 10+ times in a single session',
     'special', 'gold', '/icons/repeat.svg', 150, false, true,
     '{"type": "custom", "check": "macro_replay_count", "min_count": 10}'::jsonb,
     NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    description = EXCLUDED.description,
    points = EXCLUDED.points,
    criteria = EXCLUDED.criteria,
    updated_at = NOW();

-- =============================================================================
-- SUMMARY
-- =============================================================================
-- Vim Mastery Pathway created with:
-- - 8 lab templates covering beginner to advanced topics
-- - 8 sequential modules with the first always unlocked
-- - 7 achievements (4 progression, 3 special)
-- - Total points available: 1000 (100+100+120+120+130+120+150+160)
-- - Estimated time: ~5 hours
