-- Revert client_secret to NOT NULL (note: existing NULL values will cause error)
ALTER TABLE oidc_config ALTER COLUMN client_secret SET NOT NULL;
