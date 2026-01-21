import React, { memo } from 'react';
import { CheckCircle2, Circle, Loader2, ListTodo } from 'lucide-react';
import { type Todo, type TodoStatus } from '../types';
import { cn } from '../lib/utils';

interface TodosDisplayProps {
  todos: Todo[];
  currentTask?: string;
  className?: string;
}

const StatusIcon = ({ status }: { status: TodoStatus }) => {
  switch (status) {
    case 'completed':
      return <CheckCircle2 size={14} className="text-emerald-400 shrink-0" />;
    case 'in_progress':
      return <Loader2 size={14} className="text-blue-400 animate-spin shrink-0" />;
    case 'pending':
    default:
      return <Circle size={14} className="text-gray-500 shrink-0" />;
  }
};

const TodoItem = memo(({ todo }: { todo: Todo }) => {
  const isActive = todo.status === 'in_progress';
  const isCompleted = todo.status === 'completed';

  return (
    <div
      className={cn(
        "flex items-start gap-2 py-1.5 px-2 rounded transition-colors",
        isActive && "bg-blue-500/10 border-l-2 border-blue-400",
        isCompleted && "opacity-60"
      )}
    >
      <StatusIcon status={todo.status} />
      <span
        className={cn(
          "text-xs leading-relaxed",
          isCompleted && "line-through text-gray-500",
          isActive && "text-blue-200 font-medium",
          !isActive && !isCompleted && "text-gray-300"
        )}
      >
        {isActive && todo.active_form ? todo.active_form : todo.content}
      </span>
    </div>
  );
});

export const TodosDisplay = memo(({ todos, currentTask, className }: TodosDisplayProps) => {
  if (!todos || todos.length === 0) {
    return null;
  }

  const completedCount = todos.filter(t => t.status === 'completed').length;
  const totalCount = todos.length;
  const progressPercent = totalCount > 0 ? Math.round((completedCount / totalCount) * 100) : 0;

  return (
    <div className={cn("rounded-lg border border-gray-700/50 bg-[#0d0d0d] overflow-hidden", className)}>
      {/* Header */}
      <div className="flex items-center justify-between px-3 py-2 bg-[#111] border-b border-gray-700/50">
        <div className="flex items-center gap-2">
          <ListTodo size={14} className="text-purple-400" />
          <span className="text-xs font-medium text-gray-300">Tasks</span>
        </div>
        <div className="flex items-center gap-2">
          <span className="text-[10px] text-gray-500">
            {completedCount}/{totalCount}
          </span>
          <div className="w-12 h-1.5 bg-gray-700 rounded-full overflow-hidden">
            <div
              className="h-full bg-gradient-to-r from-purple-500 to-blue-500 transition-all duration-300"
              style={{ width: `${progressPercent}%` }}
            />
          </div>
        </div>
      </div>

      {/* Current task indicator */}
      {currentTask && (
        <div className="px-3 py-1.5 bg-blue-500/5 border-b border-blue-500/20 flex items-center gap-2">
          <Loader2 size={12} className="text-blue-400 animate-spin" />
          <span className="text-[11px] text-blue-300 truncate">{currentTask}</span>
        </div>
      )}

      {/* Todo list */}
      <div className="p-2 space-y-0.5 max-h-[200px] overflow-y-auto scrollbar-thin scrollbar-thumb-gray-700 scrollbar-track-transparent">
        {todos.map((todo, index) => (
          <TodoItem key={`${todo.content}-${index}`} todo={todo} />
        ))}
      </div>

      {/* Footer with progress */}
      {progressPercent === 100 && (
        <div className="px-3 py-1.5 bg-emerald-500/10 border-t border-emerald-500/20 flex items-center justify-center gap-1">
          <CheckCircle2 size={12} className="text-emerald-400" />
          <span className="text-[10px] text-emerald-300 font-medium">All tasks completed!</span>
        </div>
      )}
    </div>
  );
});
