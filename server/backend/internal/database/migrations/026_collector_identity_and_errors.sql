ALTER TABLE logmaster_api.upload_sessions
    ADD COLUMN IF NOT EXISTS uploader_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS uploader_email VARCHAR(320) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_users_email_normalized
    ON logmaster_api.users (lower(trim(email)))
    WHERE trim(email) <> '';

UPDATE logmaster_api.upload_sessions s
SET uploader_id = COALESCE(NULLIF(s.uploader_id, ''), u.feishu_open_id),
    uploader_email = COALESCE(NULLIF(s.uploader_email, ''), lower(trim(u.email)))
FROM logmaster_api.users u
WHERE s.uploader_id = '' AND s.created_by_open_id = u.feishu_open_id;
