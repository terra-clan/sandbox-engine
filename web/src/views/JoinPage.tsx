import React, { useState, useEffect, useCallback } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { Play, Loader2, Check, Terminal, Server, Clock } from 'lucide-react';
import { Workspace } from './Workspace';
import { SandboxInfo } from '../types';

interface SessionInfo {
  status: 'ready' | 'provisioning' | 'active' | 'expired' | 'failed';
  template?: {
    name: string;
    description: string;
    language?: string;
  };
  metadata?: Record<string, string>;
  sandbox?: {
    id: string;
    status: string;
    endpoints?: Record<string, string>;
    expires_at?: string;
  };
}

interface JoinPageProps {
  token: string;
  apiBaseUrl: string;
  wsBaseUrl: string;
}

const provisioningSteps = [
  { label: 'Preparing environment', icon: Server },
  { label: 'Starting services', icon: Terminal },
  { label: 'Connecting terminal', icon: Play },
];

export const JoinPage: React.FC<JoinPageProps> = ({ token, apiBaseUrl, wsBaseUrl }) => {
  const [session, setSession] = useState<SessionInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activating, setActivating] = useState(false);
  const [activeStep, setActiveStep] = useState(0);
  const [showWorkspace, setShowWorkspace] = useState(false);

  const fetchSession = useCallback(async () => {
    try {
      const res = await fetch(`${apiBaseUrl}/api/v1/join/${token}`);
      if (!res.ok) {
        if (res.status === 404) throw new Error('Session not found or expired');
        throw new Error(`HTTP ${res.status}`);
      }
      const data = await res.json();
      if (data.success) {
        setSession(data.data);
        setError(null);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load session');
    } finally {
      setLoading(false);
    }
  }, [apiBaseUrl, token]);

  // Initial fetch
  useEffect(() => {
    fetchSession();
  }, [fetchSession]);

  // Poll during provisioning
  useEffect(() => {
    if (session?.status !== 'provisioning') return;

    const interval = setInterval(fetchSession, 2000);
    return () => clearInterval(interval);
  }, [session?.status, fetchSession]);

  // Animate provisioning steps
  useEffect(() => {
    if (session?.status !== 'provisioning') return;

    const interval = setInterval(() => {
      setActiveStep(prev => (prev < provisioningSteps.length - 1 ? prev + 1 : prev));
    }, 3000);
    return () => clearInterval(interval);
  }, [session?.status]);

  // Transition to workspace when active
  useEffect(() => {
    if (session?.status === 'active' && session.sandbox?.id) {
      const timer = setTimeout(() => setShowWorkspace(true), 1200);
      return () => clearTimeout(timer);
    }
  }, [session?.status, session?.sandbox?.id]);

  const handleActivate = async () => {
    setActivating(true);
    try {
      const res = await fetch(`${apiBaseUrl}/api/v1/join/${token}/activate`, {
        method: 'POST',
      });
      if (!res.ok) {
        const data = await res.json().catch(() => null);
        throw new Error(data?.error?.message || `HTTP ${res.status}`);
      }
      // Refresh session state
      await fetchSession();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to activate');
      setActivating(false);
    }
  };

  // Workspace mode — session is active, show full workspace
  if (showWorkspace && session?.sandbox) {
    const sandboxInfo: SandboxInfo = {
      id: session.sandbox.id,
      templateId: session.template?.name || '',
      status: 'running',
      createdAt: '',
      expiresAt: session.sandbox.expires_at || '',
      services: [],
      workDir: '/workspace',
    };

    return (
      <Workspace
        sandbox={sandboxInfo}
        sandboxId={session.sandbox.id}
        apiToken=""
        sessionToken={token}
        apiBaseUrl={apiBaseUrl}
        wsBaseUrl={wsBaseUrl}
        loading={false}
        error={null}
      />
    );
  }

  // Loading state
  if (loading) {
    return (
      <div className="h-screen bg-slate-900 flex items-center justify-center">
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          className="text-center"
        >
          <Loader2 className="w-8 h-8 text-cyan-500 animate-spin mx-auto mb-4" />
          <p className="text-slate-400">Loading session...</p>
        </motion.div>
      </div>
    );
  }

  // Error state
  if (error && !session) {
    return (
      <div className="h-screen bg-slate-900 flex items-center justify-center">
        <div className="max-w-md text-center px-4">
          <h1 className="text-2xl font-bold text-white mb-4">Terra Sandbox</h1>
          <div className="bg-slate-800 rounded-lg p-6 border border-red-500/30">
            <p className="text-red-400">{error}</p>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="h-screen bg-slate-900 flex items-center justify-center overflow-hidden">
      <AnimatePresence mode="wait">
        {/* Ready — Welcome screen */}
        {session?.status === 'ready' && (
          <motion.div
            key="welcome"
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -20 }}
            transition={{ duration: 0.4 }}
            className="max-w-lg w-full mx-4"
          >
            <div className="text-center mb-8">
              <motion.div
                initial={{ scale: 0.8 }}
                animate={{ scale: 1 }}
                transition={{ delay: 0.1, type: 'spring', stiffness: 200 }}
              >
                <h1 className="text-4xl font-bold text-white mb-2">
                  Terra <span className="text-cyan-400">Sandbox</span>
                </h1>
              </motion.div>
              <p className="text-slate-400">Development environment</p>
            </div>

            <div className="bg-slate-800/50 backdrop-blur rounded-xl p-6 border border-slate-700 space-y-4">
              {/* Template info */}
              {session.template && (
                <div>
                  <div className="text-xs text-slate-500 uppercase tracking-wider mb-1">Environment</div>
                  <div className="text-white font-medium">{session.template.name}</div>
                  {session.template.description && (
                    <div className="text-slate-400 text-sm mt-1">{session.template.description}</div>
                  )}
                </div>
              )}

              {/* Metadata */}
              {session.metadata && Object.keys(session.metadata).length > 0 && (
                <div className="border-t border-slate-700 pt-4 space-y-2">
                  {Object.entries(session.metadata).map(([key, value]) => (
                    <div key={key} className="flex justify-between text-sm">
                      <span className="text-slate-500">{key.replace(/_/g, ' ')}</span>
                      <span className="text-slate-300">{value}</span>
                    </div>
                  ))}
                </div>
              )}

              {/* Error banner */}
              {error && (
                <div className="bg-red-500/10 border border-red-500/30 rounded-lg p-3">
                  <p className="text-red-400 text-sm">{error}</p>
                </div>
              )}

              {/* Start button */}
              <motion.button
                whileHover={{ scale: 1.02 }}
                whileTap={{ scale: 0.98 }}
                onClick={handleActivate}
                disabled={activating}
                className="w-full py-3 px-4 bg-cyan-600 hover:bg-cyan-500 disabled:bg-slate-600
                  text-white font-semibold rounded-lg transition-colors flex items-center justify-center gap-2"
              >
                {activating ? (
                  <>
                    <Loader2 className="w-5 h-5 animate-spin" />
                    Starting...
                  </>
                ) : (
                  <>
                    <Play className="w-5 h-5" />
                    Start Environment
                  </>
                )}
              </motion.button>
            </div>
          </motion.div>
        )}

        {/* Provisioning — loading steps */}
        {session?.status === 'provisioning' && (
          <motion.div
            key="provisioning"
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -20 }}
            transition={{ duration: 0.4 }}
            className="max-w-md w-full mx-4 text-center"
          >
            <h2 className="text-2xl font-bold text-white mb-2">Setting up your environment</h2>
            <p className="text-slate-400 mb-8">This usually takes 10-30 seconds</p>

            <div className="space-y-4 mb-8">
              {provisioningSteps.map((step, i) => {
                const Icon = step.icon;
                const isActive = i === activeStep;
                const isDone = i < activeStep;

                return (
                  <motion.div
                    key={step.label}
                    initial={{ opacity: 0, x: -20 }}
                    animate={{ opacity: 1, x: 0 }}
                    transition={{ delay: i * 0.15 }}
                    className={`flex items-center gap-3 p-4 rounded-lg border transition-all ${
                      isDone
                        ? 'bg-emerald-500/10 border-emerald-500/30'
                        : isActive
                        ? 'bg-cyan-500/10 border-cyan-500/30'
                        : 'bg-slate-800/50 border-slate-700'
                    }`}
                  >
                    <div className={`w-8 h-8 rounded-full flex items-center justify-center ${
                      isDone
                        ? 'bg-emerald-500/20'
                        : isActive
                        ? 'bg-cyan-500/20'
                        : 'bg-slate-700'
                    }`}>
                      {isDone ? (
                        <Check className="w-4 h-4 text-emerald-400" />
                      ) : isActive ? (
                        <Loader2 className="w-4 h-4 text-cyan-400 animate-spin" />
                      ) : (
                        <Icon className="w-4 h-4 text-slate-500" />
                      )}
                    </div>
                    <span className={`font-medium ${
                      isDone ? 'text-emerald-400' : isActive ? 'text-cyan-400' : 'text-slate-500'
                    }`}>
                      {step.label}
                    </span>
                  </motion.div>
                );
              })}
            </div>

            {/* Progress bar */}
            <div className="w-full bg-slate-800 rounded-full h-1.5 overflow-hidden">
              <motion.div
                className="h-full bg-gradient-to-r from-cyan-500 to-emerald-500 rounded-full"
                initial={{ width: '5%' }}
                animate={{ width: `${Math.min(((activeStep + 1) / provisioningSteps.length) * 90 + 10, 95)}%` }}
                transition={{ duration: 0.5 }}
              />
            </div>
          </motion.div>
        )}

        {/* Active — transition animation */}
        {session?.status === 'active' && !showWorkspace && (
          <motion.div
            key="ready"
            initial={{ opacity: 0, scale: 0.9 }}
            animate={{ opacity: 1, scale: 1 }}
            exit={{ opacity: 0, scale: 1.1 }}
            transition={{ duration: 0.3 }}
            className="text-center"
          >
            <motion.div
              initial={{ scale: 0 }}
              animate={{ scale: 1 }}
              transition={{ type: 'spring', stiffness: 300, damping: 15 }}
              className="w-16 h-16 bg-emerald-500/20 rounded-full flex items-center justify-center mx-auto mb-4"
            >
              <Check className="w-8 h-8 text-emerald-400" />
            </motion.div>
            <h2 className="text-2xl font-bold text-white">Ready!</h2>
            <p className="text-slate-400 mt-2">Launching workspace...</p>
          </motion.div>
        )}

        {/* Failed */}
        {session?.status === 'failed' && (
          <motion.div
            key="failed"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            className="max-w-md w-full mx-4 text-center"
          >
            <div className="bg-red-500/10 border border-red-500/30 rounded-xl p-6">
              <h2 className="text-xl font-bold text-red-400 mb-2">Environment Failed</h2>
              <p className="text-slate-400 mb-4">
                {session.metadata?.error || 'Something went wrong while setting up your environment.'}
              </p>
              <button
                onClick={() => window.location.reload()}
                className="px-4 py-2 bg-slate-700 hover:bg-slate-600 text-white rounded-lg transition-colors"
              >
                Try Again
              </button>
            </div>
          </motion.div>
        )}

        {/* Expired */}
        {session?.status === 'expired' && (
          <motion.div
            key="expired"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            className="max-w-md w-full mx-4 text-center"
          >
            <div className="bg-slate-800 border border-slate-700 rounded-xl p-6">
              <Clock className="w-12 h-12 text-slate-500 mx-auto mb-4" />
              <h2 className="text-xl font-bold text-white mb-2">Session Expired</h2>
              <p className="text-slate-400">This session has reached its time limit.</p>
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
};
