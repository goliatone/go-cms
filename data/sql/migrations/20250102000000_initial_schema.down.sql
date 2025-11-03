-- Drop tables in reverse order to respect foreign key constraints

DROP TABLE IF EXISTS menu_item_translations;
DROP TABLE IF EXISTS menu_items;
DROP TABLE IF EXISTS menus;
DROP TABLE IF EXISTS widget_area_placements;
DROP TABLE IF EXISTS widget_area_definitions;
DROP TABLE IF EXISTS widget_translations;
DROP TABLE IF EXISTS widget_instances;
DROP TABLE IF EXISTS widget_definitions;
DROP TABLE IF EXISTS block_versions;
DROP TABLE IF EXISTS block_translations;
DROP TABLE IF EXISTS block_instances;
DROP TABLE IF EXISTS block_definitions;
DROP TABLE IF EXISTS page_versions;
DROP TABLE IF EXISTS page_translations;
DROP TABLE IF EXISTS pages;
DROP TABLE IF EXISTS templates;
DROP TABLE IF EXISTS themes;
DROP TABLE IF EXISTS content_versions;
DROP TABLE IF EXISTS content_translations;
DROP TABLE IF EXISTS contents;
DROP TABLE IF EXISTS content_types;
DROP TABLE IF EXISTS locales;
