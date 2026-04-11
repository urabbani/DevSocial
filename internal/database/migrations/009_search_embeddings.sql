-- Enable pgvector extension
CREATE EXTENSION IF NOT EXISTS vector;

-- Add embedding columns for semantic search
-- Using 384 dimensions (common for LiteLLM default embedding models)
ALTER TABLE messages ADD COLUMN IF NOT EXISTS embedding vector(384);
ALTER TABLE posts ADD COLUMN IF NOT EXISTS embedding vector(384);
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS embedding vector(384);
ALTER TABLE files ADD COLUMN IF NOT EXISTS embedding vector(384);

-- Create HNSW indexes for fast approximate nearest neighbor search
-- M=16 (connectivity), ef_construction=64 (index build accuracy)
CREATE INDEX IF NOT EXISTS messages_embedding_idx ON messages USING hnsw (embedding vector_cosine_ops) WITH (m = 16, ef_construction = 64);
CREATE INDEX IF NOT EXISTS posts_embedding_idx ON posts USING hnsw (embedding vector_cosine_ops) WITH (m = 16, ef_construction = 64);
CREATE INDEX IF NOT EXISTS tasks_embedding_idx ON tasks USING hnsw (embedding vector_cosine_ops) WITH (m = 16, ef_construction = 64);
CREATE INDEX IF NOT EXISTS files_embedding_idx ON files USING hnsw (embedding vector_cosine_ops) WITH (m = 16, ef_construction = 64);

-- Create a function to update embeddings
-- This will be called by the application after content changes
CREATE OR REPLACE FUNCTION update_search_embedding(table_name TEXT, record_id BIGINT, content_text TEXT)
RETURNS void AS $$
DECLARE
    sql_query TEXT;
BEGIN
    -- This function is a placeholder - actual embedding is done by the application
    -- The app will generate embeddings via LiteLLM and update the column directly
    NULL;
END;
$$ LANGUAGE plpgsql;

-- Add comment for documentation
COMMENT ON COLUMN messages.embedding IS 'Semantic embedding for search (384-dim vector, cosine similarity)';
COMMENT ON COLUMN posts.embedding IS 'Semantic embedding for search (384-dim vector, cosine similarity)';
COMMENT ON COLUMN tasks.embedding IS 'Semantic embedding for search (384-dim vector, cosine similarity)';
COMMENT ON COLUMN files.embedding IS 'Semantic embedding for search (384-dim vector, cosine similarity)';
