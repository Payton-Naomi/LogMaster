ALTER TABLE logmaster_api.parse_results
    ADD COLUMN IF NOT EXISTS assigned_to_open_id VARCHAR(128),
    ADD COLUMN IF NOT EXISTS assigned_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_parse_results_assigned_to
    ON logmaster_api.parse_results (assigned_to_open_id, id);
