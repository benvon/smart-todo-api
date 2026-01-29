CREATE TABLE cors_config (
    config_key TEXT PRIMARY KEY DEFAULT 'default',
    allowed_origins TEXT NOT NULL DEFAULT 'http://localhost:3000',
    allow_credentials BOOLEAN NOT NULL DEFAULT true,
    max_age INTEGER NOT NULL DEFAULT 86400,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO cors_config (config_key, allowed_origins) VALUES ('default', 'http://localhost:3000');
