ALTER TABLE logmaster_api.ai_analysis_config
    ADD COLUMN IF NOT EXISTS llm_api_base_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS llm_api_key_encrypted TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS llm_model VARCHAR(128) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS llm_timeout_seconds INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS llm_max_matches INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS llm_max_input_bytes INTEGER NOT NULL DEFAULT 0;

ALTER TABLE logmaster_api.ai_analysis_config
    DROP CONSTRAINT IF EXISTS ai_analysis_config_llm_timeout_check,
    DROP CONSTRAINT IF EXISTS ai_analysis_config_llm_matches_check,
    DROP CONSTRAINT IF EXISTS ai_analysis_config_llm_input_bytes_check;

ALTER TABLE logmaster_api.ai_analysis_config
    ADD CONSTRAINT ai_analysis_config_llm_timeout_check CHECK (llm_timeout_seconds = 0 OR llm_timeout_seconds BETWEEN 5 AND 600),
    ADD CONSTRAINT ai_analysis_config_llm_matches_check CHECK (llm_max_matches = 0 OR llm_max_matches BETWEEN 1 AND 5000),
    ADD CONSTRAINT ai_analysis_config_llm_input_bytes_check CHECK (llm_max_input_bytes = 0 OR llm_max_input_bytes BETWEEN 1024 AND 10485760);
