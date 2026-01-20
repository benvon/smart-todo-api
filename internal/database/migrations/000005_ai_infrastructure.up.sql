-- Create user_activity table for tracking user activity and reprocessing status
CREATE TABLE user_activity (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    last_api_interaction TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    reprocessing_paused BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_user_activity_last_api_interaction ON user_activity(last_api_interaction);
CREATE INDEX idx_user_activity_reprocessing_paused ON user_activity(reprocessing_paused);

-- Create ai_context table for storing user AI context and preferences
CREATE TABLE ai_context (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    context_summary TEXT,
    preferences JSONB DEFAULT '{}',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id)
);

CREATE INDEX idx_ai_context_user_id ON ai_context(user_id);
