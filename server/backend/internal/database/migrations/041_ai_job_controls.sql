ALTER TABLE logmaster_api.parse_tasks
    ADD COLUMN IF NOT EXISTS ai_cancel_requested BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE logmaster_api.agent_analyses
    ADD COLUMN IF NOT EXISTS error_code VARCHAR(32) NOT NULL DEFAULT '';

ALTER TABLE logmaster_api.task_ai_overviews
    ADD COLUMN IF NOT EXISTS error_code VARCHAR(32) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_parse_tasks_ai_cancel_requested
    ON logmaster_api.parse_tasks (ai_cancel_requested)
    WHERE ai_cancel_requested = TRUE;

