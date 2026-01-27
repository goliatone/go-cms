DROP INDEX IF EXISTS idx_menu_items_menu_parent_ref;
DROP INDEX IF EXISTS idx_menu_items_menu_external_code;

-- SQLite does not support dropping columns via ALTER TABLE.
-- No-op for menu_items.external_code and menu_items.parent_ref.
