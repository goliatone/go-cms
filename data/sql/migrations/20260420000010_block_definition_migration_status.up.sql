-- Block definitions: add migration status tracking
ALTER TABLE block_definitions ADD COLUMN IF NOT EXISTS migration_status TEXT;

-- Backfill migration_status from schema metadata
UPDATE block_definitions
SET migration_status = COALESCE(
    schema->'metadata'->>'migration_status',
    schema->'x-cms'->>'migration_status',
    schema->'x-admin'->>'migration_status',
    CASE
        WHEN schema_version IS NULL OR schema_version = '' THEN 'unversioned'
        WHEN (schema->'metadata'->>'schema_version') IS NOT NULL
             AND (schema->'metadata'->>'schema_version') <> schema_version THEN 'mismatch'
        ELSE 'current'
    END
)
WHERE migration_status IS NULL OR migration_status = '';
