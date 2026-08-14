CREATE TABLE IF NOT EXISTS logmaster_api.upload_sessions (
    id UUID PRIMARY KEY,
    query_code VARCHAR(10) NOT NULL UNIQUE,
    created_by_open_id TEXT NOT NULL,
    client_request_id VARCHAR(128) NOT NULL,
    project_id BIGINT NOT NULL REFERENCES logmaster_api.projects(id),
    project_name VARCHAR(128) NOT NULL,
    version VARCHAR(64) NOT NULL,
    test_task_id VARCHAR(128) NOT NULL DEFAULT '',
    test_task_name VARCHAR(256) NOT NULL DEFAULT '',
    uploader_name VARCHAR(128) NOT NULL,
    config_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    storage_root TEXT NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'closed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    closed_at TIMESTAMPTZ,
    UNIQUE (created_by_open_id, client_request_id)
);

ALTER TABLE logmaster_api.log_uploads
    ADD COLUMN IF NOT EXISTS upload_session_id UUID REFERENCES logmaster_api.upload_sessions(id);

INSERT INTO logmaster_api.upload_sessions (
    id, query_code, created_by_open_id, client_request_id, project_id, project_name,
    version, test_task_id, test_task_name, uploader_name, config_snapshot, storage_root,
    status, created_at, updated_at, closed_at
)
SELECT u.id, u.query_code, COALESCE(u.created_by_open_id, ''),
       'legacy-' || u.id::text, u.project_id, p.name, u.version,
       COALESCE(u.test_task_id, ''), u.test_task_name, u.uploader_name,
       jsonb_build_object('legacy', true), u.storage_path, 'closed',
       u.created_at, u.updated_at, u.updated_at
FROM logmaster_api.log_uploads u
JOIN logmaster_api.projects p ON p.id = u.project_id
WHERE u.query_code IS NOT NULL
ON CONFLICT (id) DO NOTHING;

UPDATE logmaster_api.log_uploads
SET upload_session_id = id
WHERE query_code IS NOT NULL AND upload_session_id IS NULL;

DROP INDEX IF EXISTS logmaster_api.idx_log_uploads_query_code;
CREATE INDEX IF NOT EXISTS idx_log_uploads_query_code
    ON logmaster_api.log_uploads (query_code)
    WHERE query_code IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_log_uploads_upload_session
    ON logmaster_api.log_uploads (upload_session_id, created_at);
