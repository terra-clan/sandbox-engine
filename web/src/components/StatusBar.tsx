import React, { useState, useEffect } from 'react';
import { Clock, Activity, Server, AlertTriangle } from 'lucide-react';
import { SandboxInfo, getTimeRemaining, formatTime } from '../types';

interface StatusBarProps {
  sandbox: SandboxInfo | null;
  loading: boolean;
  error: string | null;
}

export const StatusBar: React.FC<StatusBarProps> = ({ sandbox, loading, error }) => {
  const [timeLeft, setTimeLeft] = useState({ minutes: 0, seconds: 0, expired: false });

  useEffect(() => {
    if (!sandbox?.expiresAt) return;

    const updateTime = () => {
      setTimeLeft(getTimeRemaining(sandbox.expiresAt));
    };

    updateTime();
    const interval = setInterval(updateTime, 1000);

    return () => clearInterval(interval);
  }, [sandbox?.expiresAt]);

  const getStatusColor = () => {
    if (error) return 'bg-red-500';
    if (loading) return 'bg-yellow-500 animate-pulse';
    if (!sandbox) return 'bg-slate-500';

    switch (sandbox.status) {
      case 'running': return 'bg-green-500';
      case 'creating': return 'bg-yellow-500 animate-pulse';
      case 'stopped': return 'bg-slate-500';
      case 'error': return 'bg-red-500';
      default: return 'bg-slate-500';
    }
  };

  const getStatusText = () => {
    if (error) return 'Error';
    if (loading) return 'Loading...';
    if (!sandbox) return 'No Sandbox';

    switch (sandbox.status) {
      case 'running': return 'Running';
      case 'creating': return 'Creating...';
      case 'stopped': return 'Stopped';
      case 'error': return 'Error';
      default: return sandbox.status;
    }
  };

  const isTimeCritical = timeLeft.minutes < 5 && !timeLeft.expired;

  return (
    <div className="h-10 bg-slate-800 border-b border-slate-700 flex items-center justify-between px-4">
      {/* Left section */}
      <div className="flex items-center gap-4">
        {/* Status indicator */}
        <div className="flex items-center gap-2">
          <div className={`w-2 h-2 rounded-full ${getStatusColor()}`} />
          <span className="text-sm text-slate-300">{getStatusText()}</span>
        </div>

        {/* Sandbox ID */}
        {sandbox && (
          <div className="flex items-center gap-2 text-slate-400">
            <Server size={14} />
            <span className="text-xs font-mono">{sandbox.id.slice(0, 12)}...</span>
          </div>
        )}

        {/* Template */}
        {sandbox && (
          <div className="flex items-center gap-2 text-slate-400">
            <Activity size={14} />
            <span className="text-xs">{sandbox.templateId}</span>
          </div>
        )}
      </div>

      {/* Right section */}
      <div className="flex items-center gap-4">
        {/* Error message */}
        {error && (
          <div className="flex items-center gap-2 text-red-400">
            <AlertTriangle size={14} />
            <span className="text-xs">{error}</span>
          </div>
        )}

        {/* Time remaining */}
        {sandbox && !timeLeft.expired && (
          <div className={`flex items-center gap-2 ${isTimeCritical ? 'text-red-400' : 'text-slate-400'}`}>
            <Clock size={14} className={isTimeCritical ? 'animate-pulse' : ''} />
            <span className={`text-sm font-mono ${isTimeCritical ? 'font-bold' : ''}`}>
              {formatTime(timeLeft.minutes, timeLeft.seconds)}
            </span>
          </div>
        )}

        {/* Expired warning */}
        {timeLeft.expired && sandbox && (
          <div className="flex items-center gap-2 text-red-400">
            <AlertTriangle size={14} />
            <span className="text-sm font-bold">Session Expired</span>
          </div>
        )}
      </div>
    </div>
  );
};
