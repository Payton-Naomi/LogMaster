CREATE TABLE IF NOT EXISTS logmaster_api.user_collected_upload_sessions (
    user_open_id TEXT NOT NULL REFERENCES logmaster_api.users(feishu_open_id) ON DELETE CASCADE,
    upload_session_id UUID NOT NULL REFERENCES logmaster_api.upload_sessions(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    accessed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_open_id, upload_session_id)
);

CREATE INDEX IF NOT EXISTS idx_user_collected_upload_sessions_session
    ON logmaster_api.user_collected_upload_sessions(upload_session_id, user_open_id);
