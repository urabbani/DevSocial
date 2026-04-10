-- 004: AI Agents

CREATE TABLE ai_agents (
    id            BIGSERIAL PRIMARY KEY,
    workspace_id  BIGINT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name          TEXT NOT NULL DEFAULT 'devin',
    type          TEXT NOT NULL DEFAULT 'claw-code',
    system_prompt TEXT NOT NULL DEFAULT '',
    enabled       BOOLEAN NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_ai_agents_workspace ON ai_agents(workspace_id);
