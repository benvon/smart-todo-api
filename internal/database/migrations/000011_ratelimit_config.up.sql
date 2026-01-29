CREATE TABLE ratelimit_config (
    config_key TEXT PRIMARY KEY DEFAULT 'default',
    rate TEXT NOT NULL DEFAULT '5-S',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO ratelimit_config (config_key, rate) VALUES ('default', '5-S');
