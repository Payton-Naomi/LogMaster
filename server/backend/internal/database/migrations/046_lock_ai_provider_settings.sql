-- AI provider endpoint, API key, and model are server-only environment settings.
-- Remove historical database copies so they can no longer override deployment configuration.
UPDATE logmaster_api.ai_analysis_config
SET llm_api_base_url = '',
    llm_api_key_encrypted = '',
    llm_model = ''
WHERE singleton = TRUE;
