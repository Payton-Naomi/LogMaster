ALTER TABLE logmaster_api.parse_tasks
    ADD COLUMN IF NOT EXISTS phase VARCHAR(16) NOT NULL DEFAULT 'prepare'
        CHECK (phase IN ('prepare', 'parse')),
    ADD COLUMN IF NOT EXISTS priority INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS manual_retry_requested BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS ai_retry_requested BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS attempt_no INTEGER NOT NULL DEFAULT 0 CHECK (attempt_no >= 0),
    ADD COLUMN IF NOT EXISTS worker_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS run_token UUID,
    ADD COLUMN IF NOT EXISTS lease_expires_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS heartbeat_at TIMESTAMPTZ;

UPDATE logmaster_api.parse_tasks task
SET phase = CASE
    WHEN task.total_files > 0 OR EXISTS (
        SELECT 1 FROM logmaster_api.log_files file WHERE file.upload_id = task.upload_id
    ) THEN 'parse'
    ELSE 'prepare'
END;

UPDATE logmaster_api.parse_tasks
SET attempt_no = 1
WHERE attempt_no = 0 AND status IN ('completed', 'failed');

ALTER TABLE logmaster_api.agent_analyses
    ADD COLUMN IF NOT EXISTS attempt_no INTEGER NOT NULL DEFAULT 0 CHECK (attempt_no >= 0);

UPDATE logmaster_api.agent_analyses analysis
SET attempt_no = task.attempt_no
FROM logmaster_api.parse_tasks task
WHERE task.id = analysis.task_id AND analysis.attempt_no = 0;

ALTER TABLE logmaster_api.task_ai_overviews
    ADD COLUMN IF NOT EXISTS attempt_no INTEGER NOT NULL DEFAULT 0 CHECK (attempt_no >= 0);

UPDATE logmaster_api.task_ai_overviews overview
SET attempt_no = task.attempt_no
FROM logmaster_api.parse_tasks task
WHERE task.id = overview.task_id AND overview.attempt_no = 0;

CREATE INDEX IF NOT EXISTS idx_parse_tasks_queue
    ON logmaster_api.parse_tasks (priority DESC, created_at, id)
    WHERE status = 'queued';

CREATE INDEX IF NOT EXISTS idx_parse_tasks_expired_lease
    ON logmaster_api.parse_tasks (lease_expires_at)
    WHERE status = 'running';

CREATE TABLE IF NOT EXISTS logmaster_api.parse_task_attempts (
    id BIGSERIAL PRIMARY KEY,
    task_id UUID NOT NULL REFERENCES logmaster_api.parse_tasks(id) ON DELETE CASCADE,
    attempt_no INTEGER NOT NULL CHECK (attempt_no > 0),
    run_token UUID NOT NULL UNIQUE,
    worker_id TEXT NOT NULL,
    phase VARCHAR(16) NOT NULL CHECK (phase IN ('prepare', 'parse')),
    status VARCHAR(24) NOT NULL CHECK (status IN ('running', 'completed', 'failed', 'interrupted')),
    error_message TEXT NOT NULL DEFAULT '',
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    heartbeat_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    UNIQUE (task_id, attempt_no)
);

CREATE INDEX IF NOT EXISTS idx_parse_task_attempts_task
    ON logmaster_api.parse_task_attempts (task_id, attempt_no DESC);

GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE logmaster_api.parse_task_attempts TO PUBLIC;
GRANT USAGE, SELECT, UPDATE ON SEQUENCE logmaster_api.parse_task_attempts_id_seq TO PUBLIC;
