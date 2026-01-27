-- Create tag_statistics table for storing aggregated tag analysis results
CREATE TABLE tag_statistics (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    tag_stats JSONB NOT NULL DEFAULT '{}',
    tainted BOOLEAN NOT NULL DEFAULT true,
    last_analyzed_at TIMESTAMP,
    analysis_version INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_tag_statistics_user_id ON tag_statistics(user_id);
CREATE INDEX idx_tag_statistics_tainted ON tag_statistics(tainted);
CREATE INDEX idx_tag_statistics_last_analyzed_at ON tag_statistics(last_analyzed_at);
