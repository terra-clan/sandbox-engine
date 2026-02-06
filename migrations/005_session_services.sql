-- Add services column to sessions table
-- Stores list of service names (e.g. ["postgres", "redis"]) requested for this session
-- If NULL/empty, falls back to template's services list
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS services JSONB DEFAULT '[]';
