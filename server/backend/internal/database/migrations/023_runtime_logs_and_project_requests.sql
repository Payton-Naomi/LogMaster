CREATE TABLE IF NOT EXISTS logmaster_api.runtime_logs (
    id BIGSERIAL PRIMARY KEY,
    owner_open_id TEXT NOT NULL DEFAULT '',
    module VARCHAR(64) NOT NULL,
    event VARCHAR(128) NOT NULL,
    status VARCHAR(16) NOT NULL,
    message TEXT NOT NULL DEFAULT '',
    task_id TEXT NOT NULL DEFAULT '',
    query_code TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT runtime_logs_status_check CHECK (status IN ('success', 'failed', 'warning'))
);
CREATE INDEX IF NOT EXISTS idx_runtime_logs_owner_created ON logmaster_api.runtime_logs(owner_open_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_runtime_logs_created ON logmaster_api.runtime_logs(created_at DESC);

CREATE TABLE IF NOT EXISTS logmaster_api.project_creation_requests (
    id BIGSERIAL PRIMARY KEY,
    applicant_open_id TEXT NOT NULL REFERENCES logmaster_api.users(feishu_open_id),
    name VARCHAR(128) NOT NULL,
    product_line VARCHAR(64) NOT NULL,
    product_type VARCHAR(64) NOT NULL,
    stage VARCHAR(64) NOT NULL,
    description VARCHAR(1000) NOT NULL DEFAULT '',
    reason VARCHAR(1000) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    reviewer_open_id TEXT,
    review_comment VARCHAR(1000) NOT NULL DEFAULT '',
    reviewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT project_creation_requests_status_check CHECK (status IN ('pending', 'approved', 'rejected', 'cancelled'))
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_project_request_pending_name ON logmaster_api.project_creation_requests(name) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_project_requests_applicant ON logmaster_api.project_creation_requests(applicant_open_id, created_at DESC);
