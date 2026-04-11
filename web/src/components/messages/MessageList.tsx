import { useRef } from 'react';
import type { Message } from '../../api/client';
import { MarkdownRenderer } from './MarkdownRenderer';
import { useMessageStore } from '../../stores/messages';

interface Props {
  messages: Message[];
  loading: boolean;
  channelId: number;
}

function formatTime(iso: string): string {
  const d = new Date(iso);
  const now = new Date();
  const diff = now.getTime() - d.getTime();

  if (diff < 60000) return 'Just now';
  if (diff < 3600000) return `${Math.floor(diff / 60000)}m ago`;
  if (diff < 86400000) return `${Math.floor(diff / 3600000)}h ago`;
  if (diff < 604800000) return `${Math.floor(diff / 86400000)}d ago`;
  return d.toLocaleDateString();
}

function MessageBubble({ msg, isGroupStart, channelId }: { msg: Message; isGroupStart: boolean; channelId: number }) {
  const activeToolCalls = useMessageStore((s) => s.activeToolCalls[channelId] || []);

  if (msg.is_system) {
    return (
      <div className="text-center my-2">
        <span className="text-xs text-[var(--text-muted)] bg-[var(--bg-secondary)] px-3 py-1 rounded">
          {msg.content}
        </span>
      </div>
    );
  }

  // For AI messages, include active tool calls
  const toolCalls = msg.is_ai && msg.id === -1 ? activeToolCalls : undefined;

  return (
    <div className={`flex gap-3 px-4 py-0.5 hover:bg-[var(--bg-secondary)]/50 ${isGroupStart ? 'mt-3' : ''}`}>
      {isGroupStart && msg.author && (
        <img
          src={msg.author.avatar_url}
          alt=""
          className="w-9 h-9 rounded-full mt-0.5 shrink-0"
        />
      )}
      {!isGroupStart && <div className="w-9 shrink-0" />}
      <div className="flex-1 min-w-0">
        {isGroupStart && (
          <div className="flex items-baseline gap-2 mb-0.5">
            <span className={`text-sm font-medium ${msg.is_ai ? 'text-[var(--accent)]' : 'text-[var(--text-primary)]'}`}>
              {msg.is_ai ? 'AI Assistant' : msg.author?.display_name || msg.author?.username || 'Unknown'}
            </span>
            <span className="text-xs text-[var(--text-muted)]">{formatTime(msg.created_at)}</span>
            {msg.edited_at && <span className="text-xs text-[var(--text-muted)]">(edited)</span>}
          </div>
        )}
        <div className={`text-sm text-[var(--text-primary)] leading-relaxed break-words ${msg.is_ai ? 'bg-[var(--bg-secondary)]/50 rounded-lg px-3 py-2' : ''}`}>
          <MarkdownRenderer content={msg.content} toolCalls={toolCalls} />
        </div>
      </div>
    </div>
  );
}

export function MessageList({ messages, loading, channelId }: Props) {
  const listRef = useRef<HTMLDivElement>(null);
  const topRef = useRef<HTMLDivElement>(null);

  // Group messages by author + time (5 min gap)
  const shouldShowHeader = (index: number): boolean => {
    if (index === 0) return true;
    const prev = messages[index - 1];
    const curr = messages[index];
    if (prev.author_id !== curr.author_id) return true;
    const prevTime = new Date(prev.created_at).getTime();
    const currTime = new Date(curr.created_at).getTime();
    return currTime - prevTime > 5 * 60 * 1000;
  };

  return (
    <div ref={listRef} className="flex-1 overflow-y-auto">
      <div ref={topRef} />
      {loading && (
        <div className="text-center py-8 text-sm text-[var(--text-muted)]">Loading messages...</div>
      )}
      {messages.length === 0 && !loading && (
        <div className="text-center py-8 text-sm text-[var(--text-muted)]">
          No messages yet. Be the first to say something!
        </div>
      )}
      {messages.map((msg, i) => (
        <MessageBubble key={msg.id} msg={msg} isGroupStart={shouldShowHeader(i)} channelId={channelId} />
      ))}
    </div>
  );
}
