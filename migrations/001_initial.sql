-- Migration: 001_initial
-- Created: 2024
-- Description: Initial schema for sandbox-engine

-- Таблица sandboxes
CREATE TABLE IF NOT EXISTS sandboxes (
    id VARCHAR(12) PRIMARY KEY,
    template_id VARCHAR(255) NOT NULL,
    user_id VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    status_message TEXT,
    container_id VARCHAR(64),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    started_at TIMESTAMP WITH TIME ZONE,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    metadata JSONB DEFAULT '{}',
    endpoints JSONB DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_sandboxes_user_id ON sandboxes(user_id);
CREATE INDEX IF NOT EXISTS idx_sandboxes_status ON sandboxes(status);
CREATE INDEX IF NOT EXISTS idx_sandboxes_expires_at ON sandboxes(expires_at);
CREATE INDEX IF NOT EXISTS idx_sandboxes_template_id ON sandboxes(template_id);

-- Таблица sandbox_services (provisioned services для каждой песочницы)
CREATE TABLE IF NOT EXISTS sandbox_services (
    id SERIAL PRIMARY KEY,
    sandbox_id VARCHAR(12) REFERENCES sandboxes(id) ON DELETE CASCADE,
    service_name VARCHAR(100) NOT NULL,
    service_type VARCHAR(100) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    credentials JSONB,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(sandbox_id, service_name)
);

CREATE INDEX IF NOT EXISTS idx_sandbox_services_sandbox_id ON sandbox_services(sandbox_id);
