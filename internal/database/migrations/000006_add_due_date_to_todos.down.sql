-- Remove due_date column from todos table
DROP INDEX IF EXISTS idx_todos_due_date;

ALTER TABLE todos DROP COLUMN IF EXISTS due_date;
