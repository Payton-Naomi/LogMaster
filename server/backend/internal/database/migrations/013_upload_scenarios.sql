ALTER TABLE logmaster_api.log_uploads
    ADD COLUMN IF NOT EXISTS scenario_id VARCHAR(64),
    ADD COLUMN IF NOT EXISTS scenario_snapshot JSONB NOT NULL DEFAULT '{}'::JSONB;

CREATE INDEX IF NOT EXISTS idx_log_uploads_scenario_id
    ON logmaster_api.log_uploads (scenario_id)
    WHERE scenario_id IS NOT NULL;
