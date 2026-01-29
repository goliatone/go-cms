ALTER TABLE content_types ADD COLUMN ui_schema JSONB;
ALTER TABLE content_types ADD COLUMN schema_version TEXT;
ALTER TABLE content_types ADD COLUMN status TEXT;
ALTER TABLE content_types ADD COLUMN deleted_at TIMESTAMP;

UPDATE content_types
SET status = 'draft'
WHERE status IS NULL OR status = '';

UPDATE content_types
SET schema_version = COALESCE(
    NULLIF(schema->'metadata'->>'schema_version', ''),
    CASE WHEN slug IS NOT NULL AND slug <> '' THEN slug || '@v1.0.0' ELSE NULL END
)
WHERE schema_version IS NULL OR schema_version = '';

DROP INDEX IF EXISTS idx_content_types_slug;
CREATE UNIQUE INDEX idx_content_types_slug ON content_types(slug) WHERE deleted_at IS NULL;
