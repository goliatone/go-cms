DROP INDEX IF EXISTS idx_menu_items_menu_canonical_key;

ALTER TABLE menu_items
    DROP COLUMN canonical_key;
