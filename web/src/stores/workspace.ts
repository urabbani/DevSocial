import { create } from 'zustand';
import { api, type Workspace, type Channel } from '../api/client';

interface WorkspaceState {
  workspaces: Workspace[];
  activeWorkspace: Workspace | null;
  channels: Channel[];
  activeChannel: Channel | null;
  loading: boolean;
  fetchWorkspaces: () => Promise<void>;
  setActiveWorkspace: (ws: Workspace | null) => Promise<void>;
  createWorkspace: (name: string, description?: string) => Promise<void>;
  setActiveChannel: (ch: Channel | null) => void;
  createChannel: (name: string, type?: 'text' | 'ai') => Promise<void>;
}

export const useWorkspaceStore = create<WorkspaceState>((set, get) => ({
  workspaces: [],
  activeWorkspace: null,
  channels: [],
  activeChannel: null,
  loading: false,

  fetchWorkspaces: async () => {
    const workspaces = await api.listWorkspaces();
    set({ workspaces });
    // Auto-select first workspace if none selected
    if (!get().activeWorkspace && workspaces.length > 0) {
      get().setActiveWorkspace(workspaces[0]);
    }
  },

  setActiveWorkspace: async (ws: Workspace | null) => {
    set({ activeWorkspace: ws, activeChannel: null, channels: [], loading: !!ws });
    if (ws) {
      const channels = await api.listChannels(ws.id);
      set({ channels, loading: false });
      // Auto-select #general
      const general = channels.find((c) => c.name === 'general');
      if (general) set({ activeChannel: general });
    }
  },

  createWorkspace: async (name: string, description?: string) => {
    const ws = await api.createWorkspace({ name, description });
    set((s) => ({ workspaces: [...s.workspaces, ws] }));
    get().setActiveWorkspace(ws);
  },

  setActiveChannel: (ch: Channel | null) => set({ activeChannel: ch }),

  createChannel: async (name: string, type?: 'text' | 'ai') => {
    const ws = get().activeWorkspace;
    if (!ws) return;
    const ch = await api.createChannel(ws.id, { name, type });
    set((s) => ({ channels: [...s.channels, ch].sort((a, b) => a.position - b.position) }));
  },
}));
