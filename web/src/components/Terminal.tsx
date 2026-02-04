import React, { useEffect, useRef, useState } from 'react';
import { Terminal as XTerm } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import '@xterm/xterm/css/xterm.css';

interface TerminalProps {
  sandboxId: string;
  apiToken: string;
  wsBaseUrl?: string;
}

export const Terminal: React.FC<TerminalProps> = ({
  sandboxId,
  apiToken,
  wsBaseUrl = 'wss://api.terra-sandbox.ru'
}) => {
  const terminalRef = useRef<HTMLDivElement>(null);
  const xtermRef = useRef<XTerm | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const fitAddonRef = useRef<FitAddon | null>(null);
  const [status, setStatus] = useState<'connecting' | 'connected' | 'disconnected'>('connecting');

  useEffect(() => {
    if (!terminalRef.current) return;

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

    // Connect to WebSocket
    const wsUrl = `${wsBaseUrl}/api/v1/ws/terminal/${sandboxId}?token=${apiToken}`;
    const ws = new WebSocket(wsUrl);
    wsRef.current = ws;

    ws.onopen = () => {
      setStatus('connected');
      term.writeln('\x1b[1;32m Connected to Terra Sandbox\x1b[0m');
      term.writeln('');

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

    ws.onclose = () => {
      setStatus('disconnected');
      term.writeln('\r\n\x1b[1;31m Disconnected\x1b[0m');
    };

    ws.onerror = (error) => {
      console.error('WebSocket error:', error);
      term.writeln('\r\n\x1b[1;31m Connection error\x1b[0m');
    };

    // Send input to server
    term.onData((data) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: 'input', data }));
      }
    });

    // Handle paste via keyboard
    term.attachCustomKeyEventHandler((event) => {
      // Handle Ctrl+V / Cmd+V for paste
      if ((event.ctrlKey || event.metaKey) && event.key === 'v' && event.type === 'keydown') {
        navigator.clipboard.readText().then((text) => {
          if (ws.readyState === WebSocket.OPEN && text) {
            ws.send(JSON.stringify({ type: 'input', data: text }));
          }
        }).catch(err => {
          console.error('Failed to read clipboard:', err);
        });
        return false; // Prevent default
      }
      // Handle Ctrl+C / Cmd+C for copy
      if ((event.ctrlKey || event.metaKey) && event.key === 'c' && event.type === 'keydown') {
        const selection = term.getSelection();
        if (selection) {
          navigator.clipboard.writeText(selection);
          return false; // Prevent default
        }
        // If no selection, send Ctrl+C to terminal
      }
      return true;
    });

    // Resize handler
    const handleResize = () => {
      fitAddon.fit();
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({
          type: 'resize',
          cols: term.cols,
          rows: term.rows
        }));
      }
    };

    window.addEventListener('resize', handleResize);

    // ResizeObserver for container
    const resizeObserver = new ResizeObserver(() => {
      handleResize();
    });
    resizeObserver.observe(terminalRef.current);

    // Focus terminal on click
    terminalRef.current.addEventListener('click', () => {
      term.focus();
    });

    // Focus terminal initially
    setTimeout(() => term.focus(), 100);

    return () => {
      window.removeEventListener('resize', handleResize);
      resizeObserver.disconnect();
      ws.close();
      term.dispose();
    };
  }, [sandboxId, apiToken, wsBaseUrl]);

  return (
    <div className="h-full flex flex-col bg-slate-900 rounded-lg overflow-hidden border border-slate-700">
      {/* Status bar */}
      <div className="flex items-center justify-between px-3 py-1.5 bg-slate-800 border-b border-slate-700">
        <div className="flex items-center gap-2">
          <div className={`w-2 h-2 rounded-full ${
            status === 'connected' ? 'bg-green-500' :
            status === 'connecting' ? 'bg-yellow-500 animate-pulse' :
            'bg-red-500'
          }`} />
          <span className="text-xs text-slate-400 font-mono">
            {status === 'connected' ? 'Terminal' :
             status === 'connecting' ? 'Connecting...' :
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
