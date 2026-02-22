CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email text NOT NULL UNIQUE,
    password_hash text NOT NULL,
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS roles (
    id serial PRIMARY KEY,
    name text NOT NULL UNIQUE
);

INSERT INTO roles (name) VALUES ('user'), ('admin')
ON CONFLICT (name) DO NOTHING;

CREATE TABLE IF NOT EXISTS user_roles (
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id int NOT NULL REFERENCES roles(id) ON DELETE RESTRICT,
    PRIMARY KEY (user_id, role_id)
);

CREATE INDEX IF NOT EXISTS idx_user_roles_user_id ON user_roles(user_id);
CREATE INDEX IF NOT EXISTS idx_user_roles_role_id ON user_roles(role_id);

CREATE TABLE IF NOT EXISTS user_sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token_hash text NOT NULL,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz NULL
);

CREATE INDEX IF NOT EXISTS idx_user_sessions_user_id ON user_sessions(user_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_user_sessions_refresh_token_hash ON user_sessions(refresh_token_hash);

CREATE TABLE IF NOT EXISTS texts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_texts_user_id_created_at ON texts(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS analyses (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    text_id uuid NOT NULL REFERENCES texts(id) ON DELETE CASCADE,
    status text NOT NULL,
    model_version text NOT NULL,
    threshold numeric NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    started_at timestamptz NULL,
    finished_at timestamptz NULL,
    error_message text NULL,
    CONSTRAINT chk_analysis_status CHECK (status IN ('queued', 'running', 'done', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_analyses_user_id_created_at ON analyses(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_analyses_status ON analyses(status);
CREATE INDEX IF NOT EXISTS idx_analyses_text_id ON analyses(text_id);

CREATE TABLE IF NOT EXISTS analysis_results (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    analysis_id uuid NOT NULL UNIQUE REFERENCES analyses(id) ON DELETE CASCADE,
    label text NOT NULL,
    score numeric NOT NULL,
    confidence numeric NOT NULL,
    explanation_json jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_analysis_results_label ON analysis_results(label);

CREATE TABLE IF NOT EXISTS audit_logs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_user_id uuid NULL REFERENCES users(id) ON DELETE SET NULL,
    action text NOT NULL,
    entity_type text NOT NULL,
    entity_id uuid NOT NULL,
    meta_json jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    ip text NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_actor_created_at ON audit_logs(actor_user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action);

CREATE TABLE IF NOT EXISTS model_settings (
    id smallint PRIMARY KEY CHECK (id = 1),
    default_model_version text NOT NULL,
    default_threshold numeric NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO model_settings (id, default_model_version, default_threshold)
VALUES (1, 'baseline', 0.5)
ON CONFLICT (id) DO NOTHING;

