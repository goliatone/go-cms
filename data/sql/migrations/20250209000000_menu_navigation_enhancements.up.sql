-- Enhanced menu metadata: types, collapsible hints, and i18n keys
ALTER TABLE menu_items
    ADD COLUMN type TEXT NOT NULL DEFAULT 'item',
    ADD COLUMN collapsible BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN collapsed BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN icon TEXT,
    ADD COLUMN badge JSONB,
    ADD COLUMN permissions TEXT[],
    ADD COLUMN classes TEXT[],
    ADD COLUMN styles JSONB;

---bun:split

ALTER TABLE menu_item_translations
    ADD COLUMN label_key TEXT,
    ADD COLUMN group_title TEXT,
    ADD COLUMN group_title_key TEXT;
