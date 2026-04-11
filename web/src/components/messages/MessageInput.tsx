import { useState, useRef, useEffect } from 'react';
import { api } from '../../api/client';
import { useWorkspaceStore } from '../../stores/workspace';

interface Props {
  onSend: (content: string) => Promise<void>;
  channelId: number;
  channelName?: string;
}

export function MessageInput({ onSend, channelId, channelName }: Props) {
  const [content, setContent] = useState('');
  const [sending, setSending] = useState(false);
  const [uploading, setUploading] = useState(false);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const { activeWorkspace } = useWorkspaceStore();

  useEffect(() => {
    const ta = textareaRef.current;
    if (ta) {
      ta.style.height = 'auto';
      ta.style.height = Math.min(ta.scrollHeight, 200) + 'px';
    }
  }, [content]);

  useEffect(() => {
    setContent('');
    textareaRef.current?.focus();
  }, [channelId]);

  const handleSubmit = async () => {
    if (!content.trim() || sending) return;
    const text = content;
    setContent('');
    setSending(true);
    try {
      await onSend(text);
    } catch {
      setContent(text);
    } finally {
      setSending(false);
    }
  };

  const handleFileAttach = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files;
    if (!files?.length || !activeWorkspace) return;

    setUploading(true);
    for (const file of Array.from(files)) {
      try {
        const uploaded = await api.uploadFile(activeWorkspace.id, file);
        // Insert file reference into message
        setContent((prev) => {
          const ref = file.type.startsWith('image/')
            ? `![${uploaded.filename}](${api.getFileUrl(uploaded.id)})`
            : `[${uploaded.filename}](${api.getFileUrl(uploaded.id)})`;
          return prev ? `${prev}\n${ref}` : ref;
        });
      } catch {
        // Skip failed uploads
      }
    }
    setUploading(false);
    if (fileInputRef.current) fileInputRef.current.value = '';
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSubmit();
    }
  };

  const placeholder = channelName
    ? `Message #${channelName} (Enter to send, Shift+Enter for new line)`
    : 'Message (Enter to send, Shift+Enter for new line)';

  return (
    <div className="px-4 pb-4 pt-2 shrink-0">
      <div className="flex items-end bg-[var(--bg-secondary)] rounded-lg border border-[var(--border)] px-3">
        {/* File attach button */}
        <button
          onClick={() => fileInputRef.current?.click()}
          disabled={uploading || !activeWorkspace}
          className="px-1 py-1 text-[var(--text-muted)] hover:text-[var(--text-primary)] transition-colors disabled:opacity-30 shrink-0 mb-1.5"
          title="Attach file"
        >
          {uploading ? (
            <span className="text-xs">...</span>
          ) : (
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M21.44 11.05l-9.19 9.19a6 6 0 01-8.49-8.49l9.19-9.19a4 4 0 015.66 5.66l-9.2 9.19a2 2 0 01-2.83-2.83l8.49-8.48" />
            </svg>
          )}
        </button>
        <input
          ref={fileInputRef}
          type="file"
          multiple
          onChange={handleFileAttach}
          className="hidden"
        />

        <textarea
          ref={textareaRef}
          value={content}
          onChange={(e) => setContent(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          disabled={sending}
          className="flex-1 bg-transparent text-sm text-[var(--text-primary)] py-2.5 resize-none focus:outline-none max-h-[200px] placeholder:text-[var(--text-muted)] disabled:opacity-50"
          rows={1}
        />
        <button
          onClick={handleSubmit}
          disabled={!content.trim() || sending}
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
