ALTER TABLE logmaster_api.ai_analysis_config
    RENAME COLUMN max_files_per_task TO max_tokens_per_file;

ALTER TABLE logmaster_api.ai_analysis_config
    DROP CONSTRAINT IF EXISTS ai_analysis_config_max_files_per_task_check;

ALTER TABLE logmaster_api.ai_analysis_config
    ADD CONSTRAINT ai_analysis_config_max_tokens_per_file_check
    CHECK (max_tokens_per_file BETWEEN 1 AND 1000000);
