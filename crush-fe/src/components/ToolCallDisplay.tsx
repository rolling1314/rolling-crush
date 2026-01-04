import React, { useState } from 'react';
import { Wrench, CheckCircle, XCircle, Clock, AlertCircle, ChevronDown, ChevronUp } from 'lucide-react';
import { type ToolCall, type ToolResult } from '../types';
import { cn } from '../lib/utils';

interface ToolCallDisplayProps {
  toolCall: ToolCall;
  result?: ToolResult;
  onApprove?: (toolCallId: string) => void;
  onDeny?: (toolCallId: string) => void;
  needsPermission?: boolean;
}

export const ToolCallDisplay: React.FC<ToolCallDisplayProps> = ({
  toolCall,
  result,
  onApprove,
  onDeny,
  needsPermission = false
}) => {
  const [isParamsExpanded, setIsParamsExpanded] = useState(false);
  const [isResultExpanded, setIsResultExpanded] = useState(true);
  
  console.log('ToolCallDisplay rendering:', {
    toolCallId: toolCall.id,
    needsPermission,
    hasOnApprove: !!onApprove,
    hasOnDeny: !!onDeny,
    willShowButtons: needsPermission && !!onApprove && !!onDeny
  });
  
  const isPending = !toolCall.finished && !result;
  const isError = result?.is_error;
  const isSuccess = result && !result.is_error;

  // Prettify tool name
  const prettifyToolName = (name: string): string => {
    return name
      .replace(/_/g, ' ')
      .split(' ')
      .map(word => word.charAt(0).toUpperCase() + word.slice(1))
      .join(' ');
  };

  // Parse input if it's JSON
  const parseInput = (input: string) => {
    try {
      return JSON.parse(input);
    } catch {
      return input;
    }
  };

  const parsedInput = toolCall.input ? parseInput(toolCall.input) : null;

  return (
    <div className={cn(
      "my-2 rounded-lg border-l-4 p-3 bg-gray-800/50 max-w-[95%]",
      isPending && "border-yellow-500",
      isSuccess && "border-green-500",
      isError && "border-red-500",
      needsPermission && "border-orange-500"
    )}>
      {/* Header */}
      <div className="flex items-center gap-2 mb-2">
        {isPending && <Clock className="w-4 h-4 text-yellow-500 animate-spin" />}
        {isSuccess && <CheckCircle className="w-4 h-4 text-green-500" />}
        {isError && <XCircle className="w-4 h-4 text-red-500" />}
        {needsPermission && <AlertCircle className="w-4 h-4 text-orange-500" />}
        
        <Wrench className="w-4 h-4 text-gray-400" />
        <span className="font-semibold text-sm text-blue-400">
          {prettifyToolName(toolCall.name)}
        </span>

        {needsPermission && (
          <span className="text-xs text-orange-400 ml-auto">
            Permission Required
          </span>
        )}
        {isPending && !needsPermission && (
          <span className="text-xs text-yellow-400 ml-auto animate-pulse">
            Running...
          </span>
        )}
      </div>

      {/* Permission Buttons */}
      {needsPermission && onApprove && onDeny && (
        <div className="flex gap-2 mb-2">
          <button
            onClick={() => onApprove(toolCall.id)}
            className="flex-1 px-3 py-1.5 text-xs bg-green-600 hover:bg-green-700 text-white rounded-md transition-colors flex items-center justify-center gap-1"
          >
            <CheckCircle className="w-3 h-3" />
            Approve
          </button>
          <button
            onClick={() => onDeny(toolCall.id)}
            className="flex-1 px-3 py-1.5 text-xs bg-red-600 hover:bg-red-700 text-white rounded-md transition-colors flex items-center justify-center gap-1"
          >
            <XCircle className="w-3 h-3" />
            Deny
          </button>
        </div>
      )}

      {/* Input Parameters (Collapsible) */}
      {parsedInput && (
        <div className="mt-2 text-xs">
          <button
            onClick={() => setIsParamsExpanded(!isParamsExpanded)}
            className="flex items-center gap-1 text-gray-400 hover:text-gray-300 mb-1 transition-colors"
          >
            {isParamsExpanded ? (
              <ChevronUp className="w-3 h-3" />
            ) : (
              <ChevronDown className="w-3 h-3" />
            )}
            <span>Parameters</span>
          </button>
          {isParamsExpanded && (
            <div className="bg-gray-900/50 rounded p-2 overflow-x-auto max-h-[200px] overflow-y-auto">
              {typeof parsedInput === 'object' ? (
                <pre className="text-gray-300 font-mono text-xs">
                  {JSON.stringify(parsedInput, null, 2)}
                </pre>
              ) : (
                <div className="text-gray-300 break-words">{parsedInput}</div>
              )}
            </div>
          )}
        </div>
      )}

      {/* Result (Collapsible) */}
      {result && (
        <div className="mt-2 text-xs">
          <button
            onClick={() => setIsResultExpanded(!isResultExpanded)}
            className={cn(
              "flex items-center gap-1 mb-1 transition-colors",
              result.is_error ? "text-red-400 hover:text-red-300" : "text-green-400 hover:text-green-300"
            )}
          >
            {isResultExpanded ? (
              <ChevronUp className="w-3 h-3" />
            ) : (
              <ChevronDown className="w-3 h-3" />
            )}
            <span>{result.is_error ? 'Error' : 'Result'}</span>
          </button>
          {isResultExpanded && (
            <div className={cn(
              "rounded p-2 overflow-x-auto max-h-[200px] overflow-y-auto",
              result.is_error ? "bg-red-900/20 border border-red-800/30" : "bg-gray-900/50"
            )}>
              <pre className={cn(
                "font-mono text-xs whitespace-pre-wrap break-words",
                result.is_error ? "text-red-200" : "text-gray-300"
              )}>
                {result.content}
              </pre>
            </div>
          )}
        </div>
      )}
    </div>
  );
};

