CREATE TABLE IF NOT EXISTS logmaster_api.archive_passwords (
    id BIGSERIAL PRIMARY KEY,
    password TEXT NOT NULL,
    created_by_open_id VARCHAR(128) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT archive_passwords_password_not_empty CHECK (length(btrim(password)) > 0),
    CONSTRAINT archive_passwords_password_unique UNIQUE (password)
);

CREATE INDEX IF NOT EXISTS idx_archive_passwords_updated_at
    ON logmaster_api.archive_passwords (updated_at DESC, id DESC);
