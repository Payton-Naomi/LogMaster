ALTER TABLE logmaster_api.log_uploads
    ADD COLUMN IF NOT EXISTS test_task_id VARCHAR(128),
    ADD COLUMN IF NOT EXISTS test_task_name VARCHAR(256) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS uploader_name VARCHAR(128) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS uploader_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS remark TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS client_request_id VARCHAR(128),
    ADD COLUMN IF NOT EXISTS collector_version VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS timezone VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS client_created_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS started_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS ended_at TIMESTAMPTZ;

CREATE UNIQUE INDEX IF NOT EXISTS idx_log_uploads_owner_client_request
    ON logmaster_api.log_uploads (created_by_open_id, client_request_id)
    WHERE created_by_open_id IS NOT NULL AND client_request_id IS NOT NULL;
