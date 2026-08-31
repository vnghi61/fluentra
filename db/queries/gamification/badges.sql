-- name: ListBadges :many
SELECT * FROM learn.badges ORDER BY tier, code;

-- name: GetBadgeByCode :one
SELECT * FROM learn.badges WHERE code = $1;

-- name: UpsertBadge :one
-- The catalogue is authored, not user data: seeding is an upsert so re-running
-- it corrects the copy rather than failing on the unique code.
INSERT INTO learn.badges (code, name, description, criteria, tier)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (code) DO UPDATE SET
    name        = EXCLUDED.name,
    description = EXCLUDED.description,
    criteria    = EXCLUDED.criteria,
    tier        = EXCLUDED.tier
RETURNING *;

-- name: AwardBadge :one
-- BR-GAMIFICATION-06. DO NOTHING, so the evaluator can run on every event and
-- a badge already held returns no row — which is how the caller knows not to
-- publish badge_earned again.
INSERT INTO learn.badges_earned (user_id, badge_id)
VALUES ($1, $2)
ON CONFLICT (user_id, badge_id) DO NOTHING
RETURNING *;

-- name: ListEarnedBadges :many
SELECT
    b.id, b.code, b.name, b.description, b.tier,
    e.earned_at
FROM learn.badges_earned e
JOIN learn.badges b ON b.id = e.badge_id
WHERE e.user_id = $1
ORDER BY e.earned_at DESC;

-- name: ListUnearnedBadges :many
-- The catalogue minus what the learner holds: the evaluator's candidate set,
-- so a learner with every badge costs one query and no evaluation at all.
SELECT b.* FROM learn.badges b
WHERE NOT EXISTS (
    SELECT 1 FROM learn.badges_earned e
    WHERE e.badge_id = b.id AND e.user_id = $1
)
ORDER BY b.tier, b.code;
