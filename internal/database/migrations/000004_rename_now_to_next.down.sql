-- Revert: Rename 'next' back to 'now' in time_horizon enum
ALTER TYPE time_horizon RENAME VALUE 'next' TO 'now';
