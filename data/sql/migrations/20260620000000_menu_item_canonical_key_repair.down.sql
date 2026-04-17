---bun:dialect: postgres

DROP INDEX IF EXISTS idx_menu_items_menu_canonical_key;

ALTER TABLE menu_items
    DROP COLUMN IF EXISTS canonical_key;
