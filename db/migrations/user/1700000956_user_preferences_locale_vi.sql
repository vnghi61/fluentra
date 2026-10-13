-- +goose Up
-- +goose StatementBegin

-- Vietnamese is the default interface language: the learners are Vietnamese
-- speakers learning English. Registration now writes the language the person
-- registered in, so this default is for rows written any other way.
--
-- Existing rows keep their value. 'en' there may be a choice or the old
-- default, and the two cannot be told apart; the learner can change it in
-- settings.
ALTER TABLE core.user_preferences ALTER COLUMN locale SET DEFAULT 'vi';

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE core.user_preferences ALTER COLUMN locale SET DEFAULT 'en';
-- +goose StatementEnd
