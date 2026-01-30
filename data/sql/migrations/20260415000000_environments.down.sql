-- Drop env consistency triggers.
DROP TRIGGER IF EXISTS trg_contents_env_check ON contents;
DROP FUNCTION IF EXISTS ensure_contents_env_match();

DROP TRIGGER IF EXISTS trg_pages_env_check ON pages;
DROP FUNCTION IF EXISTS ensure_pages_env_match();

-- Drop env-scoped indexes.
DROP INDEX IF EXISTS idx_content_types_env_slug;
DROP INDEX IF EXISTS idx_contents_env_type_slug;
DROP INDEX IF EXISTS idx_pages_env_slug;
DROP INDEX IF EXISTS idx_menus_env_code;
DROP INDEX IF EXISTS idx_block_definitions_env_slug;

-- Restore global slug uniqueness.
CREATE UNIQUE INDEX IF NOT EXISTS idx_content_types_slug ON content_types(slug) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_contents_slug_unique ON contents(slug);
ALTER TABLE menus ADD CONSTRAINT menus_code_key UNIQUE (code);
CREATE INDEX IF NOT EXISTS idx_menus_code ON menus(code);

-- Drop environment_id columns and block definition slug.
ALTER TABLE block_definitions DROP COLUMN IF EXISTS slug;
ALTER TABLE block_definitions DROP COLUMN IF EXISTS environment_id;
ALTER TABLE menu_items DROP COLUMN IF EXISTS environment_id;
ALTER TABLE menus DROP COLUMN IF EXISTS environment_id;
ALTER TABLE pages DROP COLUMN IF EXISTS environment_id;
ALTER TABLE contents DROP COLUMN IF EXISTS environment_id;
ALTER TABLE content_types DROP COLUMN IF EXISTS environment_id;

-- Drop environments table.
DROP INDEX IF EXISTS idx_environments_default;
DROP TABLE IF EXISTS environments;
