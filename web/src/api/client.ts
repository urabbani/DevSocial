const API_BASE = '/api';

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    credentials: options?.credentials ?? 'include',
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
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
    if (before !== undefined) params.set('before', String(before));
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

  // Admin
  getSettings: () => request<Record<string, string>>('/admin/settings'),
  updateSettings: (data: Record<string, string>) =>
    request<void>('/admin/settings', { method: 'PATCH', body: JSON.stringify(data) }),
  getModels: () => request<string[]>('/admin/models'),
  getHealth: () => request<Record<string, string>>('/health'),

  // Files
  listFiles: (workspaceId: number) => request<FileInfo[]>(`/workspaces/${workspaceId}/files`),
  getFileUrl: (id: number) => `/api/files/${id}`,
  deleteFile: (id: number) =>
    request<void>(`/files/${id}`, { method: 'DELETE' }),
  uploadFile: async (workspaceId: number, file: File): Promise<FileInfo> => {
    const form = new FormData();
    form.append('file', file);
    form.append('workspace_id', String(workspaceId));
    const res = await fetch('/upload', {
      method: 'POST',
      credentials: 'include',
      body: form,
    });
    if (!res.ok) {
      const body = await res.json().catch(() => ({ error: res.statusText }));
      throw new Error(body.error || `HTTP ${res.status}`);
    }
    return res.json();
  },

  // Feed / Posts
  getFeed: (workspaceId: number, before?: number, limit?: number) => {
    const params = new URLSearchParams();
    if (before !== undefined) params.set('before', String(before));
    if (limit) params.set('limit', String(limit));
    const qs = params.toString();
    return request<Post[]>(`/workspaces/${workspaceId}/feed${qs ? `?${qs}` : ''}`);
  },
  createPost: (workspaceId: number, data: CreatePostInput) =>
    request<Post>(`/workspaces/${workspaceId}/feed`, { method: 'POST', body: JSON.stringify(data) }),
  getPost: (id: number) => request<Post>(`/posts/${id}`),
  togglePostReaction: (postId: number, reaction = 'like') =>
    request<{ like_count: number }>(`/posts/${postId}/reactions`, { method: 'POST', body: JSON.stringify({ reaction }) }),
  getPostReplies: (postId: number) => request<Post[]>(`/posts/${postId}/replies`),
  deletePost: (id: number) =>
    request<void>(`/posts/${id}`, { method: 'DELETE' }),

  // Tasks
  listTasks: (workspaceId: number, status?: string) => {
    const params = new URLSearchParams();
    if (status) params.set('status', status);
    const qs = params.toString();
    return request<Task[]>(`/workspaces/${workspaceId}/tasks${qs ? `?${qs}` : ''}`);
  },
  createTask: (workspaceId: number, data: CreateTaskInput) =>
    request<Task>(`/workspaces/${workspaceId}/tasks`, { method: 'POST', body: JSON.stringify(data) }),
  updateTask: (id: number, data: Partial<UpdateTaskInput>) =>
    request<void>(`/tasks/${id}`, { method: 'PATCH', body: JSON.stringify(data) }),
  deleteTask: (id: number) =>
    request<void>(`/tasks/${id}`, { method: 'DELETE' }),

  // Search
  search: (query: string, workspaceId: number, limit?: number, mode?: 'keyword' | 'semantic' | 'hybrid') => {
    const params = new URLSearchParams({ q: query, workspace_id: String(workspaceId) });
    if (limit) params.set('limit', String(limit));
    if (mode) params.set('mode', mode);
    return request<SearchResult[]>(`/search?${params}`);
  },
  // Re-index embeddings (admin only)
  reindexEmbeddings: () => request<{ status: string }>('/admin/reindex-embeddings', { method: 'POST' }),

  // Notifications
  getNotifications: () => request<Notification[]>('/notifications'),
  getUnreadCount: () => request<{ count: number }>('/notifications/unread-count'),
  markNotificationRead: (id: number) => request<{ status: string }>(`/notifications/${id}/read`, { method: 'PATCH' }),
  markAllNotificationsRead: () => request<{ status: string }>('/notifications/read-all', { method: 'POST' }),

  // Code Documents
  listDocuments: (workspaceId: number) => request<CodeDocument[]>(`/workspaces/${workspaceId}/documents`),
  createDocument: (workspaceId: number, data: CreateDocumentInput) =>
    request<CodeDocument>(`/workspaces/${workspaceId}/documents`, { method: 'POST', body: JSON.stringify(data) }),
  getDocument: (id: number) => request<CodeDocument>(`/documents/${id}`),
  updateDocument: (id: number, data: UpdateDocumentInput) =>
    request<CodeDocument>(`/documents/${id}`, { method: 'PATCH', body: JSON.stringify(data) }),
  deleteDocument: (id: number) =>
    request<void>(`/documents/${id}`, { method: 'DELETE' }),
  executeDocument: (id: number) =>
    request<CodeExecutionResult>(`/documents/${id}/execute`, { method: 'POST' }),
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
  file_ids?: number[];
}

export interface ToolCall {
  id: string;
  name: string;
  arguments: string;
  status: 'pending' | 'executing' | 'completed' | 'error';
  result?: string;
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

export interface FileInfo {
  id: number;
  workspace_id: number;
  uploader_id: number;
  filename: string;
  s3_key: string;
  content_type: string;
  size_bytes: number;
  created_at: string;
  uploader_name?: string;
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

export interface Post {
  id: number;
  workspace_id: number;
  author_id?: number;
  parent_post_id?: number;
  content: string;
  content_html?: string;
  is_ai: boolean;
  post_type: string;
  pinned: boolean;
  created_at: string;
  edited_at?: string;
  author?: CurrentUser;
  reactions?: PostReaction[];
  reply_count: number;
  like_count: number;
}

export interface PostReaction {
  user_id: number;
  username: string;
  reaction: string;
  created_at: string;
}

export interface CreatePostInput {
  content: string;
  post_type?: string;
  parent_post_id?: number;
  attachment_ids?: number[];
}

export interface Task {
  id: number;
  workspace_id: number;
  channel_id?: number;
  creator_id: number;
  assignee_id?: number;
  title: string;
  description: string;
  status: string;
  priority: string;
  due_date?: string;
  created_at: string;
  updated_at: string;
  creator_name?: string;
  assignee_name?: string;
}

export interface CreateTaskInput {
  title: string;
  description?: string;
  priority?: string;
  assignee_id?: number;
  channel_id?: number;
}

export interface UpdateTaskInput {
  title?: string;
  description?: string;
  status?: string;
  priority?: string;
  assignee_id?: number;
}

export interface SearchResult {
  type: 'message' | 'post' | 'task' | 'file';
  id: number;
  title: string;
  preview: string;
  author?: string;
  date: string;
  score?: number; // Relevance score for semantic search (0-1)
}

export interface Notification {
  id: number;
  user_id: number;
  type: 'mention' | 'reaction' | 'task_assigned' | 'post_reply';
  source_user_id?: number;
  source_user?: {
    id: number;
    username: string;
    display_name: string;
    avatar_url: string;
  };
  source_message_id?: number;
  source_post_id?: number;
  source_task_id?: number;
  read: boolean;
  data?: Record<string, any>;
  created_at: string;
}

export interface CodeDocument {
  id: number;
  workspace_id: number;
  title: string;
  filename: string;
  language: string;
  content: string;
  version: number;
  created_by: number;
  created_at: string;
  last_edited_by?: number;
  updated_at: string;
  created_by_name?: string;
  last_edited_by_name?: string;
}

export interface CreateDocumentInput {
  title: string;
  filename: string;
  language?: string;
  content?: string;
}

export interface UpdateDocumentInput {
  title?: string;
  content?: string;
  language?: string;
  version: number;
}

export interface CodeExecutionResult {
  exit_code: number;
  stdout: string;
  stderr: string;
  duration?: string;
}
