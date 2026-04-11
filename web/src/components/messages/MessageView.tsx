import { useEffect, useRef, useCallback } from 'react';
import { useWorkspaceStore } from '../../stores/workspace';
import { useMessageStore } from '../../stores/messages';
import { MessageList } from './MessageList';
import { MessageInput } from './MessageInput';
import { ChannelHeader } from '../channels/ChannelHeader';

export function MessageView() {
  const { activeChannel } = useWorkspaceStore();
  const { messagesByChannel, loadingByChannel, fetchMessages } = useMessageStore();
  const bottomRef = useRef<HTMLDivElement>(null);
  const prevLengthRef = useRef(0);

  const channelId = activeChannel?.id ?? 0;
  const messages = channelId ? messagesByChannel[channelId] || [] : [];
  const loading = channelId ? loadingByChannel[channelId] : false;

  useEffect(() => {
    if (channelId) {
      fetchMessages(channelId);
    }
  }, [channelId, fetchMessages]);

  // Auto-scroll to bottom only when new messages arrive
  useEffect(() => {
    if (messages.length > prevLengthRef.current) {
      bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
    }
    prevLengthRef.current = messages.length;
  }, [messages.length]);

  const handleSend = useCallback(
    async (content: string) => {
      if (!channelId || !content.trim()) return;
      const res = await fetch(`/api/channels/${channelId}/messages`, {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ content: content.trim() }),
      });
      if (!res.ok) {
        const body = await res.json().catch(() => ({ error: 'Failed to send' }));
        throw new Error(body.error);
      }
      await res.json();
      // WebSocket broadcast will add the message
    },
    [channelId]
  );

  if (!activeChannel) {
    return (
      <div className="flex-1 flex items-center justify-center bg-[var(--bg-primary)]">
        <div className="text-center text-[var(--text-muted)]">
          <p className="text-lg mb-1">Welcome to DevSocial</p>
          <p className="text-sm">Select a channel to start chatting</p>
        </div>
      </div>
    );
  }

  return (
    <div className="flex-1 flex flex-col bg-[var(--bg-primary)]">
      <ChannelHeader channel={activeChannel} />

      <MessageList messages={messages} loading={loading} channelId={channelId} />

      <div ref={bottomRef} />
      <MessageInput onSend={handleSend} channelId={channelId} channelName={activeChannel.name} />
    </div>
  );
}
