ALTER TABLE logmaster_api.test_scenarios
    ADD COLUMN IF NOT EXISTS metadata JSONB NOT NULL DEFAULT '{}'::JSONB;
