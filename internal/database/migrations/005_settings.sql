-- 005: Admin Settings (key-value store for AI provider, model, etc.)

CREATE TABLE settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Seed default settings
INSERT INTO settings (key, value) VALUES
    ('ai_model', 'claude-sonnet'),
    ('ai_fallback_model', 'gpt-4o'),
    ('ai_system_prompt', 'You are a helpful AI assistant collaborating with a team of developers and researchers.'),
    ('ai_max_context_messages', '50'),
    ('ai_temperature', '0.7');
