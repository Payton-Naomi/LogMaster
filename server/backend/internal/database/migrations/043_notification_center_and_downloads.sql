ALTER TABLE logmaster_api.notifications
    ADD COLUMN IF NOT EXISTS result_id BIGINT REFERENCES logmaster_api.parse_results(id) ON DELETE CASCADE,
    ADD COLUMN IF NOT EXISTS dedupe_key TEXT;

ALTER TABLE logmaster_api.notifications
    DROP CONSTRAINT IF EXISTS notifications_type_check;
ALTER TABLE logmaster_api.notifications
    ADD CONSTRAINT notifications_type_check CHECK (type IN (
        'task_completed', 'task_failed', 'task_cancelled',
        'ai_completed', 'ai_failed', 'result_assigned', 'result_commented'
    ));

CREATE UNIQUE INDEX IF NOT EXISTS idx_notifications_dedupe
    ON logmaster_api.notifications (dedupe_key) WHERE dedupe_key IS NOT NULL;

CREATE TABLE IF NOT EXISTS logmaster_api.notification_settings (
    user_open_id VARCHAR(128) PRIMARY KEY REFERENCES logmaster_api.users(feishu_open_id) ON DELETE CASCADE,
    task_completed BOOLEAN NOT NULL DEFAULT TRUE,
    task_failed BOOLEAN NOT NULL DEFAULT TRUE,
    task_cancelled BOOLEAN NOT NULL DEFAULT TRUE,
    ai_completed BOOLEAN NOT NULL DEFAULT TRUE,
    ai_failed BOOLEAN NOT NULL DEFAULT TRUE,
    result_assigned BOOLEAN NOT NULL DEFAULT TRUE,
    result_commented BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

GRANT USAGE ON SCHEMA logmaster_api TO PUBLIC;
GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE logmaster_api.notifications TO PUBLIC;
GRANT USAGE, SELECT, UPDATE ON SEQUENCE logmaster_api.notifications_id_seq TO PUBLIC;
GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE logmaster_api.notification_settings TO PUBLIC;
