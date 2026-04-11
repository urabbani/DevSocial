-- 006: File Storage

CREATE TABLE files (
    id           BIGSERIAL PRIMARY KEY,
    workspace_id BIGINT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    uploader_id  BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    filename     TEXT NOT NULL,
    s3_key       TEXT NOT NULL,
    content_type TEXT NOT NULL DEFAULT 'application/octet-stream',
    size_bytes   BIGINT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_files_workspace ON files(workspace_id);
CREATE INDEX idx_files_uploader ON files(uploader_id);

-- Add file attachment support to messages
ALTER TABLE messages ADD COLUMN IF NOT EXISTS file_ids BIGINT[] DEFAULT '{}';
