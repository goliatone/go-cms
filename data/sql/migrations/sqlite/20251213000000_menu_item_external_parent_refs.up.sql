-- Menu items: stable external identifiers and deferred parent linking
ALTER TABLE menu_items
    ADD COLUMN external_code TEXT;

ALTER TABLE menu_items
    ADD COLUMN parent_ref TEXT;

CREATE INDEX IF NOT EXISTS idx_menu_items_menu_external_code
    ON menu_items(menu_id, external_code);

CREATE INDEX IF NOT EXISTS idx_menu_items_menu_parent_ref
    ON menu_items(menu_id, parent_ref);
