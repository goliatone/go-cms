-- Menu items: stable external identifiers and deferred parent linking
ALTER TABLE menu_items
    ADD COLUMN external_code TEXT,
    ADD COLUMN parent_ref TEXT;

CREATE INDEX IF NOT EXISTS idx_menu_items_menu_external_code
    ON menu_items(menu_id, external_code)
    WHERE external_code IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_menu_items_menu_parent_ref
    ON menu_items(menu_id, parent_ref)
    WHERE parent_ref IS NOT NULL;

