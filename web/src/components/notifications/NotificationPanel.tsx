import { useEffect } from 'react';
import { useNotificationStore } from '../../stores/notifications';
import type { Notification } from '../../api/client';

interface NotificationPanelProps {
  onClose: () => void;
}

const NOTIFICATION_ICONS: Record<string, string> = {
  mention: '&#128101;', // 💬
  reaction: '&#128078;', // ❤️
  task_assigned: '&#9744;', // ☑
  post_reply: '&#8620;',  // 💬
};

const NOTIFICATION_LABELS: Record<string, string> = {
  mention: 'mentioned you',
  reaction: 'reacted to your post',
  task_assigned: 'assigned you a task',
  post_reply: 'replied to your post',
};

function formatTime(iso: string): string {
  const seconds = Math.floor((Date.now() - new Date(iso).getTime()) / 1000);
  if (seconds < 60) return 'just now';
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  return new Date(iso).toLocaleDateString();
}

export function NotificationPanel({ onClose }: NotificationPanelProps) {
  const { notifications, markAsRead } = useNotificationStore();

  // Close panel when clicking outside
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (e.target instanceof Element && !e.target.closest('.notification-panel')) {
        onClose();
      }
    };
    document.addEventListener('click', handleClickOutside);
    return () => document.removeEventListener('click', handleClickOutside);
  }, [onClose]);

  const handleNotificationClick = (notif: Notification) => {
    if (!notif.read) {
      markAsRead(notif.id);
    }
    // TODO: Navigate to related content (message, post, task)
    onClose();
  };

  if (notifications.length === 0) {
    return (
      <div className="notification-panel p-4">
        <div className="text-center text-sm text-[var(--text-muted)] py-8">
          No notifications yet
        </div>
      </div>
    );
  }

  return (
    <div className="notification-panel">
      {/* Header */}
      <div className="px-4 py-3 border-b border-[var(--border)] flex items-center justify-between">
        <h3 className="text-sm font-medium text-[var(--text-primary)]">Notifications</h3>
        <button
          onClick={onClose}
          className="text-[var(--text-muted)] hover:text-[var(--text-primary)] text-sm"
        >
          ×
        </button>
      </div>

      {/* Notifications List */}
      <div className="max-h-96 overflow-y-auto">
        {notifications.map((notif) => (
          <div
            key={notif.id}
            onClick={() => handleNotificationClick(notif)}
            className={`px-4 py-3 border-b border-[var(--border)] hover:bg-[var(--bg-hover)] cursor-pointer transition-colors ${
              !notif.read ? 'bg-[var(--bg-tertiary)]/30' : ''
            }`}
          >
            <div className="flex items-start gap-3">
              {/* Icon */}
              <span className="text-lg mt-0.5">
                {NOTIFICATION_ICONS[notif.type] || '&#128276;'}
              </span>

              {/* Content */}
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2 mb-0.5">
                  {notif.source_user && (
                    <span className="text-sm font-medium text-[var(--text-primary)]">
                      {notif.source_user.display_name || notif.source_user.username}
                    </span>
                  )}
                  <span className="text-xs text-[var(--text-muted)]">
                    {formatTime(notif.created_at)}
                  </span>
                  {!notif.read && (
                    <span className="text-[10px] px-1.5 py-0.5 bg-[var(--accent)] text-white rounded-full">
                      New
                    </span>
                  )}
                </div>

                <p className="text-xs text-[var(--text-secondary)]">
                  {NOTIFICATION_LABELS[notif.type] || 'notification'}
                </p>

                {/* Additional context */}
                {notif.data?.mention_text && (
                  <p className="text-xs text-[var(--text-muted)] mt-1 line-clamp-2 italic">
                    "{notif.data.mention_text}"
                  </p>
                )}
                {notif.data?.task_title && (
                  <p className="text-xs text-[var(--text-muted)] mt-1">
                    Task: {notif.data.task_title}
                  </p>
                )}
                {notif.data?.post_preview && (
                  <p className="text-xs text-[var(--text-muted)] mt-1 line-clamp-2">
                    "{notif.data.post_preview}"
                  </p>
                )}
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
