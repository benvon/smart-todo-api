-- Rename 'now' to 'next' in time_horizon enum
ALTER TYPE time_horizon RENAME VALUE 'now' TO 'next';
