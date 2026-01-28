-- Remove backfilled _schema entries from content translations
UPDATE content_translations
SET content = json_remove(content, '$._schema')
WHERE json_extract(content, '$._schema') IS NOT NULL;
