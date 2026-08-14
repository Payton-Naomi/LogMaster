-- 上传码增加项目名称前缀，例如 DR2860-A1B2C3D4E5，扩长查询码字段。
ALTER TABLE logmaster_api.upload_sessions
    ALTER COLUMN query_code TYPE VARCHAR(64);

ALTER TABLE logmaster_api.log_uploads
    ALTER COLUMN query_code TYPE VARCHAR(64);
