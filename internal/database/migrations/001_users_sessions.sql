-- 001: Users and Sessions (evolved from SQLite schema)
-- Auth: GitHub OAuth

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
    id              BIGSERIAL PRIMARY KEY,
    github_id       BIGINT UNIQUE NOT NULL,
    username        TEXT NOT NULL,
    display_name    TEXT NOT NULL DEFAULT '',
    avatar_url      TEXT NOT NULL DEFAULT '',
    bio             TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    id          BIGSERIAL PRIMARY KEY,
    token       TEXT UNIQUE NOT NULL,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at  TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_sessions_token ON sessions(token);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);
