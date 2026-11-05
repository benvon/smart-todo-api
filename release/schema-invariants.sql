-- Every invariant returns exactly one true boolean in a read-only transaction.
-- Catalog checks are bounded independently of application data volume.
SELECT
    to_regclass('public.users') IS NOT NULL
    AND to_regclass('public.todos') IS NOT NULL
    AND to_regclass('public.ai_context') IS NOT NULL
    AND to_regclass('public.user_activity') IS NOT NULL
    AND to_regclass('public.tag_statistics') IS NOT NULL
    AND EXISTS (
        SELECT 1 FROM pg_attribute
        WHERE attrelid = to_regclass('public.todos')
          AND attname = 'due_date' AND NOT attisdropped
    )
    AND EXISTS (
        SELECT 1 FROM pg_index
        WHERE indexrelid = to_regclass('public.idx_todos_user_id_created_at_desc')
          AND indisvalid AND indisready
    );
