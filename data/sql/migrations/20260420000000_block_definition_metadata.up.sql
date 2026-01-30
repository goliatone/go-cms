-- Block definitions: add metadata fields for the Block Library IDE
ALTER TABLE block_definitions ADD COLUMN IF NOT EXISTS category TEXT;
ALTER TABLE block_definitions ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'draft';
ALTER TABLE block_definitions ADD COLUMN IF NOT EXISTS ui_schema JSONB;

-- Backfill status for existing definitions
UPDATE block_definitions
SET status = 'draft'
WHERE status IS NULL OR status = '';
