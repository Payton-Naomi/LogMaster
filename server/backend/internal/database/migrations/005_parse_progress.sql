ALTER TABLE logmaster_api.parse_tasks
    ADD COLUMN IF NOT EXISTS total_bytes BIGINT NOT NULL DEFAULT 0 CHECK (total_bytes >= 0),
    ADD COLUMN IF NOT EXISTS processed_bytes BIGINT NOT NULL DEFAULT 0 CHECK (processed_bytes >= 0);
