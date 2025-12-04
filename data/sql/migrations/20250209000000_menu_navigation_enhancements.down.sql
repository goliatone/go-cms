ALTER TABLE menu_item_translations
    DROP COLUMN group_title_key,
    DROP COLUMN group_title,
    DROP COLUMN label_key;

---bun:split

ALTER TABLE menu_items
    DROP COLUMN styles,
    DROP COLUMN classes,
    DROP COLUMN permissions,
    DROP COLUMN badge,
    DROP COLUMN icon,
    DROP COLUMN metadata,
    DROP COLUMN collapsed,
    DROP COLUMN collapsible,
    DROP COLUMN type;
