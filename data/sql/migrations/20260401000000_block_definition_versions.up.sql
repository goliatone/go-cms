-- Add schema version tracking for block definitions
ALTER TABLE block_definitions ADD COLUMN schema_version TEXT;

-- Block definition versions
CREATE TABLE block_definition_versions (
    id UUID PRIMARY KEY,
    definition_id UUID NOT NULL REFERENCES block_definitions(id) ON DELETE CASCADE,
    schema_version TEXT NOT NULL,
    schema JSONB NOT NULL,
    defaults JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(definition_id, schema_version)
);

CREATE INDEX idx_block_definition_versions_definition_id ON block_definition_versions(definition_id);
CREATE INDEX idx_block_definition_versions_version ON block_definition_versions(definition_id, schema_version);

-- Backfill schema_version for existing definitions
UPDATE block_definitions
SET schema_version = COALESCE(schema->'metadata'->>'schema_version', name || '@v1.0.0')
WHERE schema_version IS NULL;

-- Seed version records for existing definitions
INSERT INTO block_definition_versions (id, definition_id, schema_version, schema, defaults, created_at, updated_at)
SELECT id, id, COALESCE(schema->'metadata'->>'schema_version', name || '@v1.0.0'), schema, defaults, created_at, updated_at
FROM block_definitions
ON CONFLICT (definition_id, schema_version) DO NOTHING;
