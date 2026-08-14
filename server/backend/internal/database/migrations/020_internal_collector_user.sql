INSERT INTO logmaster_api.users (feishu_open_id, name, email, created_at, updated_at)
VALUES ('logmaster-internal-collector', '内部采集端', '', NOW(), NOW())
ON CONFLICT (feishu_open_id) DO UPDATE
SET name = EXCLUDED.name,
    updated_at = NOW();
