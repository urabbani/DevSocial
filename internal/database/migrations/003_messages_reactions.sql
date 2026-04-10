-- 003: Messages and Reactions

CREATE TABLE messages (
    id                BIGSERIAL PRIMARY KEY,
    channel_id        BIGINT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    author_id         BIGINT REFERENCES users(id) ON DELETE SET NULL,
    parent_message_id BIGINT REFERENCES messages(id) ON DELETE CASCADE,
    content           TEXT NOT NULL,
    content_html      TEXT,
    is_ai             BOOLEAN NOT NULL DEFAULT false,
    is_system         BOOLEAN NOT NULL DEFAULT false,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    edited_at         TIMESTAMPTZ
);

CREATE TABLE message_reactions (
    message_id BIGINT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reaction   TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (message_id, user_id, reaction)
);

CREATE TABLE channel_unreads (
    user_id              BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    channel_id           BIGINT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    last_read_message_id BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, channel_id)
);

CREATE INDEX idx_messages_channel ON messages(channel_id, created_at);
CREATE INDEX idx_messages_parent ON messages(parent_message_id);
CREATE INDEX idx_messages_author ON messages(author_id);
