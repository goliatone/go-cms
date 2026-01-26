DROP INDEX IF EXISTS idx_content_types_slug;
ALTER TABLE content_types DROP COLUMN slug;
