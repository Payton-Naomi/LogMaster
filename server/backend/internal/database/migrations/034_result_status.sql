ALTER TABLE logmaster_api.parse_results
    ADD COLUMN IF NOT EXISTS status VARCHAR(24) NOT NULL DEFAULT 'pending',
    ADD COLUMN IF NOT EXISTS status_updated_by VARCHAR(128),
    ADD COLUMN IF NOT EXISTS status_updated_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

ALTER TABLE logmaster_api.parse_results DROP CONSTRAINT IF EXISTS parse_results_status_check;
ALTER TABLE logmaster_api.parse_results
    ADD CONSTRAINT parse_results_status_check CHECK (status IN ('pending', 'confirmed', 'false_positive', 'fixed', 'closed'));

CREATE INDEX IF NOT EXISTS idx_parse_results_status ON logmaster_api.parse_results (status, updated_at);
