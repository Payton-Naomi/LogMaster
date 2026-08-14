ALTER TABLE logmaster_api.log_uploads
    ADD COLUMN IF NOT EXISTS query_code VARCHAR(10);

CREATE UNIQUE INDEX IF NOT EXISTS idx_log_uploads_query_code
    ON logmaster_api.log_uploads (query_code)
    WHERE query_code IS NOT NULL;
