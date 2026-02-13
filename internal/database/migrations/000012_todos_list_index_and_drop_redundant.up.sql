-- Add composite index for list-todos workload: WHERE user_id = $1 ORDER BY created_at DESC
CREATE INDEX idx_todos_user_id_created_at_desc ON todos(user_id, created_at DESC);

-- Drop redundant indexes (PK/UNIQUE already provide an index on user_id)
DROP INDEX IF EXISTS idx_tag_statistics_user_id;
DROP INDEX IF EXISTS idx_ai_context_user_id;
