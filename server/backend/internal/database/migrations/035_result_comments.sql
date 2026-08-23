CREATE TABLE IF NOT EXISTS logmaster_api.parse_result_comments (
    id BIGSERIAL PRIMARY KEY,
    result_id BIGINT NOT NULL REFERENCES logmaster_api.parse_results(id) ON DELETE CASCADE,
    comment TEXT NOT NULL,
    defect_id VARCHAR(128) NOT NULL DEFAULT '',
    created_by_open_id VARCHAR(128) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT parse_result_comments_comment_not_empty CHECK (length(btrim(comment)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_parse_result_comments_result
    ON logmaster_api.parse_result_comments (result_id, created_at DESC, id DESC);
