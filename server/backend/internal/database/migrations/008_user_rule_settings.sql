ALTER TABLE logmaster_api.parse_rules
    ADD COLUMN IF NOT EXISTS created_by_open_id TEXT
    REFERENCES logmaster_api.users (feishu_open_id)
    ON DELETE CASCADE;

ALTER TABLE logmaster_api.parse_rules
    ALTER COLUMN enabled SET DEFAULT FALSE;

UPDATE logmaster_api.parse_rules SET enabled = FALSE;

CREATE INDEX IF NOT EXISTS idx_parse_rules_created_by_open_id
    ON logmaster_api.parse_rules (created_by_open_id);

CREATE TABLE IF NOT EXISTS logmaster_api.user_rule_settings (
    feishu_open_id TEXT NOT NULL
        REFERENCES logmaster_api.users (feishu_open_id)
        ON DELETE CASCADE,
    rule_id BIGINT NOT NULL
        REFERENCES logmaster_api.parse_rules (id)
        ON DELETE CASCADE,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (feishu_open_id, rule_id)
);

CREATE INDEX IF NOT EXISTS idx_user_rule_settings_enabled
    ON logmaster_api.user_rule_settings (feishu_open_id, enabled, rule_id);
