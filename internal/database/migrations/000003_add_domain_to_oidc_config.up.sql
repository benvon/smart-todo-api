-- Add domain column to oidc_config for OAuth2 domain support (e.g., Cognito custom domains)
ALTER TABLE oidc_config ADD COLUMN domain TEXT;
