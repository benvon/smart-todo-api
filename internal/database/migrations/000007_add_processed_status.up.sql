-- Add 'processed' status to todo_status enum
-- Use a DO block to check if the value exists before adding (for idempotency)
DO $$ 
BEGIN
    -- Check if 'processed' value already exists in the enum
    IF NOT EXISTS (
        SELECT 1 FROM pg_enum 
        WHERE enumlabel = 'processed' 
        AND enumtypid = (SELECT oid FROM pg_type WHERE typname = 'todo_status')
    ) THEN
        -- Add the value if it doesn't exist
        ALTER TYPE todo_status ADD VALUE 'processed';
    END IF;
END $$;
