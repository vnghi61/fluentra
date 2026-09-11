-- +goose Up
-- +goose StatementBegin
ALTER TABLE learn.attempts DROP CONSTRAINT IF EXISTS fk_attempts_activity;
ALTER TABLE learn.attempts ADD CONSTRAINT fk_attempts_activity FOREIGN KEY (activity_id) REFERENCES learn.activities (id) ON DELETE RESTRICT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE learn.attempts DROP CONSTRAINT IF EXISTS fk_attempts_activity;
ALTER TABLE learn.attempts ADD CONSTRAINT fk_attempts_activity FOREIGN KEY (activity_id) REFERENCES learn.activities (id) ON DELETE CASCADE;
-- +goose StatementEnd
