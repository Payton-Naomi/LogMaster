CREATE TABLE IF NOT EXISTS logmaster_api.upload_capacity_config (
    singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
    max_upload_bytes BIGINT NOT NULL CHECK (max_upload_bytes >= 1048576),
    max_files_per_upload INTEGER NOT NULL CHECK (max_files_per_upload BETWEEN 1 AND 500),
    updated_by_open_id TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

