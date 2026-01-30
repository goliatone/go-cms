-- Environments: core scoping for content, pages, menus, and blocks
CREATE TABLE IF NOT EXISTS environments (
    id UUID PRIMARY KEY,
    key TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    description TEXT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_environments_default ON environments(is_default) WHERE is_default;

-- Seed a default environment for backfill and opt-in behavior.
INSERT INTO environments (id, key, name, description, is_active, is_default, created_at, updated_at)
VALUES ('00000000-0000-0000-0000-000000000001', 'default', 'Default', 'Default environment', TRUE, TRUE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;

-- Add environment_id columns.
ALTER TABLE content_types ADD COLUMN IF NOT EXISTS environment_id UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001';
ALTER TABLE contents ADD COLUMN IF NOT EXISTS environment_id UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001';
ALTER TABLE pages ADD COLUMN IF NOT EXISTS environment_id UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001';
ALTER TABLE menus ADD COLUMN IF NOT EXISTS environment_id UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001';
ALTER TABLE menu_items ADD COLUMN IF NOT EXISTS environment_id UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001';
ALTER TABLE block_definitions ADD COLUMN IF NOT EXISTS environment_id UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001';

-- Block definitions: add canonical slug identifier.
ALTER TABLE block_definitions ADD COLUMN IF NOT EXISTS slug TEXT;

-- Backfill environment_id to the default environment.
WITH default_env AS (
    SELECT id FROM environments WHERE key = 'default' LIMIT 1
)
UPDATE content_types
SET environment_id = (SELECT id FROM default_env)
WHERE environment_id IS NULL OR environment_id = '00000000-0000-0000-0000-000000000001';

UPDATE contents AS c
SET environment_id = ct.environment_id
FROM content_types ct
WHERE c.content_type_id = ct.id;

UPDATE contents
SET environment_id = (SELECT id FROM environments WHERE key = 'default' LIMIT 1)
WHERE environment_id IS NULL OR environment_id = '00000000-0000-0000-0000-000000000001';

UPDATE pages AS p
SET environment_id = c.environment_id
FROM contents c
WHERE p.content_id = c.id;

UPDATE pages
SET environment_id = (SELECT id FROM environments WHERE key = 'default' LIMIT 1)
WHERE environment_id IS NULL OR environment_id = '00000000-0000-0000-0000-000000000001';

UPDATE menus
SET environment_id = (SELECT id FROM environments WHERE key = 'default' LIMIT 1)
WHERE environment_id IS NULL OR environment_id = '00000000-0000-0000-0000-000000000001';

UPDATE menu_items AS mi
SET environment_id = m.environment_id
FROM menus m
WHERE mi.menu_id = m.id;

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
SET slug = substr(CAST(id AS TEXT), 1, 8)
WHERE slug IS NULL OR slug = '';

WITH duplicates AS (
    SELECT environment_id, slug
    FROM block_definitions
    GROUP BY environment_id, slug
    HAVING COUNT(*) > 1
)
UPDATE block_definitions
SET slug = slug || '-' || substr(CAST(id AS TEXT), 1, 8)
WHERE (environment_id, slug) IN (SELECT environment_id, slug FROM duplicates);

-- Replace global slug indexes with env-scoped uniqueness.
ALTER TABLE menus DROP CONSTRAINT IF EXISTS menus_code_key;
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
CREATE OR REPLACE FUNCTION ensure_contents_env_match() RETURNS trigger AS $$
BEGIN
    IF NEW.content_type_id IS NULL THEN
        RETURN NEW;
    END IF;
    IF EXISTS (
        SELECT 1 FROM content_types ct
        WHERE ct.id = NEW.content_type_id
          AND ct.environment_id = NEW.environment_id
    ) THEN
        RETURN NEW;
    END IF;
    RAISE EXCEPTION 'contents.environment_id does not match content_types.environment_id';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_contents_env_check ON contents;
CREATE TRIGGER trg_contents_env_check
BEFORE INSERT OR UPDATE OF content_type_id, environment_id ON contents
FOR EACH ROW EXECUTE FUNCTION ensure_contents_env_match();

-- Enforce env consistency between pages and contents.
CREATE OR REPLACE FUNCTION ensure_pages_env_match() RETURNS trigger AS $$
BEGIN
    IF NEW.content_id IS NULL THEN
        RETURN NEW;
    END IF;
    IF EXISTS (
        SELECT 1 FROM contents c
        WHERE c.id = NEW.content_id
          AND c.environment_id = NEW.environment_id
    ) THEN
        RETURN NEW;
    END IF;
    RAISE EXCEPTION 'pages.environment_id does not match contents.environment_id';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_pages_env_check ON pages;
CREATE TRIGGER trg_pages_env_check
BEFORE INSERT OR UPDATE OF content_id, environment_id ON pages
FOR EACH ROW EXECUTE FUNCTION ensure_pages_env_match();
