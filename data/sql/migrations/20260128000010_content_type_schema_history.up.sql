ALTER TABLE content_types ADD COLUMN schema_history JSONB;

UPDATE content_types
SET schema_history = '[]'::jsonb
WHERE schema_history IS NULL;
