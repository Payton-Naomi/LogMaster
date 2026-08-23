CREATE TABLE IF NOT EXISTS logmaster_api.notifications (
    id BIGSERIAL PRIMARY KEY,
    recipient_open_id VARCHAR(128) NOT NULL,
    task_id UUID REFERENCES logmaster_api.parse_tasks(id) ON DELETE CASCADE,
    upload_id UUID REFERENCES logmaster_api.log_uploads(id) ON DELETE CASCADE,
    type VARCHAR(32) NOT NULL,
    title VARCHAR(256) NOT NULL,
    message TEXT NOT NULL DEFAULT '',
    is_read BOOLEAN NOT NULL DEFAULT FALSE,
    read_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT notifications_type_check CHECK (type IN ('task_completed', 'task_failed', 'task_cancelled'))
);

CREATE INDEX IF NOT EXISTS idx_notifications_recipient_created
    ON logmaster_api.notifications (recipient_open_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_recipient_unread
    ON logmaster_api.notifications (recipient_open_id, is_read, created_at DESC);
