import { create } from 'zustand';
import { api, type Notification } from '../api/client';

interface NotificationState {
  notifications: Notification[];
  unreadCount: number;
  loading: boolean;

  fetchNotifications: () => Promise<void>;
  markAsRead: (id: number) => Promise<void>;
  markAllAsRead: () => Promise<void>;
  fetchUnreadCount: () => Promise<number>;
  handleWSNotification: (notification: Notification) => void;
}

export const useNotificationStore = create<NotificationState>((set) => ({
  notifications: [],
  unreadCount: 0,
  loading: false,

  fetchNotifications: async () => {
    set({ loading: true });
    try {
      const notifications = await api.getNotifications();
      const unread = notifications.filter((n) => !n.read).length;
      set({ notifications, unreadCount: unread, loading: false });
    } catch {
      set({ loading: false });
    }
  },

  markAsRead: async (id: number) => {
    try {
      await api.markNotificationRead(id);
      set((s) => ({
        notifications: s.notifications.map((n) => (n.id === id ? { ...n, read: true } : n)),
        unreadCount: Math.max(0, s.unreadCount - 1),
      }));
    } catch {
      // Silent fail - user can retry
    }
  },

  markAllAsRead: async () => {
    try {
      await api.markAllNotificationsRead();
      set((s) => ({
        notifications: s.notifications.map((n) => ({ ...n, read: true })),
        unreadCount: 0,
      }));
    } catch {
      // Silent fail - user can retry
    }
  },

  fetchUnreadCount: async () => {
    try {
      const result = await api.getUnreadCount();
      const count = result.count as number;
      set({ unreadCount: count });
      return count;
    } catch {
      return 0;
    }
  },

  handleWSNotification: (notification: Notification) => {
    set((s) => {
      // Add to beginning of list
      const newNotifications = [notification, ...s.notifications];
      // Increment unread count if not read
      const newUnreadCount = notification.read ? s.unreadCount : s.unreadCount + 1;
      return {
        notifications: newNotifications,
        unreadCount: newUnreadCount,
      };
    });
  },
}));
