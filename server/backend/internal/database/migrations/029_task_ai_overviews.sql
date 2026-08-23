CREATE TABLE IF NOT EXISTS logmaster_api.task_ai_overviews (
    task_id UUID PRIMARY KEY REFERENCES logmaster_api.parse_tasks(id) ON DELETE CASCADE,
    provider VARCHAR(64) NOT NULL,
    status VARCHAR(24) NOT NULL CHECK (status IN ('completed', 'failed')),
    summary TEXT NOT NULL DEFAULT '',
    risk_level VARCHAR(16) NOT NULL DEFAULT 'unknown',
    risks JSONB NOT NULL DEFAULT '[]'::JSONB,
    actions JSONB NOT NULL DEFAULT '[]'::JSONB,
    error_message TEXT NOT NULL DEFAULT '',
    generated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_task_ai_overviews_generated_at
    ON logmaster_api.task_ai_overviews (generated_at DESC);

GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE logmaster_api.task_ai_overviews TO PUBLIC;
