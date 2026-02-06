-- Add task_description field to sessions for storing task descriptions (markdown)
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS task_description TEXT DEFAULT '';
