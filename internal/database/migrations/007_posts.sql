-- 007: Social Feed (Posts)

CREATE TABLE posts (
    id             BIGSERIAL PRIMARY KEY,
    workspace_id   BIGINT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    author_id      BIGINT REFERENCES users(id) ON DELETE SET NULL,
    parent_post_id BIGINT REFERENCES posts(id) ON DELETE CASCADE,
    content        TEXT NOT NULL,
    content_html   TEXT,
    is_ai          BOOLEAN NOT NULL DEFAULT false,
    post_type      TEXT NOT NULL DEFAULT 'discussion' CHECK (post_type IN ('discussion', 'share', 'analysis', 'code', 'finding', 'task')),
    pinned         BOOLEAN NOT NULL DEFAULT false,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    edited_at      TIMESTAMPTZ
);

CREATE TABLE post_reactions (
    post_id    BIGINT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reaction   TEXT NOT NULL DEFAULT 'like',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (post_id, user_id, reaction)
);

CREATE TABLE post_attachments (
    id       BIGSERIAL PRIMARY KEY,
    post_id  BIGINT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    file_id  BIGINT NOT NULL REFERENCES files(id) ON DELETE CASCADE
);

CREATE INDEX idx_posts_workspace ON posts(workspace_id, created_at DESC);
CREATE INDEX idx_posts_author ON posts(author_id);
CREATE INDEX idx_posts_parent ON posts(parent_post_id);
CREATE INDEX idx_posts_type ON posts(post_type);
CREATE INDEX idx_post_attachments_post ON post_attachments(post_id);
