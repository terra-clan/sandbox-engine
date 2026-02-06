import React, { useEffect, useRef, useState, useCallback } from 'react';
import { Terminal as XTerm } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import '@xterm/xterm/css/xterm.css';

interface TerminalProps {
  sandboxId: string;
  apiToken: string;
  sessionToken?: string;
  wsBaseUrl?: string;
}

const MAX_RECONNECT_ATTEMPTS = 10;
const INITIAL_RECONNECT_DELAY = 1000;
const MAX_RECONNECT_DELAY = 15000;

export const Terminal: React.FC<TerminalProps> = ({
  sandboxId,
  apiToken,
  sessionToken,
  wsBaseUrl = 'wss://api.terra-sandbox.ru'
}) => {
  const terminalRef = useRef<HTMLDivElement>(null);
  const xtermRef = useRef<XTerm | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const fitAddonRef = useRef<FitAddon | null>(null);
  const reconnectAttempts = useRef(0);
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const unmountedRef = useRef(false);
  const [status, setStatus] = useState<'connecting' | 'connected' | 'disconnected' | 'reconnecting'>('connecting');

  const getWsUrl = useCallback(() => {
    return sessionToken
      ? `${wsBaseUrl}/api/v1/ws/session-terminal/${sandboxId}?session_token=${sessionToken}`
      : `${wsBaseUrl}/api/v1/ws/terminal/${sandboxId}?token=${apiToken}`;
  }, [sandboxId, apiToken, sessionToken, wsBaseUrl]);

  const connect = useCallback((term: XTerm) => {
    if (unmountedRef.current) return;

    const wsUrl = getWsUrl();
    const ws = new WebSocket(wsUrl);
    wsRef.current = ws;

    ws.onopen = () => {
      if (unmountedRef.current) { ws.close(); return; }
      reconnectAttempts.current = 0;
      setStatus('connected');

      if (reconnectAttempts.current === 0) {
        term.writeln('\x1b[1;32m Connected to Terra Sandbox\x1b[0m');
        term.writeln('');
      } else {
        term.writeln('\r\n\x1b[1;32m Reconnected\x1b[0m');
      }

      // Send terminal size
      ws.send(JSON.stringify({
        type: 'resize',
        cols: term.cols,
        rows: term.rows
      }));
    };

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data);
        switch (msg.type) {
          case 'output':
            term.write(msg.data);
            break;
          case 'connected':
            console.log('Session connected:', msg.data);
            break;
          case 'exit':
            term.writeln(`\r\n\x1b[1;31m Process exited with code ${msg.code}\x1b[0m`);
            break;
          case 'error':
            term.writeln(`\r\n\x1b[1;31m Error: ${msg.data}\x1b[0m`);
            break;
        }
      } catch {
        term.write(event.data);
      }
    };

    ws.onclose = (event) => {
      if (unmountedRef.current) return;

      // Don't reconnect on normal close (1000) or if explicitly closed
      if (event.code === 1000) {
        setStatus('disconnected');
        term.writeln('\r\n\x1b[1;31m Disconnected\x1b[0m');
        return;
      }

      if (reconnectAttempts.current < MAX_RECONNECT_ATTEMPTS) {
        const delay = Math.min(
          INITIAL_RECONNECT_DELAY * Math.pow(2, reconnectAttempts.current),
          MAX_RECONNECT_DELAY
        );
        reconnectAttempts.current++;
        setStatus('reconnecting');
        term.writeln(`\r\n\x1b[1;33m Connection lost. Reconnecting in ${Math.round(delay / 1000)}s... (${reconnectAttempts.current}/${MAX_RECONNECT_ATTEMPTS})\x1b[0m`);

        reconnectTimer.current = setTimeout(() => {
          if (!unmountedRef.current) {
            connect(term);
          }
        }, delay);
      } else {
        setStatus('disconnected');
        term.writeln('\r\n\x1b[1;31m Disconnected. Max reconnect attempts reached. Refresh the page to try again.\x1b[0m');
      }
    };

    ws.onerror = (error) => {
      console.error('WebSocket error:', error);
    };
  }, [getWsUrl]);

  useEffect(() => {
    if (!terminalRef.current) return;
    unmountedRef.current = false;

    // Create terminal with dark theme
    const term = new XTerm({
      cursorBlink: true,
      fontSize: 14,
      fontFamily: '"JetBrains Mono", "Fira Code", "Consolas", monospace',
      theme: {
        background: '#0f172a',
        foreground: '#e2e8f0',
        cursor: '#06b6d4',
        cursorAccent: '#0f172a',
        selectionBackground: '#334155',
        black: '#1e293b',
        red: '#f87171',
        green: '#4ade80',
        yellow: '#fbbf24',
        blue: '#60a5fa',
        magenta: '#c084fc',
        cyan: '#22d3ee',
        white: '#f1f5f9',
        brightBlack: '#475569',
        brightRed: '#fca5a5',
        brightGreen: '#86efac',
        brightYellow: '#fde047',
        brightBlue: '#93c5fd',
        brightMagenta: '#d8b4fe',
        brightCyan: '#67e8f9',
        brightWhite: '#ffffff',
      },
      allowProposedApi: true,
    });

    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);

    term.open(terminalRef.current);
    fitAddon.fit();

    xtermRef.current = term;
    fitAddonRef.current = fitAddon;

    // Connect WebSocket
    connect(term);

    // Send input to server
    term.onData((data) => {
      const ws = wsRef.current;
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'input', data }));
      }
    });

    // Handle paste via keyboard
    term.attachCustomKeyEventHandler((event) => {
      // Handle Ctrl+V / Cmd+V for paste
      if ((event.ctrlKey || event.metaKey) && event.key === 'v' && event.type === 'keydown') {
        navigator.clipboard.readText().then((text) => {
          const ws = wsRef.current;
          if (ws && ws.readyState === WebSocket.OPEN && text) {
            ws.send(JSON.stringify({ type: 'input', data: text }));
          }
        }).catch(err => {
          console.error('Failed to read clipboard:', err);
        });
        return false;
      }
      // Handle Ctrl+C / Cmd+C for copy
      if ((event.ctrlKey || event.metaKey) && event.key === 'c' && event.type === 'keydown') {
        const selection = term.getSelection();
        if (selection) {
          navigator.clipboard.writeText(selection);
          return false;
        }
      }
      return true;
    });

    // Resize handler
    const handleResize = () => {
      fitAddon.fit();
      const ws = wsRef.current;
      if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({
          type: 'resize',
          cols: term.cols,
          rows: term.rows
        }));
      }
    };

    window.addEventListener('resize', handleResize);

    const resizeObserver = new ResizeObserver(() => {
      handleResize();
    });
    resizeObserver.observe(terminalRef.current);

    terminalRef.current.addEventListener('click', () => {
      term.focus();
    });

    setTimeout(() => term.focus(), 100);

    return () => {
      unmountedRef.current = true;
      if (reconnectTimer.current) {
        clearTimeout(reconnectTimer.current);
      }
      window.removeEventListener('resize', handleResize);
      resizeObserver.disconnect();
      if (wsRef.current) {
        wsRef.current.close(1000);
      }
      term.dispose();
    };
  }, [sandboxId, apiToken, sessionToken, wsBaseUrl, connect]);

  return (
    <div className="h-full flex flex-col bg-slate-900 rounded-lg overflow-hidden border border-slate-700">
      {/* Status bar */}
      <div className="flex items-center justify-between px-3 py-1.5 bg-slate-800 border-b border-slate-700">
        <div className="flex items-center gap-2">
          <div className={`w-2 h-2 rounded-full ${
            status === 'connected' ? 'bg-green-500' :
            status === 'connecting' ? 'bg-yellow-500 animate-pulse' :
            status === 'reconnecting' ? 'bg-yellow-500 animate-pulse' :
            'bg-red-500'
          }`} />
          <span className="text-xs text-slate-400 font-mono">
            {status === 'connected' ? 'Terminal' :
             status === 'connecting' ? 'Connecting...' :
             status === 'reconnecting' ? 'Reconnecting...' :
             'Disconnected'}
          </span>
        </div>
        <span className="text-xs text-slate-500 font-mono">
          sandbox: {sandboxId.slice(0, 8)}...
        </span>
      </div>

      {/* Terminal */}
      <div
        ref={terminalRef}
        className="flex-1 p-2"
        style={{ minHeight: 0 }}
      />
    </div>
  );
};
