-- Locales table: supported languages for the CMS
CREATE TABLE locales (
    id UUID PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    native_name TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    is_default BOOLEAN NOT NULL DEFAULT false,
    metadata JSONB,
    deleted_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_locales_code ON locales(code);
CREATE INDEX idx_locales_is_active ON locales(is_active);
CREATE INDEX idx_locales_is_default ON locales(is_default);

---bun:split

-- Content types: defines available content schemas
CREATE TABLE content_types (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    schema JSONB NOT NULL,
    capabilities JSONB,
    icon TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_content_types_name ON content_types(name);

---bun:split

-- Contents: canonical records for translatable entries
CREATE TABLE contents (
    id UUID PRIMARY KEY,
    content_type_id UUID NOT NULL REFERENCES content_types(id) ON DELETE RESTRICT,
    current_version INTEGER NOT NULL DEFAULT 1,
    published_version INTEGER,
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived', 'scheduled')),
    slug TEXT NOT NULL,
    publish_at TIMESTAMP,
    unpublish_at TIMESTAMP,
    published_at TIMESTAMP,
    published_by UUID,
    created_by UUID NOT NULL,
    updated_by UUID NOT NULL,
    deleted_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_contents_content_type_id ON contents(content_type_id);
CREATE INDEX idx_contents_slug ON contents(slug);
CREATE INDEX idx_contents_status ON contents(status);
CREATE INDEX idx_contents_published_at ON contents(published_at);
CREATE INDEX idx_contents_deleted_at ON contents(deleted_at);

---bun:split

-- Content translations: localized variants
CREATE TABLE content_translations (
    id UUID PRIMARY KEY,
    content_id UUID NOT NULL REFERENCES contents(id) ON DELETE CASCADE,
    locale_id UUID NOT NULL REFERENCES locales(id) ON DELETE RESTRICT,
    title TEXT NOT NULL,
    summary TEXT,
    content JSONB NOT NULL,
    deleted_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(content_id, locale_id)
);

CREATE INDEX idx_content_translations_content_id ON content_translations(content_id);
CREATE INDEX idx_content_translations_locale_id ON content_translations(locale_id);

---bun:split

-- Content versions: immutable snapshots
CREATE TABLE content_versions (
    id UUID PRIMARY KEY,
    content_id UUID NOT NULL REFERENCES contents(id) ON DELETE CASCADE,
    version INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    snapshot JSONB NOT NULL,
    created_by UUID NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    published_at TIMESTAMP,
    published_by UUID,
    UNIQUE(content_id, version)
);

CREATE INDEX idx_content_versions_content_id ON content_versions(content_id);
CREATE INDEX idx_content_versions_version ON content_versions(content_id, version);

---bun:split

-- Themes: complete site designs
CREATE TABLE themes (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    version TEXT NOT NULL,
    author TEXT,
    is_active BOOLEAN NOT NULL DEFAULT false,
    theme_path TEXT NOT NULL,
    config JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_themes_name ON themes(name);
CREATE INDEX idx_themes_is_active ON themes(is_active);

---bun:split

-- Templates: layout surfaces for pages
CREATE TABLE templates (
    id UUID PRIMARY KEY,
    theme_id UUID NOT NULL REFERENCES themes(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    description TEXT,
    template_path TEXT NOT NULL,
    regions JSONB NOT NULL,
    metadata JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_templates_theme_id ON templates(theme_id);
CREATE INDEX idx_templates_slug ON templates(slug);

---bun:split

-- Pages: hierarchical page metadata
CREATE TABLE pages (
    id UUID PRIMARY KEY,
    content_id UUID NOT NULL REFERENCES contents(id) ON DELETE RESTRICT,
    current_version INTEGER NOT NULL DEFAULT 1,
    published_version INTEGER,
    parent_id UUID REFERENCES pages(id) ON DELETE SET NULL,
    template_id UUID NOT NULL REFERENCES templates(id) ON DELETE RESTRICT,
    slug TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived', 'scheduled')),
    publish_at TIMESTAMP,
    unpublish_at TIMESTAMP,
    published_at TIMESTAMP,
    published_by UUID,
    created_by UUID NOT NULL,
    updated_by UUID NOT NULL,
    deleted_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_pages_content_id ON pages(content_id);
CREATE INDEX idx_pages_parent_id ON pages(parent_id);
CREATE INDEX idx_pages_template_id ON pages(template_id);
CREATE INDEX idx_pages_slug ON pages(slug);
CREATE INDEX idx_pages_status ON pages(status);
CREATE INDEX idx_pages_deleted_at ON pages(deleted_at);

---bun:split

-- Page translations: localized page metadata
CREATE TABLE page_translations (
    id UUID PRIMARY KEY,
    page_id UUID NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
    locale_id UUID NOT NULL REFERENCES locales(id) ON DELETE RESTRICT,
    title TEXT NOT NULL,
    path TEXT NOT NULL,
    seo_title TEXT,
    seo_description TEXT,
    summary TEXT,
    media_bindings JSONB,
    deleted_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(page_id, locale_id)
);

CREATE INDEX idx_page_translations_page_id ON page_translations(page_id);
CREATE INDEX idx_page_translations_locale_id ON page_translations(locale_id);
CREATE INDEX idx_page_translations_path ON page_translations(path);

---bun:split

-- Page versions: snapshots of structural layout
CREATE TABLE page_versions (
    id UUID PRIMARY KEY,
    page_id UUID NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
    version INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    snapshot JSONB NOT NULL,
    deleted_at TIMESTAMP,
    created_by UUID NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    published_at TIMESTAMP,
    published_by UUID,
    UNIQUE(page_id, version)
);

CREATE INDEX idx_page_versions_page_id ON page_versions(page_id);
CREATE INDEX idx_page_versions_version ON page_versions(page_id, version);

---bun:split

-- Block definitions: reusable block templates
CREATE TABLE block_definitions (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    icon TEXT,
    schema JSONB NOT NULL,
    defaults JSONB,
    editor_style_url TEXT,
    frontend_style_url TEXT,
    deleted_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_block_definitions_name ON block_definitions(name);
CREATE INDEX idx_block_definitions_deleted_at ON block_definitions(deleted_at);

---bun:split

-- Block instances: concrete usages
CREATE TABLE block_instances (
    id UUID PRIMARY KEY,
    page_id UUID REFERENCES pages(id) ON DELETE CASCADE,
    region TEXT NOT NULL,
    position INTEGER NOT NULL DEFAULT 0,
    definition_id UUID NOT NULL REFERENCES block_definitions(id) ON DELETE RESTRICT,
    configuration JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_global BOOLEAN NOT NULL DEFAULT false,
    current_version INTEGER NOT NULL DEFAULT 1,
    published_version INTEGER,
    published_at TIMESTAMP,
    published_by UUID,
    created_by UUID NOT NULL,
    updated_by UUID NOT NULL,
    deleted_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_block_instances_page_id ON block_instances(page_id);
CREATE INDEX idx_block_instances_definition_id ON block_instances(definition_id);
CREATE INDEX idx_block_instances_region ON block_instances(region);
CREATE INDEX idx_block_instances_is_global ON block_instances(is_global);
CREATE INDEX idx_block_instances_deleted_at ON block_instances(deleted_at);

---bun:split

-- Block translations: localized block content
CREATE TABLE block_translations (
    id UUID PRIMARY KEY,
    block_instance_id UUID NOT NULL REFERENCES block_instances(id) ON DELETE CASCADE,
    locale_id UUID NOT NULL REFERENCES locales(id) ON DELETE RESTRICT,
    content JSONB NOT NULL,
    attribute_overrides JSONB,
    media_bindings JSONB,
    deleted_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(block_instance_id, locale_id)
);

CREATE INDEX idx_block_translations_block_instance_id ON block_translations(block_instance_id);
CREATE INDEX idx_block_translations_locale_id ON block_translations(locale_id);

---bun:split

-- Block versions: snapshots of block instances
CREATE TABLE block_versions (
    id UUID PRIMARY KEY,
    block_instance_id UUID NOT NULL REFERENCES block_instances(id) ON DELETE CASCADE,
    version INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    snapshot JSONB NOT NULL,
    created_by UUID NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    published_at TIMESTAMP,
    published_by UUID,
    UNIQUE(block_instance_id, version)
);

CREATE INDEX idx_block_versions_block_instance_id ON block_versions(block_instance_id);
CREATE INDEX idx_block_versions_version ON block_versions(block_instance_id, version);

---bun:split

-- Widget definitions: widget types and schemas
CREATE TABLE widget_definitions (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    schema JSONB NOT NULL,
    defaults JSONB,
    category TEXT,
    icon TEXT,
    deleted_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_widget_definitions_name ON widget_definitions(name);
CREATE INDEX idx_widget_definitions_category ON widget_definitions(category);
CREATE INDEX idx_widget_definitions_deleted_at ON widget_definitions(deleted_at);

---bun:split

-- Widget instances: concrete widget placements
CREATE TABLE widget_instances (
    id UUID PRIMARY KEY,
    definition_id UUID NOT NULL REFERENCES widget_definitions(id) ON DELETE RESTRICT,
    block_instance_id UUID REFERENCES block_instances(id) ON DELETE CASCADE,
    area_code TEXT,
    placement_metadata JSONB,
    configuration JSONB NOT NULL DEFAULT '{}'::jsonb,
    visibility_rules JSONB,
    publish_on TIMESTAMP,
    unpublish_on TIMESTAMP,
    position INTEGER NOT NULL DEFAULT 0,
    created_by UUID NOT NULL,
    updated_by UUID NOT NULL,
    deleted_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_widget_instances_definition_id ON widget_instances(definition_id);
CREATE INDEX idx_widget_instances_block_instance_id ON widget_instances(block_instance_id);
CREATE INDEX idx_widget_instances_area_code ON widget_instances(area_code);
CREATE INDEX idx_widget_instances_deleted_at ON widget_instances(deleted_at);

---bun:split

-- Widget translations: localized widget data
CREATE TABLE widget_translations (
    id UUID PRIMARY KEY,
    widget_instance_id UUID NOT NULL REFERENCES widget_instances(id) ON DELETE CASCADE,
    locale_id UUID NOT NULL REFERENCES locales(id) ON DELETE RESTRICT,
    content JSONB NOT NULL,
    deleted_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(widget_instance_id, locale_id)
);

CREATE INDEX idx_widget_translations_widget_instance_id ON widget_translations(widget_instance_id);
CREATE INDEX idx_widget_translations_locale_id ON widget_translations(locale_id);

---bun:split

-- Widget area definitions: named regions for widgets
CREATE TABLE widget_area_definitions (
    id UUID PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    description TEXT,
    scope TEXT NOT NULL DEFAULT 'global' CHECK (scope IN ('global', 'theme', 'template')),
    theme_id UUID REFERENCES themes(id) ON DELETE CASCADE,
    template_id UUID REFERENCES templates(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_widget_area_definitions_code ON widget_area_definitions(code);
CREATE INDEX idx_widget_area_definitions_scope ON widget_area_definitions(scope);
CREATE INDEX idx_widget_area_definitions_theme_id ON widget_area_definitions(theme_id);
CREATE INDEX idx_widget_area_definitions_template_id ON widget_area_definitions(template_id);

---bun:split

-- Widget area placements: binds widgets to areas
CREATE TABLE widget_area_placements (
    id UUID PRIMARY KEY,
    area_code TEXT NOT NULL,
    locale_id UUID REFERENCES locales(id) ON DELETE CASCADE,
    instance_id UUID NOT NULL REFERENCES widget_instances(id) ON DELETE CASCADE,
    position INTEGER NOT NULL DEFAULT 0,
    metadata JSONB,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_widget_area_placements_area_code ON widget_area_placements(area_code);
CREATE INDEX idx_widget_area_placements_locale_id ON widget_area_placements(locale_id);
CREATE INDEX idx_widget_area_placements_instance_id ON widget_area_placements(instance_id);
CREATE INDEX idx_widget_area_placements_area_locale ON widget_area_placements(area_code, locale_id);

---bun:split

-- Menus: navigational containers
CREATE TABLE menus (
    id UUID PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    description TEXT,
    created_by UUID NOT NULL,
    updated_by UUID NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_menus_code ON menus(code);

---bun:split

-- Menu items: individual navigational entries
CREATE TABLE menu_items (
    id UUID PRIMARY KEY,
    menu_id UUID NOT NULL REFERENCES menus(id) ON DELETE CASCADE,
    parent_id UUID REFERENCES menu_items(id) ON DELETE CASCADE,
    position INTEGER NOT NULL DEFAULT 0,
    target JSONB NOT NULL,
    created_by UUID NOT NULL,
    updated_by UUID NOT NULL,
    deleted_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_menu_items_menu_id ON menu_items(menu_id);
CREATE INDEX idx_menu_items_parent_id ON menu_items(parent_id);
CREATE INDEX idx_menu_items_deleted_at ON menu_items(deleted_at);

---bun:split

-- Menu item translations: localized menu metadata
CREATE TABLE menu_item_translations (
    id UUID PRIMARY KEY,
    menu_item_id UUID NOT NULL REFERENCES menu_items(id) ON DELETE CASCADE,
    locale_id UUID NOT NULL REFERENCES locales(id) ON DELETE RESTRICT,
    label TEXT NOT NULL,
    url_override TEXT,
    deleted_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(menu_item_id, locale_id)
);

CREATE INDEX idx_menu_item_translations_menu_item_id ON menu_item_translations(menu_item_id);
CREATE INDEX idx_menu_item_translations_locale_id ON menu_item_translations(locale_id);
