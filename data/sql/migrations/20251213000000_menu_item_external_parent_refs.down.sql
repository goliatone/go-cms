DROP INDEX IF EXISTS idx_menu_items_menu_parent_ref;
DROP INDEX IF EXISTS idx_menu_items_menu_external_code;

ALTER TABLE menu_items
    DROP COLUMN IF EXISTS parent_ref,
    DROP COLUMN IF EXISTS external_code;

