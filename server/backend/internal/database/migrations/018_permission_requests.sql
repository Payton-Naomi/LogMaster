CREATE TABLE IF NOT EXISTS logmaster_api.permission_requests (
    id BIGSERIAL PRIMARY KEY,
    applicant_user_id BIGINT NOT NULL REFERENCES logmaster_api.users (id),
    applicant_open_id TEXT NOT NULL,
    source_role VARCHAR(32) NOT NULL,
    requested_role VARCHAR(32) NOT NULL,
    reason VARCHAR(1000) NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    reviewer_open_id TEXT,
    review_comment VARCHAR(1000) NOT NULL DEFAULT '',
    reviewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT permission_requests_source_role_check
        CHECK (source_role IN ('user', 'developer', 'admin', 'super_admin')),
    CONSTRAINT permission_requests_requested_role_check
        CHECK (requested_role IN ('user', 'developer', 'admin', 'super_admin')),
    CONSTRAINT permission_requests_status_check
        CHECK (status IN ('pending', 'approved', 'rejected', 'cancelled'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_permission_requests_one_pending
    ON logmaster_api.permission_requests (applicant_user_id)
    WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_permission_requests_review
    ON logmaster_api.permission_requests (status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_permission_requests_applicant
    ON logmaster_api.permission_requests (applicant_user_id, created_at DESC);
