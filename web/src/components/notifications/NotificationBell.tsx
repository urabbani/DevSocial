import { useEffect, useState } from 'react';
import { useNotificationStore } from '../../stores/notifications';
import { NotificationPanel } from './NotificationPanel';

export function NotificationBell() {
  const { unreadCount, fetchNotifications, markAllAsRead } = useNotificationStore();
  const [isOpen, setIsOpen] = useState(false);
  const [hasFetched, setHasFetched] = useState(false);

  // Fetch notifications on first mount
  useEffect(() => {
    if (!hasFetched) {
      fetchNotifications();
      setHasFetched(true);
    }
  }, [hasFetched, fetchNotifications]);

  // Poll for unread count every 30 seconds
  useEffect(() => {
    const interval = setInterval(() => {
      fetchUnreadCount();
    }, 30000);
    return () => clearInterval(interval);
  }, []);

  const fetchUnreadCount = async () => {
    try {
      await useNotificationStore.getState().fetchUnreadCount();
    } catch {
      // Silent fail
    }
  };

  const handleClick = () => {
    setIsOpen(!isOpen);
    if (isOpen) {
      // Mark all as read when closing
      if (unreadCount > 0) {
        markAllAsRead();
      }
    }
  };

  return (
    <div className="relative">
      <button
        onClick={handleClick}
        className="relative p-2 text-[var(--text-secondary)] hover:text-[var(--text-primary)] transition-colors"
        aria-label="Notifications"
      >
        <span className="text-xl">&#128276;</span>
        {unreadCount > 0 && (
          <span className="absolute -top-1 -right-1 bg-[var(--accent)] text-white text-xs rounded-full w-5 h-5 flex items-center justify-center">
            {unreadCount > 99 ? '99+' : unreadCount}
          </span>
        )}
      </button>

      {isOpen && (
        <div className="absolute right-0 top-full mt-2 w-96 bg-[var(--bg-secondary)] border border-[var(--border)] rounded-lg shadow-lg z-50">
          <NotificationPanel onClose={() => setIsOpen(false)} />
        </div>
      )}
    </div>
  );
}
