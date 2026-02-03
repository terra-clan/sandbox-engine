import React, { useState } from 'react';
import { FolderTree, Server, ChevronLeft, ChevronRight } from 'lucide-react';
import { Terminal } from '../components/Terminal';
import { FileTree } from '../components/FileTree';
import { FileViewerModal } from '../components/FileViewerModal';
import { StatusBar } from '../components/StatusBar';
import { ServicesList } from '../components/ServicesList';
import { SandboxInfo, FileNode } from '../types';

interface WorkspaceProps {
  sandbox: SandboxInfo | null;
  sandboxId: string;
  apiToken: string;
  apiBaseUrl: string;
  wsBaseUrl: string;
  loading: boolean;
  error: string | null;
}

export const Workspace: React.FC<WorkspaceProps> = ({
  sandbox,
  sandboxId,
  apiToken,
  wsBaseUrl,
  loading,
  error
}) => {
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [activeTab, setActiveTab] = useState<'files' | 'services'>('files');
  const [selectedFile, setSelectedFile] = useState<FileNode | null>(null);

  // Mock files for now - will be populated from API
  const [files] = useState<FileNode[]>([
    {
      id: '1',
      name: 'workspace',
      type: 'folder',
      children: [
        {
          id: '2',
          name: 'src',
          type: 'folder',
          children: [
            { id: '3', name: 'main.go', type: 'file', language: 'go' },
            { id: '4', name: 'handler.go', type: 'file', language: 'go' },
          ]
        },
        { id: '5', name: 'go.mod', type: 'file', language: 'go' },
        { id: '6', name: 'README.md', type: 'file', language: 'markdown' },
      ]
    }
  ]);

  const handleFileClick = (file: FileNode) => {
    if (file.type === 'file') {
      // In future, fetch file content from API
      setSelectedFile({
        ...file,
        content: `// Content of ${file.name}\n// This will be fetched from the API`
      });
    }
  };

  return (
    <div className="h-screen flex flex-col bg-slate-900">
      {/* Status Bar */}
      <StatusBar sandbox={sandbox} loading={loading} error={error} />

      {/* Main content */}
      <div className="flex-1 flex overflow-hidden">
        {/* Sidebar */}
        <div
          className={`bg-slate-800 border-r border-slate-700 flex flex-col transition-all duration-200 ${
            sidebarCollapsed ? 'w-10' : 'w-64'
          }`}
        >
          {/* Sidebar header */}
          <div className="h-10 flex items-center justify-between px-2 border-b border-slate-700">
            {!sidebarCollapsed && (
              <div className="flex gap-1">
                <button
                  onClick={() => setActiveTab('files')}
                  className={`px-2 py-1 rounded text-xs transition-colors ${
                    activeTab === 'files'
                      ? 'bg-slate-700 text-cyan-400'
                      : 'text-slate-400 hover:text-white'
                  }`}
                >
                  <FolderTree size={14} />
                </button>
                <button
                  onClick={() => setActiveTab('services')}
                  className={`px-2 py-1 rounded text-xs transition-colors ${
                    activeTab === 'services'
                      ? 'bg-slate-700 text-cyan-400'
                      : 'text-slate-400 hover:text-white'
                  }`}
                >
                  <Server size={14} />
                </button>
              </div>
            )}
            <button
              onClick={() => setSidebarCollapsed(!sidebarCollapsed)}
              className="p-1 rounded hover:bg-slate-700 text-slate-400 hover:text-white transition-colors"
            >
              {sidebarCollapsed ? <ChevronRight size={16} /> : <ChevronLeft size={16} />}
            </button>
          </div>

          {/* Sidebar content */}
          {!sidebarCollapsed && (
            <div className="flex-1 overflow-auto p-2">
              {activeTab === 'files' ? (
                <FileTree files={files} onFileClick={handleFileClick} />
              ) : (
                <ServicesList services={sandbox?.services || []} />
              )}
            </div>
          )}
        </div>

        {/* Terminal */}
        <div className="flex-1 p-2 min-w-0">
          {sandbox?.status === 'running' ? (
            <Terminal
              sandboxId={sandboxId}
              apiToken={apiToken}
              wsBaseUrl={wsBaseUrl}
            />
          ) : (
            <div className="h-full flex items-center justify-center bg-slate-800 rounded-lg border border-slate-700">
              <div className="text-center">
                {loading ? (
                  <>
                    <div className="w-8 h-8 border-2 border-cyan-500 border-t-transparent rounded-full animate-spin mx-auto mb-4" />
                    <p className="text-slate-400">Initializing sandbox...</p>
                  </>
                ) : error ? (
                  <>
                    <p className="text-red-400 mb-2">Failed to load sandbox</p>
                    <p className="text-slate-500 text-sm">{error}</p>
                  </>
                ) : (
                  <p className="text-slate-400">Waiting for sandbox to start...</p>
                )}
              </div>
            </div>
          )}
        </div>
      </div>

      {/* File Viewer Modal */}
      <FileViewerModal file={selectedFile} onClose={() => setSelectedFile(null)} />
    </div>
  );
};
