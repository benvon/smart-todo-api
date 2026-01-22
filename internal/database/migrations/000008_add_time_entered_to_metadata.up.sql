-- Migration 000008: Add time_entered to metadata
-- This migration documents the time_entered field in the metadata JSONB structure.
-- No schema changes are needed as metadata is stored as JSONB and can accommodate new fields.
-- The time_entered field stores an ISO8601 timestamp indicating when the todo was entered,
-- which helps the AI understand relative time expressions like "this weekend" or "soon"
-- based on when the todo was created.

-- This is a documentation-only migration. The time_entered field will be automatically
-- populated by the application when todos are created.
