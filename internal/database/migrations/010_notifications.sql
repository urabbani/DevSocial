-- Notifications table for real-time alerts
CREATE TABLE IF NOT EXISTS notifications (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  type TEXT NOT NULL, -- 'mention', 'reaction', 'task_assigned', 'post_reply'
  source_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
  source_message_id BIGINT REFERENCES messages(id) ON DELETE CASCADE,
  source_post_id BIGINT REFERENCES posts(id) ON DELETE CASCADE,
  source_task_id BIGINT REFERENCES tasks(id) ON DELETE CASCADE,
  read BOOLEAN DEFAULT FALSE,
  data JSONB DEFAULT '{}'::jsonb, -- Additional metadata (reaction type, mention text, etc.)
  created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Unique constraint to prevent duplicate notifications
-- e.g., same user mentioning the same person in the same message
CREATE UNIQUE INDEX notifications_unique_idx
ON notifications(user_id, type, source_user_id,
  COALESCE(source_message_id, 0),
  COALESCE(source_post_id, 0),
  COALESCE(source_task_id, 0)
WHERE NOT (type = 'reaction'); -- Allow multiple reactions from different users

-- Index for fetching user notifications
CREATE INDEX notifications_user_idx ON notifications(user_id, created_at DESC);

-- Index for unread count queries
CREATE INDEX notifications_unread_idx ON notifications(user_id) WHERE read = FALSE;

-- Index for cleanup of old read notifications
CREATE INDEX notifications_read_created_idx ON notifications(read, created_at);

-- Comment for documentation
COMMENT ON TABLE notifications IS 'User notifications for mentions, reactions, assignments, and replies';
COMMENT ON COLUMN notifications.type IS 'Notification type: mention, reaction, task_assigned, post_reply';
COMMENT ON COLUMN notifications.data IS 'Additional metadata like reaction emoji, mention text, etc.';
