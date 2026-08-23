ALTER TABLE logmaster_api.parse_tasks
    ADD COLUMN IF NOT EXISTS ai_status VARCHAR(24) NOT NULL DEFAULT 'disabled',
    ADD COLUMN IF NOT EXISTS ai_error_message TEXT NOT NULL DEFAULT '';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'parse_tasks_ai_status_check'
          AND connamespace = 'logmaster_api'::regnamespace
    ) THEN
        ALTER TABLE logmaster_api.parse_tasks
            ADD CONSTRAINT parse_tasks_ai_status_check
            CHECK (ai_status IN ('disabled', 'queued', 'running', 'completed', 'partial_failed', 'failed', 'cancelled'));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS logmaster_api.ai_jobs (
    id BIGSERIAL PRIMARY KEY,
    task_id UUID NOT NULL REFERENCES logmaster_api.parse_tasks(id) ON DELETE CASCADE,
    attempt_no INTEGER NOT NULL CHECK (attempt_no >= 0),
    job_type VARCHAR(16) NOT NULL CHECK (job_type IN ('file', 'overview')),
    log_file_id BIGINT REFERENCES logmaster_api.log_files(id) ON DELETE CASCADE,
    owner_open_id TEXT NOT NULL DEFAULT '',
    status VARCHAR(24) NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'running', 'completed', 'failed', 'cancelled')),
    attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    worker_id TEXT NOT NULL DEFAULT '',
    run_token UUID,
    lease_expires_at TIMESTAMPTZ,
    heartbeat_at TIMESTAMPTZ,
    error_code VARCHAR(32) NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK ((job_type = 'file' AND log_file_id IS NOT NULL) OR
           (job_type = 'overview' AND log_file_id IS NULL))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ai_jobs_identity
    ON logmaster_api.ai_jobs (task_id, attempt_no, job_type, COALESCE(log_file_id, 0));
CREATE INDEX IF NOT EXISTS idx_ai_jobs_queue
    ON logmaster_api.ai_jobs (created_at, id) WHERE status = 'queued';
CREATE INDEX IF NOT EXISTS idx_ai_jobs_expired_lease
    ON logmaster_api.ai_jobs (lease_expires_at) WHERE status = 'running';

INSERT INTO logmaster_api.ai_jobs
    (task_id, attempt_no, job_type, log_file_id, owner_open_id, status)
SELECT task.id, task.attempt_no, 'file', file.id,
       COALESCE(NULLIF(upload.uploader_id, ''), upload.created_by_open_id, ''), 'queued'
FROM logmaster_api.parse_tasks task
JOIN logmaster_api.log_uploads upload ON upload.id = task.upload_id
JOIN logmaster_api.log_files file ON file.upload_id = upload.id
WHERE task.ai_retry_requested = TRUE AND task.ai_cancel_requested = FALSE
ON CONFLICT (task_id, attempt_no, job_type, (COALESCE(log_file_id, 0))) DO NOTHING;

INSERT INTO logmaster_api.ai_jobs
    (task_id, attempt_no, job_type, log_file_id, owner_open_id, status)
SELECT task.id, task.attempt_no, 'overview', NULL,
       COALESCE(NULLIF(upload.uploader_id, ''), upload.created_by_open_id, ''), 'queued'
FROM logmaster_api.parse_tasks task
JOIN logmaster_api.log_uploads upload ON upload.id = task.upload_id
WHERE task.ai_retry_requested = TRUE AND task.ai_cancel_requested = FALSE
ON CONFLICT (task_id, attempt_no, job_type, (COALESCE(log_file_id, 0))) DO NOTHING;

UPDATE logmaster_api.parse_tasks task
SET ai_status = CASE
    WHEN task.ai_cancel_requested THEN 'cancelled'
    WHEN EXISTS (
        SELECT 1 FROM logmaster_api.ai_jobs job
        WHERE job.task_id = task.id AND job.attempt_no = task.attempt_no AND job.status = 'running'
    ) THEN 'running'
    WHEN EXISTS (
        SELECT 1 FROM logmaster_api.ai_jobs job
        WHERE job.task_id = task.id AND job.attempt_no = task.attempt_no AND job.status = 'queued'
    ) THEN 'queued'
    WHEN EXISTS (
        SELECT 1 FROM logmaster_api.agent_analyses analysis
        WHERE analysis.task_id = task.id AND analysis.attempt_no = task.attempt_no AND analysis.status = 'failed'
    ) AND EXISTS (
        SELECT 1 FROM logmaster_api.agent_analyses analysis
        WHERE analysis.task_id = task.id AND analysis.attempt_no = task.attempt_no AND analysis.status = 'completed'
    ) THEN 'partial_failed'
    WHEN EXISTS (
        SELECT 1 FROM logmaster_api.agent_analyses analysis
        WHERE analysis.task_id = task.id AND analysis.attempt_no = task.attempt_no AND analysis.status = 'failed'
    ) THEN 'failed'
    WHEN EXISTS (
        SELECT 1 FROM logmaster_api.agent_analyses analysis
        WHERE analysis.task_id = task.id AND analysis.attempt_no = task.attempt_no
    ) THEN 'completed'
    ELSE 'disabled'
END;

GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE logmaster_api.ai_jobs TO PUBLIC;
GRANT USAGE, SELECT, UPDATE ON SEQUENCE logmaster_api.ai_jobs_id_seq TO PUBLIC;
