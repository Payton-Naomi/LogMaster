ALTER TABLE logmaster_api.upload_sessions
    ADD COLUMN IF NOT EXISTS ai_analysis_enabled BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE logmaster_api.log_uploads
    ADD COLUMN IF NOT EXISTS ai_analysis_enabled BOOLEAN NOT NULL DEFAULT TRUE;
