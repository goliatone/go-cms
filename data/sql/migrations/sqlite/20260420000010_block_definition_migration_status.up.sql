-- Block definitions: add migration status tracking
ALTER TABLE block_definitions ADD COLUMN migration_status TEXT;

-- Backfill migration_status from schema metadata
UPDATE block_definitions
SET migration_status = COALESCE(
    json_extract(schema, '$.metadata.migration_status'),
    json_extract(schema, '$.x-cms.migration_status'),
    json_extract(schema, '$.x-admin.migration_status'),
    CASE
        WHEN schema_version IS NULL OR schema_version = '' THEN 'unversioned'
        WHEN json_extract(schema, '$.metadata.schema_version') IS NOT NULL
             AND json_extract(schema, '$.metadata.schema_version') <> schema_version THEN 'mismatch'
        ELSE 'current'
    END
)
WHERE migration_status IS NULL OR migration_status = '';
