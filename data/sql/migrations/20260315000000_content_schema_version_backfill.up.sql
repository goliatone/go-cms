-- Backfill root _schema for existing content translations
UPDATE content_translations AS ct
SET content = jsonb_set(
    ct.content,
    '{_schema}',
    to_jsonb(
        COALESCE(
            (cty.schema->'metadata'->>'schema_version'),
            COALESCE(cty.slug, cty.name) || '@v1.0.0'
        )
    ),
    true
)
FROM contents AS c
JOIN content_types AS cty ON cty.id = c.content_type_id
WHERE ct.content_id = c.id
  AND NOT (ct.content ? '_schema');
