ALTER TABLE logmaster_api.parse_tasks
    ADD COLUMN IF NOT EXISTS ai_retry_requested BOOLEAN NOT NULL DEFAULT FALSE;
