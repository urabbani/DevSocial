import { create } from 'zustand';
import { api, type Task } from '../api/client';

interface TaskState {
  tasks: Task[];
  loading: boolean;
  statusFilter: string;

  setFilter: (status: string) => void;
  fetchTasks: (workspaceId: number) => Promise<void>;
  createTask: (workspaceId: number, data: { title: string; description?: string; priority?: string }) => Promise<Task>;
  updateTask: (taskId: number, data: { title?: string; description?: string; status?: string; priority?: string; assignee_id?: number }) => Promise<void>;
  deleteTask: (taskId: number) => Promise<void>;
}

export const useTaskStore = create<TaskState>((set, get) => ({
  tasks: [],
  loading: false,
  statusFilter: '',

  setFilter: (status: string) => {
    set({ statusFilter: status });
  },

  fetchTasks: async (workspaceId: number) => {
    set({ loading: true });
    try {
      const filter = get().statusFilter;
      const tasks = await api.listTasks(workspaceId, filter || undefined);
      set({ tasks, loading: false });
    } catch {
      set({ loading: false });
    }
  },

  createTask: async (workspaceId: number, data: { title: string; description?: string; priority?: string }) => {
    const task = await api.createTask(workspaceId, data);
    set((s) => ({ tasks: [task, ...s.tasks] }));
    return task;
  },

  updateTask: async (taskId: number, data: { title?: string; description?: string; status?: string; priority?: string; assignee_id?: number }) => {
    await api.updateTask(taskId, data);
    set((s) => ({
      tasks: s.tasks.map((t) => t.id === taskId ? { ...t, ...data, updated_at: new Date().toISOString() } : t),
    }));
  },

  deleteTask: async (taskId: number) => {
    await api.deleteTask(taskId);
    set((s) => ({ tasks: s.tasks.filter((t) => t.id !== taskId) }));
  },
}));
