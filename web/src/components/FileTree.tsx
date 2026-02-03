import React, { useState } from 'react';
import { ChevronRight, ChevronDown, File, Folder, FolderOpen } from 'lucide-react';
import { FileNode } from '../types';

interface FileTreeProps {
  files: FileNode[];
  onFileClick: (file: FileNode) => void;
}

interface FileTreeItemProps {
  node: FileNode;
  depth: number;
  onFileClick: (file: FileNode) => void;
}

const getFileIcon = (filename: string) => {
  const ext = filename.split('.').pop()?.toLowerCase();
  const iconClasses = "w-4 h-4 shrink-0";

  switch (ext) {
    case 'ts':
    case 'tsx':
      return <File className={`${iconClasses} text-blue-400`} />;
    case 'js':
    case 'jsx':
      return <File className={`${iconClasses} text-yellow-400`} />;
    case 'json':
      return <File className={`${iconClasses} text-orange-400`} />;
    case 'md':
      return <File className={`${iconClasses} text-slate-400`} />;
    case 'css':
    case 'scss':
      return <File className={`${iconClasses} text-pink-400`} />;
    case 'py':
      return <File className={`${iconClasses} text-green-400`} />;
    case 'go':
      return <File className={`${iconClasses} text-cyan-400`} />;
    case 'rs':
      return <File className={`${iconClasses} text-orange-500`} />;
    case 'yml':
    case 'yaml':
      return <File className={`${iconClasses} text-purple-400`} />;
    case 'sh':
    case 'bash':
      return <File className={`${iconClasses} text-green-500`} />;
    default:
      return <File className={`${iconClasses} text-slate-500`} />;
  }
};

const FileTreeItem: React.FC<FileTreeItemProps> = ({ node, depth, onFileClick }) => {
  const [isExpanded, setIsExpanded] = useState(depth < 2);

  const handleClick = () => {
    if (node.type === 'folder') {
      setIsExpanded(!isExpanded);
    } else {
      onFileClick(node);
    }
  };

  return (
    <div>
      <div
        onClick={handleClick}
        className="flex items-center gap-1.5 px-2 py-1 rounded cursor-pointer text-sm hover:bg-slate-700/50 transition-colors"
        style={{ paddingLeft: `${depth * 12 + 8}px` }}
      >
        {node.type === 'folder' ? (
          <>
            {isExpanded ? (
              <ChevronDown className="w-4 h-4 text-slate-500 shrink-0" />
            ) : (
              <ChevronRight className="w-4 h-4 text-slate-500 shrink-0" />
            )}
            {isExpanded ? (
              <FolderOpen className="w-4 h-4 text-cyan-400 shrink-0" />
            ) : (
              <Folder className="w-4 h-4 text-cyan-500 shrink-0" />
            )}
          </>
        ) : (
          <>
            <span className="w-4" />
            {getFileIcon(node.name)}
          </>
        )}
        <span className="truncate text-slate-300">{node.name}</span>
      </div>

      {node.type === 'folder' && isExpanded && node.children && (
        <div>
          {node.children.map((child) => (
            <FileTreeItem
              key={child.id}
              node={child}
              depth={depth + 1}
              onFileClick={onFileClick}
            />
          ))}
        </div>
      )}
    </div>
  );
};

export const FileTree: React.FC<FileTreeProps> = ({ files, onFileClick }) => {
  return (
    <div className="text-sm">
      {files.map((file) => (
        <FileTreeItem
          key={file.id}
          node={file}
          depth={0}
          onFileClick={onFileClick}
        />
      ))}
    </div>
  );
};
