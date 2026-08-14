ALTER TABLE logmaster_api.users
    ADD COLUMN IF NOT EXISTS role VARCHAR(32) NOT NULL DEFAULT 'user';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'users_role_check'
          AND conrelid = 'logmaster_api.users'::regclass
    ) THEN
        ALTER TABLE logmaster_api.users
            ADD CONSTRAINT users_role_check
            CHECK (role IN ('user', 'developer', 'admin', 'super_admin'));
    END IF;
END $$;

UPDATE logmaster_api.users
SET role = 'super_admin', updated_at = NOW()
WHERE name = '刘欣彤';

CREATE INDEX IF NOT EXISTS idx_users_role
    ON logmaster_api.users (role, name);

CREATE TABLE IF NOT EXISTS logmaster_api.user_role_audit_logs (
    id BIGSERIAL PRIMARY KEY,
    target_user_id BIGINT NOT NULL REFERENCES logmaster_api.users (id),
    target_open_id TEXT NOT NULL,
    old_role VARCHAR(32) NOT NULL,
    new_role VARCHAR(32) NOT NULL,
    changed_by_open_id TEXT NOT NULL,
    changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_role_audit_target
    ON logmaster_api.user_role_audit_logs (target_user_id, changed_at DESC);
