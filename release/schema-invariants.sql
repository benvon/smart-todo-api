-- Every invariant returns exactly one true boolean in a read-only transaction.
-- Catalog checks are bounded independently of application data volume.
-- Resolve application tables like the application's unqualified queries do.
SELECT
    to_regclass('users') IS NOT NULL
    AND to_regclass('todos') IS NOT NULL
    AND to_regclass('ai_context') IS NOT NULL
    AND to_regclass('user_activity') IS NOT NULL
    AND to_regclass('tag_statistics') IS NOT NULL
    AND EXISTS (
        SELECT 1 FROM pg_attribute
        WHERE attrelid = to_regclass('todos')
          AND attname = 'due_date' AND NOT attisdropped
    )
    AND EXISTS (
        SELECT 1 FROM pg_index i JOIN pg_class c ON c.oid = i.indexrelid
        WHERE i.indrelid = to_regclass('todos')
          AND c.relname = 'idx_todos_user_id_created_at_desc'
          AND i.indisvalid AND i.indisready
    );
