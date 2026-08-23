ALTER TABLE logmaster_api.log_uploads DROP CONSTRAINT IF EXISTS log_uploads_status_check;
ALTER TABLE logmaster_api.log_uploads
    ADD CONSTRAINT log_uploads_status_check CHECK (status IN ('uploading', 'queued', 'parsing', 'paused', 'completed', 'failed', 'cancelled'));

ALTER TABLE logmaster_api.parse_tasks DROP CONSTRAINT IF EXISTS parse_tasks_status_check;
ALTER TABLE logmaster_api.parse_tasks
    ADD CONSTRAINT parse_tasks_status_check CHECK (status IN ('queued', 'running', 'paused', 'completed', 'failed', 'cancelled'));
