CREATE TABLE IF NOT EXISTS logmaster_api.parse_rules (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    category VARCHAR(32) NOT NULL,
    keyword TEXT NOT NULL,
    scope VARCHAR(128) NOT NULL DEFAULT '',
    level VARCHAR(16) NOT NULL CHECK (level IN ('critical', 'warning', 'info')),
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS logmaster_api.test_scenarios (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    color VARCHAR(24) NOT NULL DEFAULT 'blue',
    judgement VARCHAR(32) NOT NULL DEFAULT 'any-error',
    checks JSONB NOT NULL DEFAULT '[]'::JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_parse_rules_category ON logmaster_api.parse_rules (category);
