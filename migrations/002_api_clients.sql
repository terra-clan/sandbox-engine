-- API clients table for authentication
CREATE TABLE IF NOT EXISTS api_clients (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    api_key VARCHAR(64) UNIQUE NOT NULL,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_used_at TIMESTAMP WITH TIME ZONE,
    permissions JSONB DEFAULT '["sandboxes:read", "sandboxes:write", "templates:read"]',
    metadata JSONB DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_api_clients_api_key ON api_clients(api_key);

-- Insert default clients (for development)
INSERT INTO api_clients (name, api_key, permissions) VALUES
    ('terra-hiring', 'sk_dev_terrahiring_xxxxxxxxxxxxx', '["sandboxes:*", "templates:*"]'),
    ('sota-projects', 'sk_dev_sotaprojects_xxxxxxxxxxxxx', '["sandboxes:*", "templates:*"]')
ON CONFLICT DO NOTHING;
