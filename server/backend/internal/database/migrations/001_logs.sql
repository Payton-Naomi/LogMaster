CREATE TABLE IF NOT EXISTS logmaster_api.projects (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(128) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS logmaster_api.log_uploads (
    id UUID PRIMARY KEY,
    project_id BIGINT NOT NULL REFERENCES logmaster_api.projects(id),
    version VARCHAR(64) NOT NULL DEFAULT '',
    status VARCHAR(24) NOT NULL CHECK (status IN ('uploading', 'queued', 'parsing', 'completed', 'failed')),
    original_name TEXT NOT NULL DEFAULT '',
    original_size BIGINT NOT NULL DEFAULT 0 CHECK (original_size >= 0),
    storage_path TEXT NOT NULL,
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS logmaster_api.log_files (
    id BIGSERIAL PRIMARY KEY,
    upload_id UUID NOT NULL REFERENCES logmaster_api.log_uploads(id) ON DELETE CASCADE,
    relative_path TEXT NOT NULL,
    size_bytes BIGINT NOT NULL CHECK (size_bytes >= 0),
    sha256 CHAR(64) NOT NULL,
    line_count BIGINT NOT NULL DEFAULT 0 CHECK (line_count >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (upload_id, relative_path)
);

CREATE TABLE IF NOT EXISTS logmaster_api.parse_tasks (
    id UUID PRIMARY KEY,
    upload_id UUID NOT NULL UNIQUE REFERENCES logmaster_api.log_uploads(id) ON DELETE CASCADE,
    status VARCHAR(24) NOT NULL CHECK (status IN ('queued', 'running', 'completed', 'failed')),
    total_files INTEGER NOT NULL DEFAULT 0,
    processed_files INTEGER NOT NULL DEFAULT 0,
    total_lines BIGINT NOT NULL DEFAULT 0,
    error_count BIGINT NOT NULL DEFAULT 0,
    warning_count BIGINT NOT NULL DEFAULT 0,
    error_message TEXT NOT NULL DEFAULT '',
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS logmaster_api.parse_results (
    id BIGSERIAL PRIMARY KEY,
    task_id UUID NOT NULL REFERENCES logmaster_api.parse_tasks(id) ON DELETE CASCADE,
    log_file_id BIGINT NOT NULL REFERENCES logmaster_api.log_files(id) ON DELETE CASCADE,
    level VARCHAR(16) NOT NULL,
    matched_text TEXT NOT NULL,
    line_number BIGINT NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_log_uploads_created_at ON logmaster_api.log_uploads (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_log_uploads_project_id ON logmaster_api.log_uploads (project_id);
CREATE INDEX IF NOT EXISTS idx_log_files_upload_id ON logmaster_api.log_files (upload_id);
CREATE INDEX IF NOT EXISTS idx_parse_results_task_id_level ON logmaster_api.parse_results (task_id, level);
