import React, { useState } from 'react';
import { FolderTree, Server, ChevronLeft, ChevronRight, FileText, X } from 'lucide-react';
import { motion, AnimatePresence } from 'framer-motion';
import Markdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
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
  sessionToken?: string;
  apiBaseUrl: string;
  wsBaseUrl: string;
  loading: boolean;
  error: string | null;
  taskDescription?: string;
}

export const Workspace: React.FC<WorkspaceProps> = ({
  sandbox,
  sandboxId,
  apiToken,
  sessionToken,
  wsBaseUrl,
  loading,
  error,
  taskDescription,
}) => {
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [activeTab, setActiveTab] = useState<'files' | 'services'>('files');
  const [selectedFile, setSelectedFile] = useState<FileNode | null>(null);
  const [showTaskModal, setShowTaskModal] = useState(false);

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
                {taskDescription && (
                  <button
                    onClick={() => setShowTaskModal(true)}
                    className="px-2 py-1 rounded text-xs transition-colors text-slate-400 hover:text-cyan-400 hover:bg-slate-700"
                    title="Техническое задание"
                  >
                    <FileText size={14} />
                  </button>
                )}
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
              sessionToken={sessionToken}
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

      {/* Task description modal */}
      <AnimatePresence>
        {showTaskModal && taskDescription && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            className="fixed inset-0 z-50 flex items-center justify-center p-4 md:p-8"
            onClick={() => setShowTaskModal(false)}
          >
            <div className="absolute inset-0 bg-black/70 backdrop-blur-sm" />

            <motion.div
              initial={{ opacity: 0, scale: 0.95, y: 20 }}
              animate={{ opacity: 1, scale: 1, y: 0 }}
              exit={{ opacity: 0, scale: 0.95, y: 20 }}
              transition={{ duration: 0.2 }}
              onClick={(e) => e.stopPropagation()}
              className="relative w-full max-w-3xl max-h-[85vh] bg-slate-900 rounded-2xl border border-slate-700 shadow-2xl overflow-hidden flex flex-col"
            >
              {/* Header */}
              <div className="shrink-0 flex items-center justify-between px-5 py-3 border-b border-slate-700 bg-slate-800">
                <div className="flex items-center gap-2">
                  <FileText size={16} className="text-cyan-400" />
                  <span className="text-sm font-medium text-slate-200">Техническое задание</span>
                </div>
                <button
                  onClick={() => setShowTaskModal(false)}
                  className="p-1.5 rounded-lg hover:bg-slate-700 text-slate-400 hover:text-white transition-colors"
                >
                  <X size={20} />
                </button>
              </div>

              {/* Content */}
              <div className="flex-1 overflow-y-auto p-6">
                <div className="prose prose-invert prose-sm max-w-none
                  prose-headings:text-slate-200 prose-p:text-slate-300
                  prose-strong:text-white prose-code:text-cyan-400
                  prose-code:bg-slate-700/50 prose-code:px-1 prose-code:py-0.5 prose-code:rounded
                  prose-pre:bg-slate-900 prose-pre:border prose-pre:border-slate-700
                  prose-li:text-slate-300 prose-a:text-cyan-400
                  prose-table:text-slate-300 prose-th:text-slate-200 prose-td:border-slate-700 prose-th:border-slate-700">
                  <Markdown remarkPlugins={[remarkGfm]}>{taskDescription}</Markdown>
                </div>
              </div>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
};
