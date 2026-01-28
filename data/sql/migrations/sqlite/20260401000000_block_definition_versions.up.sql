-- Add schema version tracking for block definitions
ALTER TABLE block_definitions ADD COLUMN schema_version TEXT;

-- Block definition versions
CREATE TABLE block_definition_versions (
    id TEXT PRIMARY KEY,
    definition_id TEXT NOT NULL REFERENCES block_definitions(id) ON DELETE CASCADE,
    schema_version TEXT NOT NULL,
    schema TEXT NOT NULL,
    defaults TEXT,
    created_at TEXT,
    updated_at TEXT,
    UNIQUE(definition_id, schema_version)
);

CREATE INDEX idx_block_definition_versions_definition_id ON block_definition_versions(definition_id);
CREATE INDEX idx_block_definition_versions_version ON block_definition_versions(definition_id, schema_version);

-- Backfill schema_version for existing definitions
UPDATE block_definitions
SET schema_version = COALESCE(json_extract(schema, '$.metadata.schema_version'), name || '@v1.0.0')
WHERE schema_version IS NULL;

-- Seed version records for existing definitions
INSERT OR IGNORE INTO block_definition_versions (id, definition_id, schema_version, schema, defaults, created_at, updated_at)
SELECT id, id, COALESCE(json_extract(schema, '$.metadata.schema_version'), name || '@v1.0.0'), schema, defaults, created_at, updated_at
FROM block_definitions;
