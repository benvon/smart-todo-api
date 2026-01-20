-- Add due_date column to todos table
ALTER TABLE todos ADD COLUMN due_date TIMESTAMP;

CREATE INDEX idx_todos_due_date ON todos(due_date);
