ALTER TABLE logmaster_api.log_uploads
    ADD COLUMN IF NOT EXISTS created_by_open_id TEXT;

UPDATE logmaster_api.log_uploads
SET created_by_open_id = (SELECT MIN(feishu_open_id) FROM logmaster_api.users)
WHERE created_by_open_id IS NULL
  AND (SELECT COUNT(*) FROM logmaster_api.users) = 1;

ALTER TABLE logmaster_api.log_uploads
    DROP CONSTRAINT IF EXISTS log_uploads_created_by_open_id_fkey;

ALTER TABLE logmaster_api.log_uploads
    ADD CONSTRAINT log_uploads_created_by_open_id_fkey
    FOREIGN KEY (created_by_open_id)
    REFERENCES logmaster_api.users (feishu_open_id)
    ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_log_uploads_created_by_open_id
    ON logmaster_api.log_uploads (created_by_open_id);
