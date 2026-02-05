-- Sessions table for deferred sandbox provisioning
-- Sessions are created by admins/orchestrators and activated when candidates open the join link
CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token VARCHAR(64) UNIQUE NOT NULL,
    template_id VARCHAR(100) NOT NULL,
    status VARCHAR(20) DEFAULT 'ready',
    status_message TEXT DEFAULT '',

    -- Config for deferred sandbox creation
    env JSONB DEFAULT '{}',
    metadata JSONB DEFAULT '{}',
    ttl_seconds INTEGER NOT NULL,

    -- Link to sandbox (populated on activation)
    sandbox_id VARCHAR(36),

    -- Timestamps
    created_at TIMESTAMPTZ DEFAULT NOW(),
    activated_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,

    created_by VARCHAR(100)
);

CREATE INDEX IF NOT EXISTS idx_sessions_token ON sessions(token);
CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);
