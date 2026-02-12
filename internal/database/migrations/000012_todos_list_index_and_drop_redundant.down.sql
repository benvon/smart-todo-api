-- Recreate dropped indexes
CREATE INDEX idx_ai_context_user_id ON ai_context(user_id);
CREATE INDEX idx_tag_statistics_user_id ON tag_statistics(user_id);

-- Drop composite index
DROP INDEX IF EXISTS idx_todos_user_id_created_at_desc;
