-- Environments: core scoping for content, pages, menus, and blocks
CREATE TABLE IF NOT EXISTS environments (
    id TEXT PRIMARY KEY,
    key TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    description TEXT,
    is_active INTEGER NOT NULL DEFAULT 1,
    is_default INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_environments_default ON environments(is_default) WHERE is_default = 1;

-- Seed a default environment for backfill and opt-in behavior.
INSERT OR IGNORE INTO environments (id, key, name, description, is_active, is_default, created_at, updated_at)
VALUES ('00000000-0000-0000-0000-000000000001', 'default', 'Default', 'Default environment', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);

-- Add environment_id columns.
ALTER TABLE content_types ADD COLUMN environment_id TEXT NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001';
ALTER TABLE contents ADD COLUMN environment_id TEXT NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001';
ALTER TABLE pages ADD COLUMN environment_id TEXT NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001';
ALTER TABLE menu_items ADD COLUMN environment_id TEXT NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001';
ALTER TABLE block_definitions ADD COLUMN environment_id TEXT NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001';

-- Block definitions: add canonical slug identifier.
ALTER TABLE block_definitions ADD COLUMN slug TEXT;

-- Rebuild menus to drop the global unique constraint on code and add environment_id.
PRAGMA foreign_keys = OFF;
ALTER TABLE menus RENAME TO menus_old;
CREATE TABLE menus (
    id TEXT PRIMARY KEY,
    code TEXT NOT NULL,
    description TEXT,
    location TEXT,
    environment_id TEXT NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001',
    created_by TEXT NOT NULL,
    updated_by TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO menus (id, code, description, location, environment_id, created_by, updated_by, created_at, updated_at)
SELECT id, code, description, location, '00000000-0000-0000-0000-000000000001', created_by, updated_by, created_at, updated_at
FROM menus_old;
DROP TABLE menus_old;
PRAGMA foreign_keys = ON;

-- Backfill environment_id to the default environment.
UPDATE content_types
SET environment_id = (SELECT id FROM environments WHERE key = 'default' LIMIT 1)
WHERE environment_id IS NULL OR environment_id = '00000000-0000-0000-0000-000000000001';

UPDATE contents
SET environment_id = (SELECT environment_id FROM content_types WHERE content_types.id = contents.content_type_id)
WHERE content_type_id IS NOT NULL;

UPDATE contents
SET environment_id = (SELECT id FROM environments WHERE key = 'default' LIMIT 1)
WHERE environment_id IS NULL OR environment_id = '00000000-0000-0000-0000-000000000001';

UPDATE pages
SET environment_id = (SELECT environment_id FROM contents WHERE contents.id = pages.content_id)
WHERE content_id IS NOT NULL;

UPDATE pages
SET environment_id = (SELECT id FROM environments WHERE key = 'default' LIMIT 1)
WHERE environment_id IS NULL OR environment_id = '00000000-0000-0000-0000-000000000001';

UPDATE menus
SET environment_id = (SELECT id FROM environments WHERE key = 'default' LIMIT 1)
WHERE environment_id IS NULL OR environment_id = '00000000-0000-0000-0000-000000000001';

UPDATE menu_items
SET environment_id = (SELECT environment_id FROM menus WHERE menus.id = menu_items.menu_id)
WHERE menu_id IS NOT NULL;

UPDATE menu_items
SET environment_id = (SELECT id FROM environments WHERE key = 'default' LIMIT 1)
WHERE environment_id IS NULL OR environment_id = '00000000-0000-0000-0000-000000000001';

UPDATE block_definitions
SET environment_id = (SELECT id FROM environments WHERE key = 'default' LIMIT 1)
WHERE environment_id IS NULL OR environment_id = '00000000-0000-0000-0000-000000000001';

-- Backfill block definition slugs.
UPDATE block_definitions
SET slug = lower(replace(trim(name), ' ', '-'))
WHERE slug IS NULL OR slug = '';

UPDATE block_definitions
SET slug = substr(id, 1, 8)
WHERE slug IS NULL OR slug = '';

WITH duplicates AS (
    SELECT environment_id, slug
    FROM block_definitions
    GROUP BY environment_id, slug
    HAVING COUNT(*) > 1
)
UPDATE block_definitions
SET slug = slug || '-' || substr(id, 1, 8)
WHERE (environment_id, slug) IN (SELECT environment_id, slug FROM duplicates);

-- Replace global slug indexes with env-scoped uniqueness.
DROP INDEX IF EXISTS idx_menus_code;
DROP INDEX IF EXISTS idx_content_types_slug;
DROP INDEX IF EXISTS idx_contents_slug_unique;

CREATE UNIQUE INDEX IF NOT EXISTS idx_content_types_env_slug ON content_types(environment_id, slug) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_contents_env_type_slug ON contents(environment_id, content_type_id, slug);
CREATE UNIQUE INDEX IF NOT EXISTS idx_pages_env_slug ON pages(environment_id, slug) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_menus_env_code ON menus(environment_id, code);
CREATE INDEX IF NOT EXISTS idx_menus_code ON menus(code);
CREATE UNIQUE INDEX IF NOT EXISTS idx_block_definitions_env_slug ON block_definitions(environment_id, slug) WHERE deleted_at IS NULL;

-- Enforce env consistency between contents and content types.
CREATE TRIGGER IF NOT EXISTS trg_contents_env_check_insert
BEFORE INSERT ON contents
BEGIN
    SELECT CASE
        WHEN (SELECT environment_id FROM content_types WHERE id = NEW.content_type_id) IS NULL THEN
            RAISE(ABORT, 'content type not found')
        WHEN (SELECT environment_id FROM content_types WHERE id = NEW.content_type_id) != NEW.environment_id THEN
            RAISE(ABORT, 'contents.environment_id does not match content_types.environment_id')
    END;
END;

CREATE TRIGGER IF NOT EXISTS trg_contents_env_check_update
BEFORE UPDATE OF content_type_id, environment_id ON contents
BEGIN
    SELECT CASE
        WHEN (SELECT environment_id FROM content_types WHERE id = NEW.content_type_id) IS NULL THEN
            RAISE(ABORT, 'content type not found')
        WHEN (SELECT environment_id FROM content_types WHERE id = NEW.content_type_id) != NEW.environment_id THEN
            RAISE(ABORT, 'contents.environment_id does not match content_types.environment_id')
    END;
END;

-- Enforce env consistency between pages and contents.
CREATE TRIGGER IF NOT EXISTS trg_pages_env_check_insert
BEFORE INSERT ON pages
BEGIN
    SELECT CASE
        WHEN (SELECT environment_id FROM contents WHERE id = NEW.content_id) IS NULL THEN
            RAISE(ABORT, 'content not found')
        WHEN (SELECT environment_id FROM contents WHERE id = NEW.content_id) != NEW.environment_id THEN
            RAISE(ABORT, 'pages.environment_id does not match contents.environment_id')
    END;
END;

CREATE TRIGGER IF NOT EXISTS trg_pages_env_check_update
BEFORE UPDATE OF content_id, environment_id ON pages
BEGIN
    SELECT CASE
        WHEN (SELECT environment_id FROM contents WHERE id = NEW.content_id) IS NULL THEN
            RAISE(ABORT, 'content not found')
        WHEN (SELECT environment_id FROM contents WHERE id = NEW.content_id) != NEW.environment_id THEN
            RAISE(ABORT, 'pages.environment_id does not match contents.environment_id')
    END;
END;
