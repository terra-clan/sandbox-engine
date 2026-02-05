import React, { useState, useEffect } from 'react';
import { BrowserRouter, Routes, Route, useParams } from 'react-router-dom';
import { Workspace } from './views/Workspace';
import { JoinPage } from './views/JoinPage';
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
  const services = api.services ? Object.entries(api.services).map(([_, svc]) => ({
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

// Resolve base URLs from current location or query params
const resolveBaseUrls = () => {
  const params = new URLSearchParams(window.location.search);
  const proto = window.location.protocol === 'https:' ? 'https' : 'http';
  const wsProto = proto === 'https' ? 'wss' : 'ws';
  const host = window.location.host;

  return {
    apiBaseUrl: params.get('api') || `${proto}://${host}`,
    wsBaseUrl: params.get('ws') || `${wsProto}://${host}`,
  };
};

// --- Join route ---

const JoinRoute: React.FC = () => {
  const { token } = useParams<{ token: string }>();
  const { apiBaseUrl, wsBaseUrl } = resolveBaseUrls();

  if (!token) {
    return (
      <div className="h-screen bg-slate-900 flex items-center justify-center">
        <p className="text-red-400">Invalid join link</p>
      </div>
    );
  }

  return <JoinPage token={token} apiBaseUrl={apiBaseUrl} wsBaseUrl={wsBaseUrl} />;
};

// --- Direct workspace route (legacy: ?sandbox=&token=) ---

const WorkspaceRoute: React.FC = () => {
  const params = new URLSearchParams(window.location.search);
  const sandboxId = params.get('sandbox') || '';
  const apiToken = params.get('token') || '';
  const { apiBaseUrl, wsBaseUrl } = resolveBaseUrls();

  const [sandbox, setSandbox] = useState<SandboxInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!sandboxId || !apiToken) {
      setError('No sandbox ID or API token provided');
      setLoading(false);
      return;
    }

    const fetchSandboxInfo = async () => {
      try {
        const response = await fetch(
          `${apiBaseUrl}/api/v1/sandboxes/${sandboxId}`,
          {
            headers: {
              'Authorization': `Bearer ${apiToken}`,
              'Content-Type': 'application/json'
            }
          }
        );

        if (!response.ok) {
          if (response.status === 401) throw new Error('Invalid or expired API token');
          if (response.status === 404) throw new Error('Sandbox not found');
          throw new Error(`HTTP ${response.status}: ${response.statusText}`);
        }

        const data: ApiResponse = await response.json();
        if (data.success && data.data) {
          setSandbox(transformSandbox(data.data));
          setError(null);
        } else {
          const errMsg = typeof data.error === 'string' ? data.error : data.error?.message || 'Unknown error';
          throw new Error(errMsg);
        }
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Failed to fetch sandbox info';
        setError(message);
      } finally {
        setLoading(false);
      }
    };

    fetchSandboxInfo();
    const interval = setInterval(fetchSandboxInfo, 5000);
    return () => clearInterval(interval);
  }, [sandboxId, apiToken, apiBaseUrl]);

  if (!sandboxId || !apiToken) {
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
      sandboxId={sandboxId}
      apiToken={apiToken}
      apiBaseUrl={apiBaseUrl}
      wsBaseUrl={wsBaseUrl}
      loading={loading}
      error={error}
    />
  );
};

// --- App with routing ---

const App: React.FC = () => {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/join/:token" element={<JoinRoute />} />
        <Route path="*" element={<WorkspaceRoute />} />
      </Routes>
    </BrowserRouter>
  );
};

export default App;
