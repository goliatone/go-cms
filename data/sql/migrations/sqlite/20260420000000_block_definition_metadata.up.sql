-- Block definitions: add metadata fields for the Block Library IDE
ALTER TABLE block_definitions ADD COLUMN category TEXT;
ALTER TABLE block_definitions ADD COLUMN status TEXT;
ALTER TABLE block_definitions ADD COLUMN ui_schema JSON;

-- Backfill status for existing definitions
UPDATE block_definitions
SET status = 'draft'
WHERE status IS NULL OR status = '';
