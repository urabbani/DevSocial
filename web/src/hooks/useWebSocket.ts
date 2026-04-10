import { useEffect, useRef, useCallback } from 'react';
import { useWorkspaceStore } from '../stores/workspace';
import { useMessageStore } from '../stores/messages';
import { useAuthStore } from '../stores/auth';

export interface WSMessage {
  type: string;
  channel_id?: number;
  channel_ids?: number[];
  user_id?: number;
  content?: string;
  status?: string;
  message?: any;
  message_id?: number;
}

export function useWebSocket() {
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const subscribedChannels = useRef<Set<number>>(new Set());

  const connect = useCallback(() => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const ws = new WebSocket(`${protocol}//${window.location.host}/ws`);

    ws.onopen = () => {
      // Subscribe to all active workspace channels
      const channels = useWorkspaceStore.getState().channels || [];
      if (channels.length > 0) {
        ws.send(JSON.stringify({
          type: 'subscribe',
          channel_ids: channels.map((c) => c.id),
        }));
        channels.forEach((c) => subscribedChannels.current.add(c.id));
      }
    };

    ws.onmessage = (event) => {
      try {
        const msg: WSMessage = JSON.parse(event.data);
        const msgStore = useMessageStore.getState();

        switch (msg.type) {
          case 'message':
            msgStore.handleWSMessage(msg);
            break;
          case 'typing':
            if (msg.channel_id && msg.user_id) {
              const authStore = useAuthStore.getState();
              if (msg.user_id !== authStore.user?.id) {
                // Fetch username from message store or cache
                msgStore.setTyping(msg.channel_id, msg.user_id, 'User');
              }
            }
            break;
          case 'presence':
            // Could update user presence indicator
            break;
          case 'ai_chunk':
            // Stream AI response chunks
            if (msg.channel_id && msg.content) {
              msgStore.addMessage(msg.channel_id, {
                id: 0,
                channel_id: msg.channel_id,
                content: msg.content,
                is_ai: true,
                is_system: false,
                created_at: new Date().toISOString(),
              });
            }
            break;
        }
      } catch {
        // Ignore parse errors
      }
    };

    ws.onclose = () => {
      // Auto-reconnect with exponential backoff
      reconnectTimer.current = setTimeout(() => connect(), 2000) as unknown as ReturnType<typeof setTimeout>;
    };

    wsRef.current = ws;
  }, []);

  // Subscribe to new channels when they change
  useEffect(() => {
    const channels = useWorkspaceStore.getState().channels || [];
    const newChannelIds = channels
      .map((c) => c.id)
      .filter((id) => !subscribedChannels.current.has(id));

    if (newChannelIds.length > 0 && wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(
        JSON.stringify({ type: 'subscribe', channel_ids: newChannelIds })
      );
      newChannelIds.forEach((id) => subscribedChannels.current.add(id));
    }
  });

  const sendMessage = useCallback((channelId: number, content: string) => {
    if (wsRef.current?.readyState !== WebSocket.OPEN) return;
    wsRef.current.send(JSON.stringify({ type: 'chat', channel_id: channelId, content }));
  }, []);

  const sendTyping = useCallback((channelId: number) => {
    if (wsRef.current?.readyState !== WebSocket.OPEN) return;
    wsRef.current.send(JSON.stringify({ type: 'typing', channel_id: channelId }));
  }, []);

  useEffect(() => {
    connect();
    return () => {
      wsRef.current?.close();
      if (reconnectTimer.current) clearTimeout(reconnectTimer.current as unknown as number);
    };
  }, [connect]);

  return { sendMessage, sendTyping };
}

export async function postMessage(channelId: number, content: string) {
  const res = await fetch('/api/channels/' + channelId + '/messages', {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ content }),
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: 'Failed to send message' }));
    throw new Error(body.error);
  }
  return res.json();
}

export async function sendMessageViaAPI(channelId: number, content: string, isAI?: boolean) {
  // This will be handled by the WebSocket flow in production.
  // For now, post directly via REST.
  const res = await fetch('/api/channels/' + channelId + '/messages', {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ content, is_ai: isAI }),
  });
  if (!res.ok) throw new Error('Failed to send message');
  return res.json();
}
