import React, { useState, useMemo } from 'react';
import { type ToolCall, type ToolResult } from '../types';
import { cn } from '../lib/utils';

interface ToolCallDisplayProps {
  toolCall: ToolCall;
  result?: ToolResult;
  onApprove?: (toolCallId: string) => void;
  onDeny?: (toolCallId: string) => void;
  needsPermission?: boolean;
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
  const displayLines = lines.slice(0, maxLines);
  const hiddenCount = lines.length - maxLines;
  const maxLineNum = startLine + displayLines.length - 1;
  const lineNumWidth = Math.max(String(maxLineNum).length, 2);
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
        </div>
      )}
      {/* Code content */}
      <div className="bg-[#1e1e1e]">
        {displayLines.map((line, idx) => (
          <div key={idx} className="flex hover:bg-[#2a2a2a]">
            <span 
              className="text-gray-600 bg-[#1e1e1e] px-2 py-0.5 text-right select-none border-r border-gray-800"
              style={{ minWidth: `${lineNumWidth + 1.5}ch` }}
            >
              {startLine + idx}
            </span>
            <span className="text-gray-300 px-3 py-0.5 flex-1 overflow-x-auto whitespace-pre">
              {line || ' '}
            </span>
          </div>
        ))}
        {hiddenCount > 0 && (
          <div className="text-gray-500 bg-[#252526] px-3 py-1.5 text-xs border-t border-gray-700/50">
            … ({hiddenCount} more lines)
          </div>
        )}
      </div>
    </div>
  );
};

// Plain text output (TUI style)
const PlainContent: React.FC<{ content: string; maxLines?: number; label?: string }> = ({
  content,
  maxLines = MAX_DISPLAY_LINES,
  label
}) => {
  const lines = content.split('\n');
  const displayLines = lines.slice(0, maxLines);
  const hiddenCount = lines.length - maxLines;

  return (
    <div className="font-mono text-xs rounded overflow-hidden border border-gray-700/50">
      {label && (
        <div className="flex items-center gap-2 px-3 py-1.5 bg-[#2d2d30] border-b border-gray-700/50">
          <span className="text-gray-400 text-[10px] font-bold px-1.5 py-0.5 bg-gray-700/50 rounded">{label}</span>
        </div>
      )}
      <div className="bg-[#1e1e1e] p-2 text-gray-400">
        {displayLines.map((line, idx) => (
          <div key={idx} className="whitespace-pre-wrap break-all leading-relaxed">
            {line || '\u00A0'}
          </div>
        ))}
        {hiddenCount > 0 && (
          <div className="text-gray-500 mt-1 pt-1 border-t border-gray-700/30">
            … ({hiddenCount} more lines)
          </div>
        )}
      </div>
    </div>
  );
};

// Diff view component (TUI style)
const DiffContent: React.FC<{ oldContent: string; newContent: string; fileName?: string }> = ({
  oldContent,
  newContent,
  fileName
}) => {
  const diff = useMemo(() => {
    const oldLines = oldContent.split('\n');
    const newLines = newContent.split('\n');
    
    // Simple diff algorithm - find changes
    const result: Array<{ type: 'equal' | 'insert' | 'delete'; content: string; lineNum?: number }> = [];
    let oldIdx = 0;
    let newIdx = 0;
    
    while (oldIdx < oldLines.length || newIdx < newLines.length) {
      if (oldIdx >= oldLines.length) {
        result.push({ type: 'insert', content: newLines[newIdx], lineNum: newIdx + 1 });
        newIdx++;
      } else if (newIdx >= newLines.length) {
        result.push({ type: 'delete', content: oldLines[oldIdx], lineNum: oldIdx + 1 });
        oldIdx++;
      } else if (oldLines[oldIdx] === newLines[newIdx]) {
        result.push({ type: 'equal', content: oldLines[oldIdx], lineNum: oldIdx + 1 });
        oldIdx++;
        newIdx++;
      } else {
        // Check for insert or delete
        const oldInNew = newLines.indexOf(oldLines[oldIdx], newIdx);
        const newInOld = oldLines.indexOf(newLines[newIdx], oldIdx);
        
        if (oldInNew === -1 && newInOld === -1) {
          result.push({ type: 'delete', content: oldLines[oldIdx], lineNum: oldIdx + 1 });
          result.push({ type: 'insert', content: newLines[newIdx], lineNum: newIdx + 1 });
          oldIdx++;
          newIdx++;
        } else if (oldInNew !== -1 && (newInOld === -1 || oldInNew - newIdx < newInOld - oldIdx)) {
          while (newIdx < oldInNew) {
            result.push({ type: 'insert', content: newLines[newIdx], lineNum: newIdx + 1 });
            newIdx++;
          }
        } else {
          while (oldIdx < newInOld) {
            result.push({ type: 'delete', content: oldLines[oldIdx], lineNum: oldIdx + 1 });
            oldIdx++;
          }
        }
      }
    }
    
    return result.slice(0, MAX_DISPLAY_LINES * 2); // Allow more lines for diff
  }, [oldContent, newContent]);

  const additions = diff.filter(d => d.type === 'insert').length;
  const deletions = diff.filter(d => d.type === 'delete').length;
  const totalChanges = additions + deletions;

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
      {/* Diff content */}
      <div className="bg-[#1e1e1e]">
        {diff.map((line, idx) => (
          <div 
            key={idx} 
            className={cn(
              "flex",
              line.type === 'insert' && "bg-[#1e3a1e]",
              line.type === 'delete' && "bg-[#3a1e1e]"
            )}
          >
            <span 
              className={cn(
                "px-2 py-0.5 text-right select-none min-w-[3ch] border-r border-gray-800",
                line.type === 'insert' && "text-emerald-500 bg-[#1a2e1a]",
                line.type === 'delete' && "text-red-500 bg-[#2e1a1a]",
                line.type === 'equal' && "text-gray-600 bg-[#1e1e1e]"
              )}
            >
              {line.type === 'insert' ? '+' : line.type === 'delete' ? '-' : ' '}
            </span>
            <span 
              className={cn(
                "px-3 py-0.5 flex-1 whitespace-pre overflow-x-auto",
                line.type === 'insert' && "text-emerald-300",
                line.type === 'delete' && "text-red-300",
                line.type === 'equal' && "text-gray-400"
              )}
            >
              {line.content || ' '}
            </span>
          </div>
        ))}
        {totalChanges > MAX_DISPLAY_LINES * 2 && (
          <div className="text-gray-500 bg-[#252526] px-3 py-1.5 text-xs border-t border-gray-700/50">
            … (more changes)
          </div>
        )}
      </div>
    </div>
  );
};

export const ToolCallDisplay: React.FC<ToolCallDisplayProps> = ({
  toolCall,
  result,
  onApprove,
  onDeny,
  needsPermission = false
}) => {
  const [isBodyExpanded, setIsBodyExpanded] = useState(true);
  
  // Debug log
  console.log('ToolCallDisplay render:', { 
    id: toolCall.id, 
    name: toolCall.name, 
    input: toolCall.input?.substring(0, 100),
    finished: toolCall.finished,
    hasResult: !!result 
  });
  
  const isPending = !toolCall.finished && !result;
  const isError = result?.is_error;
  const isSuccess = result && !result.is_error;
  const isCancelled = toolCall.finished && !result;

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
                <div className="bg-[#1e1e1e] p-2">
                  <div className="flex items-start gap-2">
                    <span className="text-emerald-400 select-none">$</span>
                    <span className="text-gray-300 whitespace-pre-wrap break-all">{command}</span>
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
        const filePath = metadata.file_path as string || mainParam;
        const content = (metadata.content as string) || result.content;
        const offset = parseInt(extraParams.offset || '0', 10);
        return (
          <div className="pl-4 mt-2">
            <CodeContent content={content} startLine={offset + 1} filePath={filePath} />
          </div>
        );
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
            {mainParam}
            {extraParamsStr && (
              <span className="text-gray-600 ml-1">
                ({extraParamsStr})
              </span>
            )}
          </span>
        )}

        {/* Working animation for pending */}
        {isPending && !needsPermission && (
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
            <span className="text-emerald-600 text-xs">Working</span>
          </div>
        )}
      </div>

      {/* Permission Buttons - more compact TUI style */}
      {needsPermission && onApprove && onDeny && (
        <div className="flex gap-2 mt-2 pl-4">
          <button
            onClick={() => onApprove(toolCall.id)}
            className="px-3 py-1 text-xs bg-emerald-700 hover:bg-emerald-600 text-white rounded transition-colors"
          >
            ✓ Allow
          </button>
          <button
            onClick={() => onDeny(toolCall.id)}
            className="px-3 py-1 text-xs bg-gray-700 hover:bg-gray-600 text-white rounded transition-colors"
          >
            × Deny
          </button>
        </div>
      )}

      {/* Body - collapsible */}
      {isBodyExpanded && renderBody()}
    </div>
  );
};
