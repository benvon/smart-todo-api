-- Make client_secret nullable to support public OIDC clients (no secret required)
ALTER TABLE oidc_config ALTER COLUMN client_secret DROP NOT NULL;
