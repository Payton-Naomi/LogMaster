CREATE TABLE IF NOT EXISTS logmaster_api.users (
    id BIGSERIAL PRIMARY KEY,
    feishu_open_id TEXT NOT NULL UNIQUE,
    name VARCHAR(128) NOT NULL,
    email TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_name
    ON logmaster_api.users (name);

CREATE OR REPLACE VIEW logmaster_api.user_openid_mapping AS
SELECT name, feishu_open_id, email, updated_at
FROM logmaster_api.users;
