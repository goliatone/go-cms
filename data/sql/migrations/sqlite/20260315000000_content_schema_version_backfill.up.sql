-- Backfill root _schema for existing content translations
UPDATE content_translations
SET content = json_set(
    content,
    '$._schema',
    COALESCE(
        (SELECT json_extract(ct.schema, '$.metadata.schema_version')
         FROM contents c
         JOIN content_types ct ON ct.id = c.content_type_id
         WHERE c.id = content_translations.content_id),
        (SELECT COALESCE(ct.slug, ct.name) || '@v1.0.0'
         FROM contents c
         JOIN content_types ct ON ct.id = c.content_type_id
         WHERE c.id = content_translations.content_id)
    )
)
WHERE json_extract(content, '$._schema') IS NULL;
