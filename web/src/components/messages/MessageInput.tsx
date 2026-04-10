import { useState, useRef, useEffect } from 'react';

interface Props {
  onSend: (content: string) => void;
  channelId: number;
}

export function MessageInput({ onSend, channelId }: Props) {
  const [content, setContent] = useState('');
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  // Auto-resize textarea
  useEffect(() => {
    const ta = textareaRef.current;
    if (ta) {
      ta.style.height = 'auto';
      ta.style.height = Math.min(ta.scrollHeight, 200) + 'px';
    }
  }, [content]);

  // Focus input when channel changes
  useEffect(() => {
    setContent('');
    textareaRef.current?.focus();
  }, [channelId]);

  const handleSubmit = () => {
    if (!content.trim()) return;
    onSend(content);
    setContent('');
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSubmit();
    }
  };

  return (
    <div className="px-4 pb-4 pt-2 shrink-0">
      <div className="flex items-end bg-[var(--bg-secondary)] rounded-lg border border-[var(--border)] px-3">
        <textarea
          ref={textareaRef}
          value={content}
          onChange={(e) => setContent(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={`Message #${channelId || '...'} (Enter to send, Shift+Enter for new line)`}
          className="flex-1 bg-transparent text-sm text-[var(--text-primary)] py-2.5 resize-none focus:outline-none max-h-[200px] placeholder:text-[var(--text-muted)]"
          rows={1}
        />
        <button
          onClick={handleSubmit}
          disabled={!content.trim()}
          className="px-2 py-1 text-[var(--text-muted)] hover:text-[var(--text-primary)] transition-colors disabled:opacity-30 shrink-0 mb-1"
          title="Send message"
        >
          <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor">
            <path d="M2.01 21L23 12 2.01 3 2.01 10l10-2L21 12l-9 2z" />
          </svg>
        </button>
      </div>
    </div>
  );
}
