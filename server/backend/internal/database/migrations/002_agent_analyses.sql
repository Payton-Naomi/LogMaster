CREATE TABLE IF NOT EXISTS logmaster_api.agent_analyses (
    id BIGSERIAL PRIMARY KEY,
    task_id UUID NOT NULL REFERENCES logmaster_api.parse_tasks(id) ON DELETE CASCADE,
    log_file_id BIGINT NOT NULL REFERENCES logmaster_api.log_files(id) ON DELETE CASCADE,
    provider VARCHAR(64) NOT NULL,
    status VARCHAR(24) NOT NULL CHECK (status IN ('completed', 'failed')),
    summary TEXT NOT NULL DEFAULT '',
    findings JSONB NOT NULL DEFAULT '[]'::JSONB,
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (task_id, log_file_id, provider)
);

CREATE INDEX IF NOT EXISTS idx_agent_analyses_task_id ON logmaster_api.agent_analyses (task_id);
