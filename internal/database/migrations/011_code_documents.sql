-- Phase 6: Collaborative Code Editor
-- Code documents table for storing workspace code files

CREATE TABLE IF NOT EXISTS code_documents (
    id BIGSERIAL PRIMARY KEY,
    workspace_id BIGINT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    filename TEXT NOT NULL,
    language TEXT NOT NULL DEFAULT 'text',
    content TEXT DEFAULT '',
    version INT NOT NULL DEFAULT 1,
    created_by BIGINT NOT NULL REFERENCES users(id) ON DELETE SET NULL,
    last_edited_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(workspace_id, filename)
);

-- Index for fast workspace document listing
CREATE INDEX IF NOT EXISTS idx_code_documents_workspace ON code_documents(workspace_id);

-- Index for finding recently updated documents
CREATE INDEX IF NOT EXISTS idx_code_documents_updated_at ON code_documents(updated_at DESC);

-- Index for documents by creator
CREATE INDEX IF NOT EXISTS idx_code_documents_created_by ON code_documents(created_by);
