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
  text?: string;
  status?: string;
  message?: any;
  message_id?: number;
}

const MAX_RECONNECT_ATTEMPTS = 20;

export function useWebSocket() {
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const reconnectDelay = useRef(2000);
  const reconnectAttempts = useRef(0);
  const subscribedChannels = useRef<Set<number>>(new Set());

  const connect = useCallback(() => {
    if (reconnectAttempts.current >= MAX_RECONNECT_ATTEMPTS) return;

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const ws = new WebSocket(`${protocol}//${window.location.host}/ws`);

    ws.onopen = () => {
      reconnectDelay.current = 2000;
      reconnectAttempts.current = 0;
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
          case 'message_delete':
            if (msg.message_id) {
              msgStore.handleWSDelete(msg.message_id);
            }
            break;
          case 'typing':
            if (msg.channel_id && msg.user_id) {
              const authStore = useAuthStore.getState();
              if (msg.user_id !== authStore.user?.id) {
                msgStore.setTyping(msg.channel_id, msg.user_id, 'User');
              }
            }
            break;
          case 'presence':
            break;
          case 'ai_chunk': {
            const text = msg.text ?? msg.content;
            if (msg.channel_id && text) {
              msgStore.addAIChunk(msg.channel_id, text);
            }
            break;
          }
        }
      } catch {
        // Ignore parse errors
      }
    };

    ws.onclose = () => {
      const delay = reconnectDelay.current;
      reconnectDelay.current = Math.min(delay * 1.5, 30000);
      reconnectAttempts.current++;
      reconnectTimer.current = setTimeout(() => connect(), delay) as unknown as ReturnType<typeof setTimeout>;
    };

    wsRef.current = ws;
  }, []);

  // Subscribe to new channels when they change
  useEffect(() => {
    const unsub = useWorkspaceStore.subscribe((state) => {
      const channels = state.channels || [];
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
    return unsub;
  }, []);

  useEffect(() => {
    connect();
    return () => {
      wsRef.current?.close();
      if (reconnectTimer.current) clearTimeout(reconnectTimer.current as unknown as number);
      subscribedChannels.current.clear();
    };
  }, [connect]);
}
