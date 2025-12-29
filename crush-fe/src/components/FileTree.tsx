import React, { useState } from 'react';
import { ChevronRight, ChevronDown, File, Folder, FolderOpen } from 'lucide-react';
import { type FileNode } from '../types';
import { cn } from '../lib/utils';

interface FileTreeProps {
  data: FileNode[];
  onSelectFile: (file: FileNode) => void;
  selectedFileId?: string;
}

const FileTreeNode = ({ 
  node, 
  depth = 0, 
  onSelect, 
  selectedId 
}: { 
  node: FileNode; 
  depth?: number; 
  onSelect: (node: FileNode) => void;
  selectedId?: string;
}) => {
  const [isOpen, setIsOpen] = useState(false);
  const isFolder = node.type === 'folder';

  const handleToggle = (e: React.MouseEvent) => {
    e.stopPropagation();
    if (isFolder) {
      setIsOpen(!isOpen);
    } else {
      onSelect(node);
    }
  };

  return (
    <div>
      <div
        className={cn(
          "flex items-center py-1 px-2 cursor-pointer hover:bg-white/10 text-sm select-none",
          selectedId === node.id && !isFolder && "bg-blue-600/20 text-blue-400"
        )}
        style={{ paddingLeft: `${depth * 12 + 8}px` }}
        onClick={handleToggle}
      >
        <span className="mr-1 opacity-70">
          {isFolder ? (
            isOpen ? <ChevronDown size={14} /> : <ChevronRight size={14} />
          ) : (
            <span className="w-[14px]" />
          )}
        </span>
        <span className="mr-2 text-blue-400">
          {isFolder ? (
             isOpen ? <FolderOpen size={14} /> : <Folder size={14} />
          ) : (
            <File size={14} className="text-gray-400" />
          )}
        </span>
        <span className="truncate">{node.name}</span>
      </div>
      {isOpen && node.children && (
        <div>
          {node.children.map((child) => (
            <FileTreeNode 
              key={child.id} 
              node={child} 
              depth={depth + 1} 
              onSelect={onSelect}
              selectedId={selectedId}
            />
          ))}
        </div>
      )}
    </div>
  );
};

export const FileTree = ({ data, onSelectFile, selectedFileId }: FileTreeProps) => {
  if (!Array.isArray(data)) {
    console.warn('FileTree received non-array data:', data);
    return (
      <div className="flex flex-col h-full bg-[#1e1e1e] text-gray-500 p-4 text-sm">
        No files to display
      </div>
    );
  }

  return (
    <div className="flex flex-col h-full overflow-y-auto bg-[#1e1e1e] text-gray-300 py-2">
      <div className="px-4 py-2 text-xs font-bold text-gray-500 uppercase tracking-wider">
        Explorer
      </div>
      {data.map((node) => (
        <FileTreeNode 
          key={node.id} 
          node={node} 
          onSelect={onSelectFile}
          selectedId={selectedFileId}
        />
      ))}
    </div>
  );
};

