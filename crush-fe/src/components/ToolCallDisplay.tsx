import React, { useState, useMemo, memo } from 'react';
import CodeMirror, { EditorView, Decoration, RangeSetBuilder } from '@uiw/react-codemirror';
import { javascript } from '@codemirror/lang-javascript';
import { vscodeDark } from '@uiw/codemirror-theme-vscode';
import { Copy, Check, ChevronDown, ChevronUp } from 'lucide-react';
import { type ToolCall, type ToolResult } from '../types';
import { cn } from '../lib/utils';

interface ToolCallDisplayProps {
  toolCall: ToolCall;
  result?: ToolResult;
  onApprove?: (toolCallId: string) => void;
  onDeny?: (toolCallId: string) => void;
  needsPermission?: boolean;
  onFileClick?: (filePath: string) => void;
}

// TUI-style icons
const ICONS = {
  pending: '●',
  success: '✓',
  error: '×',
  working: '◐',
};

// Maximum lines to display in code/output sections
const MAX_DISPLAY_LINES = 10;

// Reusable CodeBlock component with Copy button
const CodeBlock: React.FC<{
  content: string;
  language?: string;
  className?: string;
  maxHeight?: string;
  copyContent?: string;
  extensions?: any[];
}> = ({ content, language = 'javascript', className, maxHeight = '400px', copyContent, extensions = [] }) => {
  const [copied, setCopied] = useState(false);
  const [isExpanded, setIsExpanded] = useState(false);
  
  const lineCount = content.split('\n').length;
  // 5 lines * ~20px = ~100px. Using 120px for safety.
  const collapsedHeight = '120px';
  const shouldCollapse = lineCount > 6;

  const allExtensions = useMemo(() => [
    javascript({ jsx: true, typescript: true }),
    // Removed EditorView.lineWrapping to prevent wrapping
    ...extensions
  ], [extensions]);

  const handleCopy = () => {
    navigator.clipboard.writeText(copyContent || content);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className={cn("bg-[#1e1e1e] relative group", className)}>
      <div className="absolute right-4 top-2 opacity-0 group-hover:opacity-100 transition-opacity z-20">
        <button
          onClick={handleCopy}
          className="p-1.5 bg-gray-700/80 hover:bg-gray-600 rounded text-gray-300 hover:text-white transition-colors backdrop-blur-sm"
          title={copyContent ? "Copy result code" : "Copy code"}
        >
          {copied ? <Check size={14} className="text-green-400" /> : <Copy size={14} />}
        </button>
      </div>
      
      <div className={cn(
          "transition-all duration-200 ease-in-out relative",
          !isExpanded && shouldCollapse ? "overflow-hidden" : ""
      )}
      style={{ maxHeight: !isExpanded && shouldCollapse ? collapsedHeight : 'none' }}
      >
        <CodeMirror
            value={content}
            theme={vscodeDark}
            extensions={allExtensions}
            readOnly={true}
            basicSetup={{ lineNumbers: true, foldGutter: false }}
            className="text-xs"
            height="auto" // Allow CodeMirror to size itself based on content
        />
      </div>

      {shouldCollapse && (
        <div className={cn(
            "absolute bottom-0 left-0 right-0 flex justify-center z-10 pointer-events-none",
            !isExpanded ? "bg-gradient-to-t from-[#1e1e1e] via-[#1e1e1e]/40 to-transparent pt-10 pb-1" : "sticky bottom-0 h-0 flex items-end pb-1 overflow-visible"
        )}>
            <div className={cn("pointer-events-auto transform transition-transform", isExpanded ? "translate-y-1/2" : "")}>
                <button
                onClick={() => setIsExpanded(!isExpanded)}
                className="bg-[#252526] hover:bg-[#2d2d30] text-gray-400 hover:text-gray-200 text-[10px] px-3 py-0.5 rounded-full border border-gray-600/50 shadow-sm flex items-center gap-1 backdrop-blur-sm"
                >
                {isExpanded ? (
                    <><ChevronUp size={10} /> Collapse</>
                ) : (
                    <><ChevronDown size={10} /> Show all {lineCount} lines</>
                )}
                </button>
            </div>
        </div>
      )}
      
      {/* Spacer for button when expanded */}
      {isExpanded && shouldCollapse && <div className="h-4" />}
    </div>
  );
};

// Prettify tool name (matching TUI style)
const prettifyToolName = (name: string): string => {
  const nameMap: Record<string, string> = {
    'bash': 'Bash',
    'view': 'View',
    'edit': 'Edit',
    'multi_edit': 'Multi-Edit',
    'write': 'Write',
    'fetch': 'Fetch',
    'agentic_fetch': 'Agentic Fetch',
    'web_fetch': 'Fetch',
    'glob': 'Glob',
    'grep': 'Grep',
    'ls': 'List',
    'download': 'Download',
    'sourcegraph': 'Sourcegraph',
    'diagnostics': 'Diagnostics',
    'agent': 'Agent',
    'job_output': 'Job: Output',
    'job_kill': 'Job: Kill',
  };
  return nameMap[name] || name.split('_').map(w => 
    w.charAt(0).toUpperCase() + w.slice(1)
  ).join(' ');
};

// Parse and format parameters based on tool type
const formatParams = (name: string, input: string): { main: string; extra: Record<string, string> } => {
  try {
    const params = JSON.parse(input);
    const extra: Record<string, string> = {};
    let main = '';

    switch (name) {
      case 'bash':
        main = params.command?.replace(/\n/g, ' ').replace(/\t/g, '    ') || '';
        if (params.run_in_background) extra.background = 'true';
        break;
      case 'view':
        main = params.file_path || params.filePath || '';
        if (params.limit) extra.limit = String(params.limit);
        if (params.offset) extra.offset = String(params.offset);
        break;
      case 'edit':
      case 'write':
        main = params.file_path || params.filePath || '';
        break;
      case 'multi_edit':
        main = params.file_path || params.filePath || '';
        if (params.edits?.length) extra.edits = String(params.edits.length);
        break;
      case 'fetch':
      case 'web_fetch':
        main = params.url || '';
        if (params.format) extra.format = params.format;
        if (params.timeout) extra.timeout = `${params.timeout}s`;
        break;
      case 'agentic_fetch':
        main = params.url || '';
        break;
      case 'grep':
        main = params.pattern || '';
        if (params.path) extra.path = params.path;
        if (params.include) extra.include = params.include;
        if (params.literal_text) extra.literal = 'true';
        break;
      case 'glob':
        main = params.pattern || '';
        if (params.path) extra.path = params.path;
        break;
      case 'ls':
        main = params.path || '.';
        break;
      case 'download':
        main = params.url || '';
        if (params.file_path) extra.file_path = params.file_path;
        if (params.timeout) extra.timeout = `${params.timeout}s`;
        break;
      case 'sourcegraph':
        main = params.query || '';
        if (params.count) extra.count = String(params.count);
        if (params.context_window) extra.context = String(params.context_window);
        break;
      case 'diagnostics':
        main = 'project';
        break;
      case 'agent':
        main = params.prompt?.replace(/\n/g, ' ').slice(0, 60) || '';
        if (main.length === 60) main += '…';
        break;
      default:
        // Generic handling
        const keys = Object.keys(params);
        if (keys.length > 0) {
          main = String(params[keys[0]] || '');
          for (let i = 1; i < keys.length; i++) {
            const val = params[keys[i]];
            if (val !== undefined && val !== null && val !== '') {
              extra[keys[i]] = String(val);
            }
          }
        }
    }

    return { main, extra };
  } catch {
    return { main: input, extra: {} };
  }
};

// Get file extension for syntax highlighting
const getLanguageFromPath = (path: string): string => {
  const ext = path.split('.').pop()?.toLowerCase() || '';
  const langMap: Record<string, string> = {
    'go': 'go',
    'js': 'javascript',
    'mjs': 'javascript',
    'ts': 'typescript',
    'tsx': 'typescript',
    'jsx': 'javascript',
    'py': 'python',
    'rs': 'rust',
    'java': 'java',
    'c': 'c',
    'cpp': 'cpp',
    'cc': 'cpp',
    'cxx': 'cpp',
    'h': 'c',
    'hpp': 'cpp',
    'sh': 'bash',
    'bash': 'bash',
    'zsh': 'bash',
    'json': 'json',
    'yaml': 'yaml',
    'yml': 'yaml',
    'xml': 'xml',
    'html': 'html',
    'css': 'css',
    'md': 'markdown',
    'sql': 'sql',
  };
  return langMap[ext] || 'text';
};

// Shorten file path for display
const shortenPath = (path: string, maxLen: number = 40): string => {
  if (!path) return '';
  if (path.length <= maxLen) return path;
  
  const parts = path.split('/');
  const fileName = parts.pop() || '';
  
  // If filename alone is too long, truncate it
  if (fileName.length > maxLen - 5) {
    return '…/' + fileName.slice(-(maxLen - 5));
  }
  
  // Build path from end until it fits
  let result = fileName;
  for (let i = parts.length - 1; i >= 0; i--) {
    const newResult = parts[i] + '/' + result;
    if (newResult.length > maxLen - 2) {
      return '…/' + result;
    }
    result = newResult;
  }
  return result;
};

// Get just the filename from a path
const getFileName = (path: string): string => {
  if (!path) return '';
  const parts = path.split('/');
  return parts[parts.length - 1] || path;
};

// Check if the param looks like a file path
const isFilePath = (param: string, toolName: string): boolean => {
  const fileTools = ['view', 'edit', 'multi_edit', 'write'];
  return fileTools.includes(toolName) && param.includes('/');
};

// Code content with line numbers (TUI style)
const CodeContent: React.FC<{ 
  content: string; 
  startLine?: number; 
  maxLines?: number;
  filePath?: string;
  isNew?: boolean;
}> = ({
  content,
  startLine = 1,
  maxLines = MAX_DISPLAY_LINES,
  filePath,
  isNew = false
}) => {
  const lines = content.split('\n');
  const language = filePath ? getLanguageFromPath(filePath) : 'text';

  return (
    <div className="font-mono text-xs rounded overflow-hidden border border-gray-700/50">
      {/* File header bar */}
      {filePath && (
        <div className="flex items-center gap-2 px-3 py-1.5 bg-[#2d2d30] border-b border-gray-700/50">
          {isNew ? (
            <span className="text-emerald-400 text-[10px] font-bold px-1.5 py-0.5 bg-emerald-900/30 rounded">NEW</span>
          ) : (
            <span className="text-blue-400 text-[10px] font-bold px-1.5 py-0.5 bg-blue-900/30 rounded">FILE</span>
          )}
          <span className="text-gray-400 text-xs truncate" title={filePath}>
            {shortenPath(filePath)}
          </span>
          <span className="text-gray-600 text-[10px] ml-auto">{language}</span>
          <span className="text-gray-500 text-[10px]">{lines.length} lines</span>
        </div>
      )}
      <CodeBlock content={content} language={language} />
    </div>
  );
};

// Plain text output (TUI style)
const PlainContent: React.FC<{ content: string; maxLines?: number; label?: string }> = ({
  content,
  maxLines = MAX_DISPLAY_LINES,
  label
}) => {
  const [isExpanded, setIsExpanded] = useState(false);
  const lines = content.split('\n');
  const effectiveMaxLines = isExpanded ? lines.length : maxLines;
  const displayLines = lines.slice(0, effectiveMaxLines);
  const hiddenCount = lines.length - effectiveMaxLines;

  return (
    <div className="font-mono text-xs rounded overflow-hidden border border-gray-700/50">
      {label && (
        <div className="flex items-center gap-2 px-3 py-1.5 bg-[#2d2d30] border-b border-gray-700/50">
          <span className="text-gray-400 text-[10px] font-bold px-1.5 py-0.5 bg-gray-700/50 rounded">{label}</span>
          <span className="text-gray-600 text-[10px] ml-auto">{lines.length} lines</span>
        </div>
      )}
      <div className={cn(
        "bg-[#1e1e1e] p-2 text-gray-400 overflow-x-auto",
        isExpanded && "max-h-[400px] overflow-y-auto"
      )}>
        <div className="min-w-max">
          {displayLines.map((line, idx) => (
            <pre key={idx} className="whitespace-pre leading-relaxed m-0">
              {line || '\u00A0'}
            </pre>
          ))}
        </div>
      </div>
      {/* Footer with expand/collapse */}
      {lines.length > maxLines && (
        <div 
          className="flex items-center justify-between px-3 py-1.5 bg-[#252526] border-t border-gray-700/50 cursor-pointer hover:bg-[#2a2a2a] transition-colors"
          onClick={() => setIsExpanded(!isExpanded)}
        >
          <span className="text-gray-500 text-xs">
            {isExpanded ? `▲ Collapse` : `▼ Show all ${lines.length} lines`}
          </span>
          {!isExpanded && (
            <span className="text-gray-600 text-xs">
              … ({hiddenCount} more lines)
            </span>
          )}
        </div>
      )}
    </div>
  );
};

// Diff view component (CodeMirror style, Unified View)
const DiffContent: React.FC<{ oldContent: string; newContent: string; fileName?: string }> = ({
  oldContent,
  newContent,
  fileName
}) => {
  const language = fileName ? getLanguageFromPath(fileName) : 'javascript';
  
  // Calculate Diff and Decorations
  const { mergedContent, diffExtensions, additions, deletions } = useMemo(() => {
    const oldLines = oldContent.split('\n');
    const newLines = newContent.split('\n');
    
    // Simple Diff Logic (Heuristic)
    const diffLines: { type: 'equal' | 'insert' | 'delete', content: string }[] = [];
    let oldIdx = 0;
    let newIdx = 0;
    
    while (oldIdx < oldLines.length || newIdx < newLines.length) {
      if (oldIdx >= oldLines.length) {
        diffLines.push({ type: 'insert', content: newLines[newIdx] });
        newIdx++;
      } else if (newIdx >= newLines.length) {
        diffLines.push({ type: 'delete', content: oldLines[oldIdx] });
        oldIdx++;
      } else if (oldLines[oldIdx] === newLines[newIdx]) {
        diffLines.push({ type: 'equal', content: oldLines[oldIdx] });
        oldIdx++;
        newIdx++;
      } else {
        // Heuristic: look ahead
        let found = false;
        const searchLimit = 20; // limit search depth
        
        // Try to find old line in new content (deleted block ending?)
        for (let i = 1; i < searchLimit; i++) {
            if (newIdx + i < newLines.length && oldLines[oldIdx] === newLines[newIdx + i]) {
                // Found match ahead in new lines -> these were inserts
                for (let j = 0; j < i; j++) {
                    diffLines.push({ type: 'insert', content: newLines[newIdx + j] });
                }
                newIdx += i;
                found = true;
                break;
            }
            if (oldIdx + i < oldLines.length && newLines[newIdx] === oldLines[oldIdx + i]) {
                // Found match ahead in old lines -> these were deletes
                for (let j = 0; j < i; j++) {
                    diffLines.push({ type: 'delete', content: oldLines[oldIdx + j] });
                }
                oldIdx += i;
                found = true;
                break;
            }
        }
        
        if (!found) {
            // No simple match found, treat as 1 delete and 1 insert (substitution)
            // But prefer grouping deletes then inserts
             diffLines.push({ type: 'delete', content: oldLines[oldIdx] });
             oldIdx++;
             // We don't increment newIdx here, so next loop might insert it
        }
      }
    }
    
    // Build merged content
    const mergedContent = diffLines.map(l => l.content).join('\n');
    
    // Build decorations
    const diffTheme = EditorView.theme({
        ".cm-diff-delete": { backgroundColor: "#781b1b60" },
        ".cm-diff-insert": { backgroundColor: "#1b5e2060" },
    });

    const diffDecorations = EditorView.decorations.of((view) => {
        const builder = new RangeSetBuilder<Decoration>();
        let pos = 0;
        
        for (const line of diffLines) {
            const length = line.content.length;
            // Decoration logic
            if (line.type === 'delete') {
                 // For delete lines, add decoration to the full line
                 // Note: we need to handle the case where pos + length is valid
                 builder.add(pos, pos, Decoration.line({ class: "cm-diff-delete" }));
            } else if (line.type === 'insert') {
                 builder.add(pos, pos, Decoration.line({ class: "cm-diff-insert" }));
            }
            
            // Move pos (content + newline)
            // CodeMirror document positions include newlines as 1 character
            pos += length + 1; 
        }
        return builder.finish();
    });

    const adds = diffLines.filter(l => l.type === 'insert').length;
    const dels = diffLines.filter(l => l.type === 'delete').length;

    return {
        mergedContent,
        diffExtensions: [diffTheme, diffDecorations],
        additions: adds,
        deletions: dels
    };
  }, [oldContent, newContent]);

  return (
    <div className="font-mono text-xs rounded overflow-hidden border border-gray-700/50">
      {/* Diff header bar */}
      <div className="flex items-center gap-2 px-3 py-1.5 bg-[#2d2d30] border-b border-gray-700/50">
        <span className="text-yellow-400 text-[10px] font-bold px-1.5 py-0.5 bg-yellow-900/30 rounded">EDIT</span>
        {fileName && (
          <span className="text-gray-400 text-xs truncate" title={fileName}>
            {shortenPath(fileName)}
          </span>
        )}
        <div className="ml-auto flex items-center gap-2 text-[10px]">
          <span className="text-emerald-400">+{additions}</span>
          <span className="text-red-400">-{deletions}</span>
        </div>
      </div>

      <div>
           <CodeBlock 
             content={mergedContent} 
             copyContent={newContent} // Copy button gets the new clean content
             language={language} 
             extensions={diffExtensions}
             maxHeight="400px"
           />
      </div>
    </div>
  );
};

export const ToolCallDisplay: React.FC<ToolCallDisplayProps> = memo(({
  toolCall,
  result,
  onApprove,
  onDeny,
  needsPermission = false,
  onFileClick
}) => {
  const [isBodyExpanded, setIsBodyExpanded] = useState(true);
  
  // Debug log
  console.log('=== ToolCallDisplay render ===');
  console.log('Tool Call ID:', toolCall.id);
  console.log('Tool Call Name:', toolCall.name);
  console.log('needsPermission:', needsPermission);
  console.log('onApprove:', typeof onApprove);
  console.log('onDeny:', typeof onDeny);
  console.log('finished:', toolCall.finished);
  console.log('status:', toolCall.status);
  console.log('hasResult:', !!result);
  console.log('input preview:', toolCall.input?.substring(0, 100));
  
  // Use status field if available, otherwise fall back to old logic
  const status = toolCall.status;
  const isPending = status ? status === 'pending' : (!toolCall.finished && !result);
  const isError = status ? status === 'error' : result?.is_error;
  const isSuccess = status ? status === 'completed' : (result && !result.is_error);
  const isExecuting = status ? status === 'running' : (toolCall.finished && !result);
  const isCancelled = status === 'cancelled';

  // Parse parameters - handle empty/undefined input
  const { main: mainParam, extra: extraParams } = useMemo(() => 
    formatParams(toolCall.name || 'unknown', toolCall.input || '{}'),
    [toolCall.name, toolCall.input]
  );

  // Format extra params string
  const extraParamsStr = Object.entries(extraParams)
    .map(([k, v]) => `${k}=${v}`)
    .join(', ');

  // Get status icon and color
  const getStatusIcon = () => {
    if (needsPermission && isPending) {
      return <span className="text-orange-500">{ICONS.pending}</span>;
    }
    if (isPending) {
      return <span className="text-emerald-600 animate-pulse">{ICONS.working}</span>;
    }
    if (isSuccess) {
      return <span className="text-emerald-500">{ICONS.success}</span>;
    }
    if (isError) {
      return <span className="text-red-500">{ICONS.error}</span>;
    }
    if (isCancelled) {
      return <span className="text-gray-500">{ICONS.pending}</span>;
    }
    if (isExecuting) {
      return <span className="text-emerald-600 animate-pulse">{ICONS.working}</span>;
    }
    return <span className="text-gray-500">{ICONS.pending}</span>;
  };

  // Parse input params for preview
  const parsedInput = useMemo(() => {
    try {
      return JSON.parse(toolCall.input || '{}');
    } catch {
      return {};
    }
  }, [toolCall.input]);

  // Render preview for pending tools (before result)
  const renderPendingPreview = () => {
    switch (toolCall.name) {
      case 'write': {
        const content = parsedInput.content || '';
        const filePath = parsedInput.file_path || parsedInput.filePath || mainParam;
        if (content) {
          return (
            <div className="pl-4 mt-2">
              <CodeContent content={content} filePath={filePath} isNew={true} />
            </div>
          );
        }
        return null;
      }

      case 'edit': {
        const oldString = parsedInput.old_string || '';
        const newString = parsedInput.new_string || '';
        if (oldString || newString) {
          return (
            <div className="pl-4 mt-2">
              <DiffContent 
                oldContent={oldString} 
                newContent={newString}
                fileName={mainParam}
              />
            </div>
          );
        }
        return null;
      }

      case 'multi_edit': {
        const edits = parsedInput.edits || [];
        if (edits.length > 0) {
          return (
            <div className="pl-4 mt-2 space-y-2">
              {edits.slice(0, 3).map((edit: { old_string?: string; new_string?: string }, idx: number) => (
                <div key={idx}>
                  <div className="text-xs text-gray-500 mb-1">Edit {idx + 1}</div>
                  <DiffContent 
                    oldContent={edit.old_string || ''} 
                    newContent={edit.new_string || ''}
                  />
                </div>
              ))}
              {edits.length > 3 && (
                <div className="text-xs text-gray-500">
                  … and {edits.length - 3} more edits
                </div>
              )}
            </div>
          );
        }
        return null;
      }

      case 'bash': {
        const command = parsedInput.command || '';
        if (command) {
          return (
            <div className="pl-4 mt-2">
              <div className="font-mono text-xs rounded overflow-hidden border border-gray-700/50">
                <div className="flex items-center gap-2 px-3 py-1.5 bg-[#2d2d30] border-b border-gray-700/50">
                  <span className="text-purple-400 text-[10px] font-bold px-1.5 py-0.5 bg-purple-900/30 rounded">BASH</span>
                  {parsedInput.run_in_background && (
                    <span className="text-gray-500 text-[10px]">background</span>
                  )}
                </div>
                <div className="bg-[#1e1e1e] p-2 overflow-x-auto">
                  <div className="flex items-start gap-2 min-w-max">
                    <span className="text-emerald-400 select-none sticky left-0 bg-[#1e1e1e]">$</span>
                    <pre className="text-gray-300 whitespace-pre m-0">{command}</pre>
                  </div>
                </div>
              </div>
            </div>
          );
        }
        return null;
      }

      default:
        return null;
    }
  };

  // Render result body based on tool type
  const renderBody = () => {
    if (!result) {
      // Show preview for pending tools
      const preview = renderPendingPreview();
      
      if (needsPermission) {
        return (
          <>
            {preview}
            <div className="text-gray-500 text-xs pl-4 mt-2">
              Requesting permission...
            </div>
          </>
        );
      }
      if (isPending) {
        return (
          <>
            {preview}
            <div className="text-gray-500 text-xs pl-4 mt-2">
              Waiting for tool response...
            </div>
          </>
        );
      }
      if (isExecuting) {
        return (
          <>
            {preview}
            <div className="text-gray-500 text-xs pl-4 mt-2 flex items-center gap-2">
              <span className="animate-spin">⟳</span> Running...
            </div>
          </>
        );
      }
      if (isCancelled) {
        return (
          <>
            {preview}
            <div className="text-gray-500 text-xs italic pl-4 mt-2">
              Canceled.
            </div>
          </>
        );
      }
      return preview;
    }

    if (result.is_error) {
      return (
        <div className="pl-4 mt-2">
          <div className="flex items-center gap-2">
            <span className="bg-red-600 text-white text-xs px-2 py-0.5 rounded">
              ERROR
            </span>
            <span className="text-gray-400 text-xs truncate">
              {result.content.split('\n')[0]}
            </span>
          </div>
        </div>
      );
    }

    // Parse metadata if available
    let metadata: Record<string, unknown> = {};
    if (result.metadata) {
      try {
        metadata = JSON.parse(result.metadata);
      } catch {
        // ignore parse errors
      }
    }

    // Render based on tool type
    switch (toolCall.name) {
      case 'view': {
        // Don't show file content for view tool - just show success
        return null;
      }

      case 'edit':
      case 'multi_edit': {
        const oldContent = (metadata.old_content as string) || '';
        const newContent = (metadata.new_content as string) || '';
        if (oldContent || newContent) {
          return (
            <div className="pl-4 mt-2">
              <DiffContent 
                oldContent={oldContent} 
                newContent={newContent}
                fileName={mainParam}
              />
            </div>
          );
        }
        return (
          <div className="pl-4 mt-2">
            <PlainContent content={result.content} />
          </div>
        );
      }

      case 'write': {
        try {
          const params = JSON.parse(toolCall.input);
          const content = params.content || result.content;
          const filePath = params.file_path || params.filePath || mainParam;
          return (
            <div className="pl-4 mt-2">
              <CodeContent content={content} filePath={filePath} isNew={true} />
            </div>
          );
        } catch {
          return (
            <div className="pl-4 mt-2">
              <PlainContent content={result.content} />
            </div>
          );
        }
      }

      case 'bash': {
        const output = (metadata.output as string) || result.content;
        if (output && output !== 'No output') {
          return (
            <div className="pl-4 mt-2">
              <PlainContent content={output} label="OUTPUT" />
            </div>
          );
        }
        return null;
      }

      case 'ls':
        // Don't show directory listing content
        return null;

      default:
        if (result.content && result.content.trim()) {
          return (
            <div className="pl-4 mt-2">
              <PlainContent content={result.content} />
            </div>
          );
        }
        return null;
    }
  };

  // Don't render if toolCall has no name
  if (!toolCall.name) {
    console.warn('ToolCallDisplay: toolCall has no name', toolCall);
    return null;
  }

  return (
    <div className="my-2 pl-2 border-l-2 border-emerald-700/50 hover:border-emerald-600/70 transition-colors">
      {/* Header - TUI style: [icon] [tool name] [params] */}
      <div 
        className="flex items-center gap-2 cursor-pointer select-none"
        onClick={() => setIsBodyExpanded(!isBodyExpanded)}
      >
        {getStatusIcon()}
        
        <span className="font-medium text-sm text-blue-400">
          {prettifyToolName(toolCall.name)}
        </span>

        {mainParam && (
          <span className="text-gray-500 text-xs truncate flex-1">
            {isFilePath(mainParam, toolCall.name) ? (
              <span 
                className="text-blue-400 hover:text-blue-300 hover:underline cursor-pointer"
                onClick={(e) => {
                  e.stopPropagation();
                  onFileClick?.(mainParam);
                }}
                title={mainParam}
              >
                {getFileName(mainParam)}
              </span>
            ) : (
              mainParam
            )}
            {extraParamsStr && (
              <span className="text-gray-600 ml-1">
                ({extraParamsStr})
              </span>
            )}
          </span>
        )}

        {/* Working animation for pending/executing */}
        {(isPending || isExecuting) && !needsPermission && (
          <div className="flex items-center gap-1">
            <div className="flex gap-0.5">
              {[0, 1, 2].map(i => (
                <div 
                  key={i}
                  className="w-1 h-1 bg-emerald-500 rounded-full animate-pulse"
                  style={{ animationDelay: `${i * 150}ms` }}
                />
              ))}
            </div>
            <span className="text-emerald-600 text-xs">{isExecuting ? 'Running' : 'Working'}</span>
          </div>
        )}
      </div>

      {/* Permission Buttons - more compact TUI style */}
      {needsPermission && onApprove && onDeny ? (
        <div className="flex gap-2 mt-2 pl-4">
          <button
            onClick={() => {
              console.log('=== Approve button clicked ===');
              console.log('Tool Call ID:', toolCall.id);
              onApprove(toolCall.id);
            }}
            className="px-3 py-1 text-xs bg-emerald-700 hover:bg-emerald-600 text-white rounded transition-colors"
          >
            ✓ Allow
          </button>
          <button
            onClick={() => {
              console.log('=== Deny button clicked ===');
              console.log('Tool Call ID:', toolCall.id);
              onDeny(toolCall.id);
            }}
            className="px-3 py-1 text-xs bg-gray-700 hover:bg-gray-600 text-white rounded transition-colors"
          >
            × Deny
          </button>
        </div>
      ) : (
        needsPermission && console.log('=== Permission buttons NOT rendered ===', {
          needsPermission,
          hasOnApprove: !!onApprove,
          hasOnDeny: !!onDeny
        }) && null
      )}

      {/* Body - collapsible */}
      {isBodyExpanded && renderBody()}
    </div>
  );
}, (prevProps, nextProps) => {
  // Only re-render if tool call status, result, or permission status changes
  return (
    prevProps.toolCall.id === nextProps.toolCall.id &&
    prevProps.toolCall.status === nextProps.toolCall.status &&
    prevProps.toolCall.finished === nextProps.toolCall.finished &&
    prevProps.result === nextProps.result &&
    prevProps.needsPermission === nextProps.needsPermission
  );
});