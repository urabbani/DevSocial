import { useState } from 'react';
import type { ToolCall } from '../../api/client';

interface ToolCallBlockProps {
  toolCall: ToolCall;
}

const STATUS_ICONS: Record<string, string> = {
  pending: '⏳',
  executing: '▶️',
  completed: '✅',
  error: '❌',
};

const STATUS_COLORS: Record<string, string> = {
  pending: 'text-yellow-500',
  executing: 'text-blue-500',
  completed: 'text-green-500',
  error: 'text-red-500',
};

export function ToolCallBlock({ toolCall }: ToolCallBlockProps) {
  const [expanded, setExpanded] = useState(true);

  const toggleExpanded = () => setExpanded((prev) => !prev);

  return (
    <div className="bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg overflow-hidden">
      {/* Header */}
      <button
        onClick={toggleExpanded}
        className="w-full px-3 py-2 flex items-center gap-2 hover:bg-[var(--bg-hover)] transition-colors text-left"
      >
        <span className="text-sm">{expanded ? '▼' : '▶'}</span>
        <span className={STATUS_COLORS[toolCall.status] || 'text-gray-500'}>
          {STATUS_ICONS[toolCall.status] || '🔧'}
        </span>
        <span className="text-sm font-medium text-[var(--text-primary)]">
          {toolCall.name}
        </span>
        <span className="text-xs text-[var(--text-muted)] ml-auto">
          {toolCall.status}
        </span>
      </button>

      {/* Content */}
      {expanded && (
        <div className="px-3 pb-3 border-t border-[var(--border)]">
          {/* Arguments */}
          {toolCall.arguments && (
            <div className="mt-2">
              <div className="text-xs text-[var(--text-muted)] mb-1">Arguments:</div>
              <pre className="bg-[var(--bg-tertiary)] rounded p-2 text-xs overflow-x-auto">
                {JSON.stringify(JSON.parse(toolCall.arguments), null, 2)}
              </pre>
            </div>
          )}

          {/* Result */}
          {toolCall.result && (
            <div className="mt-2">
              <div className="text-xs text-[var(--text-muted)] mb-1">Result:</div>
              <div className="bg-[var(--bg-tertiary)] rounded p-2 text-xs whitespace-pre-wrap break-words">
                {toolCall.result}
              </div>
            </div>
          )}

          {/* Error indicator */}
          {toolCall.status === 'error' && !toolCall.result && (
            <div className="mt-2 text-xs text-red-400">
              Tool execution failed
            </div>
          )}
        </div>
      )}
    </div>
  );
}
