import React from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { X, File, Copy, Check } from 'lucide-react';
import { FileNode } from '../types';

interface FileViewerModalProps {
  file: FileNode | null;
  onClose: () => void;
}

const getLanguageLabel = (language?: string): string => {
  const labels: Record<string, string> = {
    typescript: 'TypeScript',
    javascript: 'JavaScript',
    json: 'JSON',
    markdown: 'Markdown',
    css: 'CSS',
    html: 'HTML',
    python: 'Python',
    go: 'Go',
    rust: 'Rust',
    yaml: 'YAML',
    shell: 'Shell'
  };
  return labels[language || ''] || language || 'Plain Text';
};

export const FileViewerModal: React.FC<FileViewerModalProps> = ({ file, onClose }) => {
  const [copied, setCopied] = React.useState(false);

  const handleCopy = async () => {
    if (file?.content) {
      await navigator.clipboard.writeText(file.content);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  return (
    <AnimatePresence>
      {file && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          className="fixed inset-0 z-50 flex items-center justify-center p-4 md:p-8"
          onClick={onClose}
        >
          {/* Backdrop */}
          <div className="absolute inset-0 bg-black/70 backdrop-blur-sm" />

          {/* Modal */}
          <motion.div
            initial={{ opacity: 0, scale: 0.95, y: 20 }}
            animate={{ opacity: 1, scale: 1, y: 0 }}
            exit={{ opacity: 0, scale: 0.95, y: 20 }}
            transition={{ duration: 0.2 }}
            onClick={(e) => e.stopPropagation()}
            className="relative w-full max-w-4xl max-h-[85vh] bg-slate-900 rounded-2xl border border-slate-700 shadow-2xl overflow-hidden flex flex-col"
          >
            {/* Header */}
            <div className="shrink-0 flex items-center justify-between px-4 py-3 border-b border-slate-700 bg-slate-800">
              <div className="flex items-center gap-3">
                <File size={16} className="text-cyan-400" />
                <span className="font-mono text-sm text-slate-200">{file.name}</span>
                <span className="px-2 py-0.5 rounded bg-slate-700 text-xs text-slate-400">
                  {getLanguageLabel(file.language)}
                </span>
                <span className="px-2 py-0.5 rounded bg-yellow-900/50 text-xs text-yellow-400">
                  Read Only
                </span>
              </div>

              <div className="flex items-center gap-2">
                <button
                  onClick={handleCopy}
                  className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-slate-700 hover:bg-slate-600 text-slate-300 hover:text-white transition-colors text-sm"
                >
                  {copied ? (
                    <>
                      <Check size={14} className="text-green-400" />
                      <span className="text-green-400">Copied</span>
                    </>
                  ) : (
                    <>
                      <Copy size={14} />
                      <span>Copy</span>
                    </>
                  )}
                </button>

                <button
                  onClick={onClose}
                  className="p-1.5 rounded-lg hover:bg-slate-700 text-slate-400 hover:text-white transition-colors"
                >
                  <X size={20} />
                </button>
              </div>
            </div>

            {/* Content */}
            <div className="flex-1 overflow-auto bg-slate-950">
              <pre className="p-4 font-mono text-sm leading-relaxed">
                <code className="text-slate-300">
                  {file.content?.split('\n').map((line, i) => (
                    <div key={i} className="flex">
                      <span className="select-none w-12 pr-4 text-right text-slate-600">
                        {i + 1}
                      </span>
                      <span className="flex-1">{line || ' '}</span>
                    </div>
                  ))}
                </code>
              </pre>
            </div>
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  );
};
