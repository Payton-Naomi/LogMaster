ALTER TABLE logmaster_api.projects
    ADD COLUMN IF NOT EXISTS product_line VARCHAR(32) NOT NULL DEFAULT 'vehicle',
    ADD COLUMN IF NOT EXISTS product_type VARCHAR(32) NOT NULL DEFAULT 'dashcam',
    ADD COLUMN IF NOT EXISTS stage VARCHAR(32) NOT NULL DEFAULT 'production',
    ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

UPDATE logmaster_api.projects
SET product_line = 'vehicle',
    product_type = CASE WHEN name LIKE 'RC%' THEN 'extension_camera' ELSE 'dashcam' END,
    stage = CASE
        WHEN name IN ('DR2861', 'DR2863', 'DR1810', 'DR7800', 'DR2852', 'DR2841', 'DR1210') THEN 'development'
        ELSE 'production'
    END,
    is_active = name IN (
        'DR5800', 'DR2820', 'DR2410', 'DR1400', 'DR4800', 'DR2510', 'DR2800', 'DR1800', 'DR2211', 'DR2210',
        'DR2830', 'DR3500', 'DR5400', 'DR2200-C02', 'DR6500', 'DR2840', 'DR3400', 'DR2810', 'DR1500', 'RC23',
        'RC25', 'DR2500', 'DR2821', 'DR5401', 'DR2822', 'DR2860', 'DR2420', 'DR6501', 'DR6800', 'DR6400',
        'DR2850', 'DR2520', 'DR1810', 'DR7800', 'DR2841', 'DR2852', 'DR2863', 'DR2861', 'DR1210'
    ),
    updated_at = NOW();

ALTER TABLE logmaster_api.projects
    ADD CONSTRAINT projects_product_line_check CHECK (product_line IN ('vehicle')),
    ADD CONSTRAINT projects_product_type_check CHECK (product_type IN ('dashcam', 'extension_camera')),
    ADD CONSTRAINT projects_stage_check CHECK (stage IN ('development', 'production'));
