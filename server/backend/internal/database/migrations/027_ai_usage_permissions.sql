-- Keep AI usage accounting writable after databases are restored or migrated
-- by a different PostgreSQL role than the backend runtime role.
GRANT USAGE ON SCHEMA logmaster_api TO PUBLIC;
GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE logmaster_api.ai_usage TO PUBLIC;
GRANT USAGE, SELECT, UPDATE ON SEQUENCE logmaster_api.ai_usage_id_seq TO PUBLIC;
