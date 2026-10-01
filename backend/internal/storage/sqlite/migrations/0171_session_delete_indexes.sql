-- +goose Up
-- +goose StatementBegin
-- Seed-session rollback deletes change_log rows directly by session_id and
-- deleting the session cascades into conversation_provider_events. Both tables
-- can become large on long-lived installs, so keep those cleanup predicates
-- indexed instead of scanning provider history / CDC retention state.
CREATE INDEX IF NOT EXISTS idx_conversation_provider_events_session
    ON conversation_provider_events(session_id);
CREATE INDEX IF NOT EXISTS idx_change_log_session
    ON change_log(session_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_change_log_session;
DROP INDEX IF EXISTS idx_conversation_provider_events_session;
-- +goose StatementEnd
