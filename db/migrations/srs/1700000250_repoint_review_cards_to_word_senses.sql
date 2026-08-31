-- +goose Up
-- +goose StatementBegin

-- Point existing review cards at the word, not at the exercise that taught it.
--
-- The vocabulary grader used to schedule `req.ContentVersionID` — the activity's
-- own content version, whose body is an answer key: a prompt and the strings
-- that count as correct. The review screen wants a dictionary entry, so every
-- card a learner had earned rendered as "This card has no content yet".
--
-- The grader was fixed to resolve the word instead, which corrects new cards and
-- does nothing for the ones already scheduled. This is those.
--
-- Three activities in a lesson can all be about "habit", so three cards collapse
-- into one — which is right, a learner reviews the word once — but the unique
-- constraint on (user_id, content_version_id) means the merge has to be
-- deliberate rather than an UPDATE that fails halfway.
--
-- Wrapped in a DO block because sqlc parses this directory to build its schema
-- and cannot follow temporary tables; inside a block it sees one opaque
-- statement, which is what it should see for a data backfill.
DO $$
BEGIN
    -- 1. Which activity version stands for which dictionary entry.
    --
    -- The lemma is the authored answer. `correct_answer` first, then the
    -- acceptable list, because a multiple-choice activity's key is an option id
    -- — "opt_habit", which no dictionary has — while the lemma sits beside it in
    -- `acceptable`. Versions that already carry `word` are dictionary entries
    -- themselves and are left alone.
    CREATE TEMPORARY TABLE remap ON COMMIT DROP AS
    WITH candidates AS (
        SELECT cv.id AS activity_version_id,
               lower(btrim(c.candidate)) AS lemma,
               c.ordinality AS preference
        FROM content.content_versions cv
        CROSS JOIN LATERAL unnest(
            array_remove(
                ARRAY[cv.body ->> 'correct_answer']
                || CASE
                     WHEN jsonb_typeof(cv.body -> 'acceptable') = 'array'
                     THEN ARRAY(SELECT jsonb_array_elements_text(cv.body -> 'acceptable'))
                     ELSE ARRAY[]::text[]
                   END,
                NULL)
        ) WITH ORDINALITY AS c(candidate, ordinality)
        WHERE cv.body ? 'correct_answer'
          AND NOT (cv.body ? 'word')
    )
    SELECT DISTINCT ON (c.activity_version_id)
           c.activity_version_id,
           s.content_version_id AS sense_version_id
    FROM candidates c
    JOIN skill.words w ON lower(w.lemma) = c.lemma
    JOIN skill.word_senses s ON s.word_id = w.id AND s.content_version_id IS NOT NULL
    WHERE c.lemma <> ''
    ORDER BY c.activity_version_id, c.preference, w.frequency_rank NULLS LAST, w.pos;

    -- 2. For each learner and target word, the card whose history is worth
    --    keeping. Most reviews first: that card carries the most real FSRS
    --    signal. Earliest due breaks a tie, so the merge never pushes a review
    --    further away than the learner already expected it.
    CREATE TEMPORARY TABLE keepers ON COMMIT DROP AS
    SELECT DISTINCT ON (rc.user_id, r.sense_version_id)
           rc.id AS keeper_id,
           rc.user_id,
           r.sense_version_id
    FROM learn.review_cards rc
    JOIN remap r ON r.activity_version_id = rc.content_version_id
    ORDER BY rc.user_id, r.sense_version_id, rc.reps DESC, rc.due_at ASC, rc.id;

    -- 3. The cards being merged away, and where their history goes.
    CREATE TEMPORARY TABLE losers ON COMMIT DROP AS
    SELECT rc.id AS loser_id, k.keeper_id
    FROM learn.review_cards rc
    JOIN remap r ON r.activity_version_id = rc.content_version_id
    JOIN keepers k ON k.user_id = rc.user_id AND k.sense_version_id = r.sense_version_id
    WHERE rc.id <> k.keeper_id;

    -- 4. The review history follows the card it is merged into. review_logs has
    --    no foreign key to review_cards — the table is partitioned — so deleting
    --    a card would otherwise strand its logs rather than fail loudly.
    UPDATE learn.review_logs l
    SET card_id = losers.keeper_id
    FROM losers
    WHERE l.card_id = losers.loser_id;

    DELETE FROM learn.review_cards rc USING losers WHERE rc.id = losers.loser_id;

    -- 5. The survivors now point at the dictionary entry.
    UPDATE learn.review_cards rc
    SET content_version_id = k.sense_version_id,
        updated_at         = now()
    FROM keepers k
    WHERE rc.id = k.keeper_id
      AND rc.content_version_id <> k.sense_version_id;
END
$$;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Deliberately not reversible.
--
-- Merging three cards into one destroys the two schedules that lost, and no
-- record of them survives to rebuild. Pointing the survivors back at activity
-- versions would also restore the exact fault this migration exists to remove:
-- cards a learner cannot read. Rolling back leaves them readable, which is the
-- safe direction.
SELECT 1;

-- +goose StatementEnd
