import { create } from 'zustand';
import { api, type Message } from '../api/client';
import type { WSMessage } from '../hooks/useWebSocket';

interface MessageState {
  messagesByChannel: Record<number, Message[]>;
  hasMoreByChannel: Record<number, boolean>;
  loadingByChannel: Record<number, boolean>;
  typingUsers: Record<number, Map<number, string>>; // channelId -> userId -> username

  fetchMessages: (channelId: number, before?: number) => Promise<void>;
  addMessage: (channelId: number, message: Message) => void;
  removeMessage: (channelId: number, messageId: number) => void;
  updateMessage: (channelId: number, messageId: number, content: string, contentHtml: string) => void;
  handleWSMessage: (msg: WSMessage) => void;
  setTyping: (channelId: number, userId: number, username: string) => void;
  clearTyping: (channelId: number, userId: number) => void;
}

export const useMessageStore = create<MessageState>((set, get) => ({
  messagesByChannel: {},
  hasMoreByChannel: {},
  loadingByChannel: {},
  typingUsers: {},

  fetchMessages: async (channelId: number, before?: number) => {
    set((s) => ({ loadingByChannel: { ...s.loadingByChannel, [channelId]: true } }));
    const existing = get().messagesByChannel[channelId] || [];
    const oldestId = before ?? (existing.length > 0 ? existing[0].id : 0);
    const messages = await api.listMessages(channelId, oldestId || undefined);
    const hasMore = messages.length >= 50;
    // Messages come newest first from API, we want chronological
    const chronological = [...messages].reverse();
    set((s) => ({
      messagesByChannel: {
        ...s.messagesByChannel,
        [channelId]: before ? [...chronological, ...existing] : chronological,
      },
      hasMoreByChannel: { ...s.hasMoreByChannel, [channelId]: hasMore },
      loadingByChannel: { ...s.loadingByChannel, [channelId]: false },
    }));
  },

  addMessage: (channelId: number, message: Message) => {
    set((s) => ({
      messagesByChannel: {
        ...s.messagesByChannel,
        [channelId]: [...(s.messagesByChannel[channelId] || []), message],
      },
    }));
  },

  removeMessage: (channelId: number, messageId: number) => {
    set((s) => ({
      messagesByChannel: {
        ...s.messagesByChannel,
        [channelId]: (s.messagesByChannel[channelId] || []).filter((m) => m.id !== messageId),
      },
    }));
  },

  updateMessage: (channelId: number, messageId: number, content: string, contentHtml: string) => {
    set((s) => ({
      messagesByChannel: {
        ...s.messagesByChannel,
        [channelId]: (s.messagesByChannel[channelId] || []).map((m) =>
          m.id === messageId ? { ...m, content, content_html: contentHtml, edited_at: new Date().toISOString() } : m
        ),
      },
    }));
  },

  handleWSMessage: (msg: WSMessage) => {
    if (msg.type === 'message' && msg.message) {
      const m = msg.message as Message;
      get().addMessage(m.channel_id, m);
    } else if (msg.type === 'message_delete' && msg.message_id) {
      // Handled when we add this event type
    }
  },

  setTyping: (channelId: number, userId: number, username: string) => {
    const typingUsers = { ...get().typingUsers };
    typingUsers[channelId] = new Map(typingUsers[channelId] || []);
    typingUsers[channelId].set(userId, username);
    // Auto-clear after 3s
    setTimeout(() => get().clearTyping(channelId, userId), 3000);
    set({ typingUsers });
  },

  clearTyping: (channelId: number, userId: number) => {
    const typingUsers = { ...get().typingUsers };
    if (typingUsers[channelId]) {
      typingUsers[channelId].delete(userId);
      if (typingUsers[channelId].size === 0) delete typingUsers[channelId];
    }
    set({ typingUsers });
  },
}));
