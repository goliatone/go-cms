DROP INDEX IF EXISTS idx_menu_items_menu_parent_ref;
DROP INDEX IF EXISTS idx_menu_items_menu_external_code;

ALTER TABLE menu_items
    DROP COLUMN parent_ref;

ALTER TABLE menu_items
    DROP COLUMN external_code;
