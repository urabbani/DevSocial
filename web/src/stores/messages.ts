import { create } from 'zustand';
import { api, type Message, type ToolCall } from '../api/client';
import type { WSMessage } from '../hooks/useWebSocket';

interface MessageState {
  messagesByChannel: Record<number, Message[]>;
  hasMoreByChannel: Record<number, boolean>;
  loadingByChannel: Record<number, boolean>;
  typingUsers: Record<number, Map<number, string>>; // channelId -> userId -> username
  activeToolCalls: Record<number, ToolCall[]>; // channelId -> tool calls

  fetchMessages: (channelId: number, before?: number) => Promise<void>;
  addMessage: (channelId: number, message: Message) => void;
  addAIChunk: (channelId: number, text: string) => void;
  clearAIStream: (channelId: number) => void;
  removeMessage: (channelId: number, messageId: number) => void;
  updateMessage: (channelId: number, messageId: number, content: string) => void;
  upsertToolCall: (channelId: number, toolCall: ToolCall) => void;
  handleWSMessage: (msg: WSMessage) => void;
  handleWSDelete: (messageId: number) => void;
  setTyping: (channelId: number, userId: number, username: string) => void;
  clearTyping: (channelId: number, userId: number) => void;
}

export const useMessageStore = create<MessageState>((set, get) => ({
  messagesByChannel: {},
  hasMoreByChannel: {},
  loadingByChannel: {},
  typingUsers: {},
  activeToolCalls: {},

  fetchMessages: async (channelId: number, before?: number) => {
    set((s) => ({ loadingByChannel: { ...s.loadingByChannel, [channelId]: true } }));
    try {
      const existing = get().messagesByChannel[channelId] || [];
      const oldestId = before !== undefined ? before : (existing.length > 0 ? existing[0].id : undefined);
      const messages = await api.listMessages(channelId, oldestId);
      const hasMore = messages.length >= 50;
      const chronological = [...messages].reverse();
      set((s) => ({
        messagesByChannel: {
          ...s.messagesByChannel,
          [channelId]: before !== undefined ? [...chronological, ...existing] : chronological,
        },
        hasMoreByChannel: { ...s.hasMoreByChannel, [channelId]: hasMore },
        loadingByChannel: { ...s.loadingByChannel, [channelId]: false },
      }));
    } catch {
      set((s) => ({ loadingByChannel: { ...s.loadingByChannel, [channelId]: false } }));
    }
  },

  addMessage: (channelId: number, message: Message) => {
    set((s) => {
      const existing = s.messagesByChannel[channelId] || [];
      // Deduplicate: skip if message with same ID already exists
      if (message.id > 0 && existing.some((m) => m.id === message.id)) {
        return s;
      }
      return {
        messagesByChannel: {
          ...s.messagesByChannel,
          [channelId]: [...existing, message],
        },
      };
    });
  },

  // Accumulate AI streaming chunks into a single synthetic message
  addAIChunk: (channelId: number, text: string) => {
    set((s) => {
      const existing = s.messagesByChannel[channelId] || [];
      // Find or create the streaming message (id === -1 is our sentinel)
      const streamIdx = existing.findIndex((m) => m.id === -1);
      const updated = [...existing];
      if (streamIdx >= 0) {
        updated[streamIdx] = { ...updated[streamIdx], content: updated[streamIdx].content + text };
      } else {
        updated.push({
          id: -1,
          channel_id: channelId,
          content: text,
          is_ai: true,
          is_system: false,
          created_at: new Date().toISOString(),
        });
      }
      return {
        messagesByChannel: { ...s.messagesByChannel, [channelId]: updated },
      };
    });
  },

  // Clear the AI streaming sentinel message (id=-1)
  clearAIStream: (channelId: number) => {
    set((s) => ({
      messagesByChannel: {
        ...s.messagesByChannel,
        [channelId]: (s.messagesByChannel[channelId] || []).filter((m) => m.id !== -1),
      },
      activeToolCalls: {
        ...s.activeToolCalls,
        [channelId]: [], // Clear tool calls too
      },
    }));
  },

  // Add or update a tool call for a channel
  upsertToolCall: (channelId: number, toolCall: ToolCall) => {
    set((s) => {
      const existing = s.activeToolCalls[channelId] || [];
      const idx = existing.findIndex((tc) => tc.id === toolCall.id);
      const updated = [...existing];
      if (idx >= 0) {
        updated[idx] = toolCall;
      } else {
        updated.push(toolCall);
      }
      return {
        activeToolCalls: { ...s.activeToolCalls, [channelId]: updated },
      };
    });
  },

  removeMessage: (channelId: number, messageId: number) => {
    set((s) => ({
      messagesByChannel: {
        ...s.messagesByChannel,
        [channelId]: (s.messagesByChannel[channelId] || []).filter((m) => m.id !== messageId),
      },
    }));
  },

  updateMessage: (channelId: number, messageId: number, content: string) => {
    set((s) => ({
      messagesByChannel: {
        ...s.messagesByChannel,
        [channelId]: (s.messagesByChannel[channelId] || []).map((m) =>
          m.id === messageId ? { ...m, content, edited_at: new Date().toISOString() } : m
        ),
      },
    }));
  },

  handleWSMessage: (msg: WSMessage) => {
    if (msg.type === 'message' && msg.message) {
      const m = msg.message as Message;
      get().addMessage(m.channel_id, m);
    } else if (msg.type === 'ai_chunk' && msg.text !== undefined) {
      if (msg.text === '') {
        // Empty text signals clear
        get().clearAIStream(msg.channel_id);
      } else {
        get().addAIChunk(msg.channel_id, msg.text);
      }
    } else if (msg.type === 'ai_stream_done') {
      get().clearAIStream(msg.channel_id);
    } else if (msg.type === 'tool_call' && msg.tool_call) {
      get().upsertToolCall(msg.channel_id, msg.tool_call);
    } else if (msg.type === 'tool_result' && msg.tool_call) {
      get().upsertToolCall(msg.channel_id, msg.tool_call);
    }
  },

  handleWSDelete: (messageId: number) => {
    const state = get();
    for (const [channelId, msgs] of Object.entries(state.messagesByChannel)) {
      if (msgs.some((m) => m.id === messageId)) {
        get().removeMessage(Number(channelId), messageId);
        break;
      }
    }
  },

  setTyping: (channelId: number, userId: number, username: string) => {
    const typingUsers = { ...get().typingUsers };
    typingUsers[channelId] = new Map(typingUsers[channelId] || []);
    typingUsers[channelId].set(userId, username);
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
