-- Add slug to content types with deterministic backfill.
ALTER TABLE content_types ADD COLUMN slug TEXT;

UPDATE content_types
SET slug = lower(replace(trim(name), ' ', '-'))
WHERE slug IS NULL OR slug = '';

UPDATE content_types
SET slug = substr(CAST(id AS TEXT), 1, 8)
WHERE slug IS NULL OR slug = '';

WITH duplicates AS (
    SELECT slug
    FROM content_types
    GROUP BY slug
    HAVING COUNT(*) > 1
)
UPDATE content_types
SET slug = slug || '-' || substr(CAST(id AS TEXT), 1, 8)
WHERE slug IN (SELECT slug FROM duplicates);

CREATE UNIQUE INDEX idx_content_types_slug ON content_types(slug);
