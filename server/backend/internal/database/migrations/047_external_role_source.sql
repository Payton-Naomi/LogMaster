-- 045 introduced external password accounts but the legacy role_source
-- constraint only allowed feishu/manual, which rejected external registration.
ALTER TABLE logmaster_api.users
    DROP CONSTRAINT IF EXISTS users_role_source_check;

ALTER TABLE logmaster_api.users
    ADD CONSTRAINT users_role_source_check
    CHECK (role_source IN ('feishu', 'manual', 'external'));
