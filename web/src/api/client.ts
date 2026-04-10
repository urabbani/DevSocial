const API_BASE = '/api';

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    ...options,
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(body.error || `HTTP ${res.status}`);
  }
  return res.json();
}

export const api = {
  // Auth
  getMe: () => request<CurrentUser>('/me'),

  // Workspaces
  listWorkspaces: () => request<Workspace[]>('/workspaces'),
  createWorkspace: (data: CreateWorkspaceInput) =>
    request<Workspace>('/workspaces', { method: 'POST', body: JSON.stringify(data) }),
  getWorkspace: (id: number) => request<Workspace>(`/workspaces/${id}`),
  updateWorkspace: (id: number, data: Partial<Workspace>) =>
    request<void>(`/workspaces/${id}`, { method: 'PATCH', body: JSON.stringify(data) }),

  // Members
  listMembers: (workspaceId: number) => request<WorkspaceMember[]>(`/workspaces/${workspaceId}/members`),
  addMember: (workspaceId: number, userId: number) =>
    request<void>(`/workspaces/${workspaceId}/members`, { method: 'POST', body: JSON.stringify({ user_id: userId }) }),
  removeMember: (workspaceId: number, userId: number) =>
    request<void>(`/workspaces/${workspaceId}/members/${userId}`, { method: 'DELETE' }),

  // Channels
  listChannels: (workspaceId: number) => request<Channel[]>(`/workspaces/${workspaceId}/channels`),
  createChannel: (workspaceId: number, data: CreateChannelInput) =>
    request<Channel>(`/workspaces/${workspaceId}/channels`, { method: 'POST', body: JSON.stringify(data) }),
  updateChannel: (id: number, data: Partial<Channel>) =>
    request<void>(`/channels/${id}`, { method: 'PATCH', body: JSON.stringify(data) }),
  deleteChannel: (id: number) =>
    request<void>(`/channels/${id}`, { method: 'DELETE' }),

  // Messages
  listMessages: (channelId: number, before?: number, limit?: number) => {
    const params = new URLSearchParams();
    if (before) params.set('before', String(before));
    if (limit) params.set('limit', String(limit));
    const qs = params.toString();
    return request<Message[]>(`/channels/${channelId}/messages${qs ? `?${qs}` : ''}`);
  },
  getMessage: (id: number) => request<Message>(`/messages/${id}`),
  editMessage: (id: number, content: string) =>
    request<void>(`/messages/${id}`, { method: 'PATCH', body: JSON.stringify({ content }) }),
  deleteMessage: (id: number) =>
    request<void>(`/messages/${id}`, { method: 'DELETE' }),

  // Reactions
  toggleReaction: (messageId: number, reaction: string) =>
    request<Reaction[]>(`/messages/${messageId}/reactions`, { method: 'POST', body: JSON.stringify({ reaction }) }),

  // AI Agents
  listAgents: (workspaceId: number) => request<AIAgent[]>(`/workspaces/${workspaceId}/agents`),
  createAgent: (workspaceId: number, data: CreateAgentInput) =>
    request<AIAgent>(`/workspaces/${workspaceId}/agents`, { method: 'POST', body: JSON.stringify(data) }),
  updateAgent: (id: number, data: Partial<AIAgent>) =>
    request<void>(`/agents/${id}`, { method: 'PATCH', body: JSON.stringify(data) }),
};

// Types
export interface CurrentUser {
  id: number;
  github_id: number;
  username: string;
  display_name: string;
  avatar_url: string;
  bio: string;
  created_at: string;
}

export interface Workspace {
  id: number;
  name: string;
  description: string;
  slug: string;
  created_at: string;
  member_count: number;
  is_member: boolean;
}

export interface Channel {
  id: number;
  workspace_id: number;
  name: string;
  description: string;
  type: 'text' | 'ai';
  position: number;
  created_at: string;
  unread_count: number;
}

export interface Message {
  id: number;
  channel_id: number;
  author_id?: number;
  parent_message_id?: number;
  content: string;
  content_html?: string;
  is_ai: boolean;
  is_system: boolean;
  created_at: string;
  edited_at?: string;
  author?: CurrentUser;
  reactions?: Reaction[];
}

export interface Reaction {
  user_id: number;
  username: string;
  reaction: string;
  created_at: string;
}

export interface AIAgent {
  id: number;
  workspace_id: number;
  name: string;
  type: string;
  system_prompt: string;
  enabled: boolean;
  created_at: string;
}

export interface WorkspaceMember {
  user_id: number;
  username: string;
  role: 'owner' | 'admin' | 'member';
  joined_at: string;
}

export interface CreateWorkspaceInput {
  name: string;
  description?: string;
  slug?: string;
}

export interface CreateChannelInput {
  name: string;
  description?: string;
  type?: 'text' | 'ai';
  position?: number;
}

export interface CreateAgentInput {
  name: string;
  type?: string;
  system_prompt?: string;
}
