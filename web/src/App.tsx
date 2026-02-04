import React, { useState, useEffect } from 'react';
import { Workspace } from './views/Workspace';
import { SandboxInfo } from './types';

// API response from sandbox-engine
interface ApiSandbox {
  id: string;
  template_id: string;
  user_id: string;
  status: string;
  created_at: string;
  started_at?: string;
  expires_at: string;
  container_id?: string;
  services?: Record<string, {
    name: string;
    type: string;
    status: string;
    credentials: Record<string, any>;
  }>;
  endpoints?: Record<string, string>;
}

interface ApiResponse {
  success: boolean;
  data?: ApiSandbox;
  error?: { code: string; message: string } | string;
}

// Transform API response to frontend format
const transformSandbox = (api: ApiSandbox): SandboxInfo => {
  const services = api.services ? Object.entries(api.services).map(([key, svc]) => ({
    name: svc.name,
    port: svc.credentials?.port || 0,
    status: svc.status === 'ready' ? 'running' as const : 'starting' as const,
    url: svc.credentials?.uri
  })) : [];

  return {
    id: api.id,
    templateId: api.template_id,
    status: api.status as 'creating' | 'running' | 'stopped' | 'error',
    createdAt: api.created_at,
    expiresAt: api.expires_at,
    services,
    workDir: '/workspace'
  };
};

// Get config from URL parameters
const getConfigFromUrl = () => {
  const params = new URLSearchParams(window.location.search);

  return {
    sandboxId: params.get('sandbox') || '',
    apiToken: params.get('token') || '',
    apiBaseUrl: params.get('api') || 'https://api.terra-sandbox.ru',
    wsBaseUrl: params.get('ws') || 'wss://api.terra-sandbox.ru'
  };
};

const App: React.FC = () => {
  const [config] = useState(getConfigFromUrl);
  const [sandbox, setSandbox] = useState<SandboxInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!config.sandboxId) {
      setError('No sandbox ID provided. Add ?sandbox=xxx to URL');
      setLoading(false);
      return;
    }

    if (!config.apiToken) {
      setError('No API token provided. Add &token=xxx to URL');
      setLoading(false);
      return;
    }

    fetchSandboxInfo();

    // Poll for updates every 5 seconds
    const interval = setInterval(fetchSandboxInfo, 5000);

    return () => clearInterval(interval);
  }, [config.sandboxId, config.apiToken]);

  const fetchSandboxInfo = async () => {
    try {
      const response = await fetch(
        `${config.apiBaseUrl}/api/v1/sandboxes/${config.sandboxId}`,
        {
          headers: {
            'Authorization': `Bearer ${config.apiToken}`,
            'Content-Type': 'application/json'
          }
        }
      );

      if (!response.ok) {
        if (response.status === 401) {
          throw new Error('Invalid or expired API token');
        }
        if (response.status === 404) {
          throw new Error('Sandbox not found');
        }
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      }

      const data: ApiResponse = await response.json();

      if (data.success && data.data) {
        const transformed = transformSandbox(data.data);
        setSandbox(transformed);
        setError(null);
      } else {
        const errMsg = typeof data.error === 'string' ? data.error : data.error?.message || 'Unknown error';
        throw new Error(errMsg);
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to fetch sandbox info';
      setError(message);
      console.error('Failed to fetch sandbox:', err);
    } finally {
      setLoading(false);
    }
  };

  // Show error page if no config
  if (!config.sandboxId || !config.apiToken) {
    return (
      <div className="h-screen bg-slate-900 flex items-center justify-center">
        <div className="max-w-md text-center px-4">
          <h1 className="text-2xl font-bold text-white mb-4">Terra Sandbox</h1>
          <div className="bg-slate-800 rounded-lg p-6 border border-slate-700">
            <p className="text-red-400 mb-4">{error}</p>
            <p className="text-slate-400 text-sm mb-4">
              To access a sandbox, use the URL format:
            </p>
            <code className="block bg-slate-950 text-cyan-400 p-3 rounded text-sm break-all">
              {window.location.origin}/?sandbox=SANDBOX_ID&token=API_TOKEN
            </code>
          </div>
        </div>
      </div>
    );
  }

  return (
    <Workspace
      sandbox={sandbox}
      sandboxId={config.sandboxId}
      apiToken={config.apiToken}
      apiBaseUrl={config.apiBaseUrl}
      wsBaseUrl={config.wsBaseUrl}
      loading={loading}
      error={error}
    />
  );
};

export default App;
