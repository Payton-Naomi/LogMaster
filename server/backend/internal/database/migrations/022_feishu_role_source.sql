ALTER TABLE logmaster_api.users
    ADD COLUMN IF NOT EXISTS job_title TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS role_source VARCHAR(16) NOT NULL DEFAULT 'feishu';

ALTER TABLE logmaster_api.users
    DROP CONSTRAINT IF EXISTS users_role_source_check;

ALTER TABLE logmaster_api.users
    ADD CONSTRAINT users_role_source_check CHECK (role_source IN ('feishu', 'manual'));
