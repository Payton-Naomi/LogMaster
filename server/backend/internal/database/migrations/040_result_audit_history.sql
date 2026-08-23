ALTER TABLE logmaster_api.parse_results
    ADD COLUMN IF NOT EXISTS assignment_updated_by VARCHAR(128);

CREATE TABLE IF NOT EXISTS logmaster_api.parse_result_audit_logs (
    id BIGSERIAL PRIMARY KEY,
    result_id BIGINT NOT NULL REFERENCES logmaster_api.parse_results(id) ON DELETE CASCADE,
    action VARCHAR(32) NOT NULL,
    actor_open_id VARCHAR(128) NOT NULL,
    old_value JSONB NOT NULL DEFAULT '{}'::JSONB,
    new_value JSONB NOT NULL DEFAULT '{}'::JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT parse_result_audit_action_check CHECK (action IN ('status_changed', 'assignment_changed', 'comment_added'))
);

CREATE INDEX IF NOT EXISTS idx_parse_result_audit_result
    ON logmaster_api.parse_result_audit_logs (result_id, created_at DESC, id DESC);

CREATE OR REPLACE FUNCTION logmaster_api.audit_parse_result_update() RETURNS TRIGGER AS $$
BEGIN
    IF NEW.status IS DISTINCT FROM OLD.status THEN
        INSERT INTO logmaster_api.parse_result_audit_logs (result_id, action, actor_open_id, old_value, new_value)
        VALUES (NEW.id, 'status_changed', COALESCE(NEW.status_updated_by, ''),
                jsonb_build_object('status', OLD.status), jsonb_build_object('status', NEW.status));
    END IF;
    IF NEW.assigned_to_open_id IS DISTINCT FROM OLD.assigned_to_open_id THEN
        INSERT INTO logmaster_api.parse_result_audit_logs (result_id, action, actor_open_id, old_value, new_value)
        VALUES (NEW.id, 'assignment_changed', COALESCE(NEW.assignment_updated_by, ''),
                jsonb_build_object('assigned_to', COALESCE(OLD.assigned_to_open_id, '')),
                jsonb_build_object('assigned_to', COALESCE(NEW.assigned_to_open_id, '')));
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS parse_result_update_audit ON logmaster_api.parse_results;
CREATE TRIGGER parse_result_update_audit AFTER UPDATE ON logmaster_api.parse_results
    FOR EACH ROW EXECUTE FUNCTION logmaster_api.audit_parse_result_update();

CREATE OR REPLACE FUNCTION logmaster_api.audit_parse_result_comment() RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO logmaster_api.parse_result_audit_logs (result_id, action, actor_open_id, new_value)
    VALUES (NEW.result_id, 'comment_added', NEW.created_by_open_id,
            jsonb_build_object('comment_id', NEW.id, 'comment', NEW.comment, 'defect_id', NEW.defect_id));
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS parse_result_comment_audit ON logmaster_api.parse_result_comments;
CREATE TRIGGER parse_result_comment_audit AFTER INSERT ON logmaster_api.parse_result_comments
    FOR EACH ROW EXECUTE FUNCTION logmaster_api.audit_parse_result_comment();
