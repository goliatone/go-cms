-- Enhanced menu metadata: types, collapsible hints, and i18n keys
ALTER TABLE menu_items ADD COLUMN type TEXT NOT NULL DEFAULT 'item';
ALTER TABLE menu_items ADD COLUMN collapsible INTEGER NOT NULL DEFAULT 0;
ALTER TABLE menu_items ADD COLUMN collapsed INTEGER NOT NULL DEFAULT 0;
ALTER TABLE menu_items ADD COLUMN metadata TEXT NOT NULL DEFAULT '{}';
ALTER TABLE menu_items ADD COLUMN icon TEXT;
ALTER TABLE menu_items ADD COLUMN badge TEXT;
ALTER TABLE menu_items ADD COLUMN permissions TEXT;
ALTER TABLE menu_items ADD COLUMN classes TEXT;
ALTER TABLE menu_items ADD COLUMN styles TEXT;

---bun:split

ALTER TABLE menu_item_translations ADD COLUMN label_key TEXT;
ALTER TABLE menu_item_translations ADD COLUMN group_title TEXT;
ALTER TABLE menu_item_translations ADD COLUMN group_title_key TEXT;
