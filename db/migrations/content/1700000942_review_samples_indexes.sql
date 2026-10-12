-- +goose Up
-- +goose StatementBegin

-- content.review_samples.decided_by references core.users; the schema gate
-- refuses a foreign key with no index whose leading columns match it (WO 22
-- Stage A.5).
CREATE INDEX IF NOT EXISTS idx_review_samples_decided_by
    ON content.review_samples (decided_by);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS content.idx_review_samples_decided_by;

-- +goose StatementEnd
