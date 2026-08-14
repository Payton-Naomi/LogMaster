ALTER TABLE logmaster_api.projects
    DROP CONSTRAINT IF EXISTS projects_product_line_check,
    DROP CONSTRAINT IF EXISTS projects_product_type_check,
    DROP CONSTRAINT IF EXISTS projects_stage_check;

CREATE TABLE IF NOT EXISTS logmaster_api.project_taxonomies (
    id BIGSERIAL PRIMARY KEY,
    kind VARCHAR(16) NOT NULL CHECK (kind IN ('line', 'type', 'stage')),
    code VARCHAR(32) NOT NULL UNIQUE,
    name VARCHAR(64) NOT NULL,
    is_system BOOLEAN NOT NULL DEFAULT FALSE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (kind, name)
);

INSERT INTO logmaster_api.project_taxonomies (kind, code, name, is_system) VALUES
    ('line', 'vehicle', '车载线', TRUE),
    ('line', 'pet', '宠物线', TRUE),
    ('line', 'security', '安防线', TRUE),
    ('type', 'dashcam', '行车记录仪', TRUE),
    ('type', 'extension_camera', '扩展摄像头', TRUE),
    ('stage', 'development', '在研', TRUE),
    ('stage', 'production', '量产维护', TRUE)
ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name, is_active = TRUE;

CREATE INDEX IF NOT EXISTS idx_project_taxonomies_kind_active
    ON logmaster_api.project_taxonomies (kind, is_active, name);
