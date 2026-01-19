-- Create enum types
CREATE TYPE time_horizon AS ENUM ('now', 'soon', 'later');
CREATE TYPE todo_status AS ENUM ('pending', 'processing', 'completed');

-- Users table
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL UNIQUE,
    provider_id VARCHAR(255),
    name VARCHAR(255),
    email_verified BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_provider_id ON users(provider_id);

-- Todos table
CREATE TABLE todos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    text TEXT NOT NULL,
    time_horizon time_horizon NOT NULL DEFAULT 'soon',
    status todo_status NOT NULL DEFAULT 'pending',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP
);

CREATE INDEX idx_todos_user_id ON todos(user_id);
CREATE INDEX idx_todos_time_horizon ON todos(time_horizon);
CREATE INDEX idx_todos_status ON todos(status);
CREATE INDEX idx_todos_metadata ON todos USING GIN(metadata);

-- OIDC Configuration table
CREATE TABLE oidc_config (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider VARCHAR(100) NOT NULL UNIQUE,
    issuer VARCHAR(500) NOT NULL,
    client_id VARCHAR(255) NOT NULL,
    client_secret TEXT NOT NULL,
    redirect_uri VARCHAR(500) NOT NULL,
    jwks_url VARCHAR(500),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_oidc_config_provider ON oidc_config(provider);
