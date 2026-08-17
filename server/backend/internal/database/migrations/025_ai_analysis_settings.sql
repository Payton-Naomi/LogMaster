CREATE TABLE IF NOT EXISTS logmaster_api.ai_analysis_config (
    singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
    max_files_per_task INTEGER NOT NULL CHECK (max_files_per_task BETWEEN 1 AND 500),
    daily_token_quota BIGINT NOT NULL CHECK (daily_token_quota >= 0),
    updated_by_open_id TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS logmaster_api.ai_usage (
    id BIGSERIAL PRIMARY KEY,
    user_open_id TEXT NOT NULL,
    usage_date DATE NOT NULL DEFAULT CURRENT_DATE,
    prompt_tokens BIGINT NOT NULL DEFAULT 0,
    completion_tokens BIGINT NOT NULL DEFAULT 0,
    task_id UUID,
    log_file_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ai_usage_user_date ON logmaster_api.ai_usage (user_open_id, usage_date);
