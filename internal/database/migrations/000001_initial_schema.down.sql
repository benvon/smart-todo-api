-- Drop tables in reverse order
DROP TABLE IF EXISTS oidc_config;
DROP TABLE IF EXISTS todos;
DROP TABLE IF EXISTS users;

-- Drop enum types
DROP TYPE IF EXISTS todo_status;
DROP TYPE IF EXISTS time_horizon;
