-- +goose Up
-- +goose StatementBegin

-- A word one learner had rejected ("asdfgh", a proper noun) is answered for the
-- next learner from that verdict instead of a second dictionary and model call.
-- The lookup is by normalised term across every learner, newest verdict first.
CREATE INDEX IF NOT EXISTS idx_vocab_upload_items_rejected_term
    ON skill.vocab_upload_items (lower(btrim(term)), verified_at DESC)
    WHERE status = 'rejected';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS skill.idx_vocab_upload_items_rejected_term;

-- +goose StatementEnd
