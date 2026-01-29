ALTER TABLE content_types ADD COLUMN schema_history JSON;

UPDATE content_types
SET schema_history = json('[]')
WHERE schema_history IS NULL;
