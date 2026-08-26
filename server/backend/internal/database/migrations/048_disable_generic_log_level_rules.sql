-- Generic ERROR/WARN matches are noisy and must not be used as parse findings.
UPDATE logmaster_api.parse_rules
SET enabled = FALSE, updated_at = NOW()
WHERE source = 'system'
  AND upper(replace(trim(keyword), ' ', '')) IN ('FATAL|ERROR', 'WARNING|WARN');
