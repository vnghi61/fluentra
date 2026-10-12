-- +goose Up
-- +goose StatementBegin

-- WO 22 Stage J: fixed tests are numbered and disjoint. Test N draws only
-- questions no earlier fixed test of the blueprint uses, so the tests of one
-- exam share no question.

ALTER TABLE assess.mock_tests ADD COLUMN IF NOT EXISTS number int;

-- Existing fixed tests predate numbering. Number them per blueprint, oldest
-- first, so the partial unique index below can be created.
WITH ranked AS (
    SELECT id,
           ROW_NUMBER() OVER (PARTITION BY blueprint_id ORDER BY created_at, id) AS n
    FROM assess.mock_tests
    WHERE mode = 'fixed'
)
UPDATE assess.mock_tests m
SET number = r.n
FROM ranked r
WHERE m.id = r.id;

-- A fixed test is numbered; a random, weak-topic, full or custom composition is
-- not. The database states that rather than trusting the writer.
ALTER TABLE assess.mock_tests
    ADD CONSTRAINT ck_mock_tests_fixed_number
    CHECK ((mode = 'fixed') = (number IS NOT NULL));

-- Two workers composing Test 6 end with one row.
CREATE UNIQUE INDEX IF NOT EXISTS uq_mock_tests_fixed_number
    ON assess.mock_tests (blueprint_id, number)
    WHERE mode = 'fixed';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS assess.uq_mock_tests_fixed_number;
ALTER TABLE assess.mock_tests DROP CONSTRAINT IF EXISTS ck_mock_tests_fixed_number;
ALTER TABLE assess.mock_tests DROP COLUMN IF EXISTS number;

-- +goose StatementEnd
