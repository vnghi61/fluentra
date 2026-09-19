-- +goose Up
-- +goose StatementBegin

-- -------------------------------------------------------- speaking_consents
-- BR-SPEAKING-03: explicit consent is required before the first recording, and
-- is recorded with a timestamp.
--
-- The consent screen existed before this table did, and it remembered the
-- answer in the browser's localStorage. That is not a record: it is gone when
-- the learner clears site data, it never existed on any other device, and it
-- cannot answer the only questions a consent record is kept to answer — who
-- agreed, and when. Voice is biometric-adjacent personal data, so the answer has
-- to outlive a browser profile.
--
-- One row per learner. Consent is asked once and, once given, is not asked
-- again; `consented_at` is the timestamp the rule requires.
--
-- CASCADE on the user, like speaking_feedback: erasing an account erases the
-- consent it gave, and the audio objects go with it.
CREATE TABLE IF NOT EXISTS skill.speaking_consents (
    user_id       uuid          PRIMARY KEY
                                REFERENCES core.users (id) ON DELETE CASCADE,
    consented_at  timestamptz   NOT NULL DEFAULT now(),
    created_at    timestamptz   NOT NULL DEFAULT now(),
    updated_at    timestamptz   NOT NULL DEFAULT now()
);

COMMENT ON TABLE skill.speaking_consents IS
    'BR-SPEAKING-03: one row per learner who has consented to voice recording.';
COMMENT ON COLUMN skill.speaking_consents.consented_at IS
    'When consent was given. The timestamp the business rule requires.';

-- Deliberately not backfilled. Nobody who ticked the old browser-only notice
-- has a record of having done so, and writing rows for them would be inventing
-- consent. They are asked once more, and that answer is kept.

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS skill.speaking_consents;
-- +goose StatementEnd
