-- Drop env consistency triggers.
DROP TRIGGER IF EXISTS trg_contents_env_check_insert;
DROP TRIGGER IF EXISTS trg_contents_env_check_update;
DROP TRIGGER IF EXISTS trg_pages_env_check_insert;
DROP TRIGGER IF EXISTS trg_pages_env_check_update;

-- Drop env-scoped indexes.
DROP INDEX IF EXISTS idx_content_types_env_slug;
DROP INDEX IF EXISTS idx_contents_env_type_slug;
DROP INDEX IF EXISTS idx_pages_env_slug;
DROP INDEX IF EXISTS idx_menus_env_code;
DROP INDEX IF EXISTS idx_block_definitions_env_slug;

-- Restore global slug uniqueness indexes.
CREATE UNIQUE INDEX IF NOT EXISTS idx_content_types_slug ON content_types(slug) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_contents_slug_unique ON contents(slug);
CREATE INDEX IF NOT EXISTS idx_menus_code ON menus(code);

-- Drop environments table.
DROP INDEX IF EXISTS idx_environments_default;
DROP TABLE IF EXISTS environments;

-- SQLite does not support dropping columns via ALTER TABLE.
-- No-op for environment_id/slug columns and menu unique constraint changes.
