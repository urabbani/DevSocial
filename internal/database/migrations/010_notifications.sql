-- Notifications table for real-time alerts
CREATE TABLE IF NOT EXISTS notifications (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  type TEXT NOT NULL,
  source_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
  source_message_id BIGINT REFERENCES messages(id) ON DELETE CASCADE,
  source_post_id BIGINT REFERENCES posts(id) ON DELETE CASCADE,
  source_task_id BIGINT REFERENCES tasks(id) ON DELETE CASCADE,
  read BOOLEAN DEFAULT FALSE,
  data JSONB DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Index for fetching user notifications
CREATE INDEX IF NOT EXISTS notifications_user_idx ON notifications(user_id, created_at DESC);

-- Index for unread count queries
CREATE INDEX IF NOT EXISTS notifications_unread_idx ON notifications(user_id) WHERE read = FALSE;

-- Index for cleanup of old read notifications
CREATE INDEX IF NOT EXISTS notifications_read_created_idx ON notifications(read, created_at);

-- Comment for documentation
COMMENT ON TABLE notifications IS 'User notifications for mentions, reactions, assignments, and replies';
COMMENT ON COLUMN notifications.type IS 'Notification type: mention, reaction, task_assigned, post_reply';
COMMENT ON COLUMN notifications.data IS 'Additional metadata like reaction emoji, mention text, etc.';
