-- API clients table for authentication
CREATE TABLE IF NOT EXISTS api_clients (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    api_key VARCHAR(64) UNIQUE NOT NULL,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_used_at TIMESTAMP WITH TIME ZONE,
    permissions JSONB DEFAULT '["sandboxes:read", "sandboxes:write", "templates:read", "sessions:read", "sessions:write"]',
    metadata JSONB DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_api_clients_api_key ON api_clients(api_key);

-- Insert default clients (for development)
INSERT INTO api_clients (name, api_key, permissions) VALUES
    ('terra-sandbox', 'sk_dev_terrasandbox_xxxxxxxxxxxxx', '["sandboxes:*", "templates:*", "sessions:*"]'),
    ('terra-admin', 'sk_dev_terraadmin_xxxxxxxxxxxxx', '["sandboxes:*", "templates:*", "sessions:*"]')
ON CONFLICT DO NOTHING;
