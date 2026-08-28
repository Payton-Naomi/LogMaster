ALTER TABLE logmaster_api.users
    ADD COLUMN IF NOT EXISTS identity_type VARCHAR(16) NOT NULL DEFAULT 'feishu',
    ADD COLUMN IF NOT EXISTS external_company VARCHAR(128) NOT NULL DEFAULT '';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'users_identity_type_check'
          AND conrelid = 'logmaster_api.users'::regclass
    ) THEN
        ALTER TABLE logmaster_api.users
            ADD CONSTRAINT users_identity_type_check
            CHECK (identity_type IN ('feishu', 'external'));
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS logmaster_api.external_password_credentials (
    user_id BIGINT PRIMARY KEY REFERENCES logmaster_api.users(id) ON DELETE CASCADE,
    password_hash TEXT NOT NULL,
    password_updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_external_users_email_normalized
    ON logmaster_api.users (lower(trim(email)))
    WHERE identity_type = 'external' AND trim(email) <> '';
