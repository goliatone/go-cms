ALTER TABLE widget_area_definitions
DROP CONSTRAINT IF EXISTS widget_area_definitions_scope_check;

ALTER TABLE widget_area_definitions
ADD CONSTRAINT widget_area_definitions_scope_check
CHECK (scope IN ('global', 'theme', 'template', 'page'));
