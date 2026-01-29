DROP INDEX IF EXISTS idx_content_types_slug;
CREATE UNIQUE INDEX idx_content_types_slug ON content_types(slug);

ALTER TABLE content_types DROP COLUMN IF EXISTS ui_schema;
ALTER TABLE content_types DROP COLUMN IF EXISTS schema_version;
ALTER TABLE content_types DROP COLUMN IF EXISTS status;
ALTER TABLE content_types DROP COLUMN IF EXISTS deleted_at;
