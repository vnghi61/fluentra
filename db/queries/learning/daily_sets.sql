-- name: GetPoolPracticeCourseID :one
SELECT id
FROM learn.courses
WHERE slug = 'pool-practice';

-- name: GetDailySet :one
SELECT id, user_id, local_date, activity_ids, created_at
FROM learn.daily_sets
WHERE user_id = $1 AND local_date = $2;

-- name: CreateDailySet :one
INSERT INTO learn.daily_sets (
    user_id, local_date, activity_ids
) VALUES (
    $1, $2, $3
) ON CONFLICT (user_id, local_date) DO UPDATE
SET activity_ids = learn.daily_sets.activity_ids
RETURNING id, user_id, local_date, activity_ids, created_at;

-- name: RecordItemExposure :exec
INSERT INTO learn.item_exposures (
    user_id, activity_id, first_served_at
) VALUES (
    $1, $2, now()
) ON CONFLICT (user_id, activity_id) DO UPDATE
SET first_served_at = now();

-- name: CountActivePoolActivitiesForSlot :one
SELECT count(*)::bigint AS active_count
FROM learn.activities a
JOIN learn.lessons l ON l.id = a.lesson_id
JOIN learn.course_units u ON u.id = l.unit_id
JOIN learn.courses c ON c.id = u.course_id
WHERE c.slug = 'pool-practice'
  AND u.title = $1
  AND a.kind = $2
  AND a.retired_at IS NULL;

-- name: ListPoolActivitiesForSlot :many
SELECT a.id, a.lesson_id, a.position, a.kind, a.content_version_id, a.config, a.weight
FROM learn.activities a
JOIN learn.lessons l ON l.id = a.lesson_id
JOIN learn.course_units u ON u.id = l.unit_id
JOIN learn.courses c ON c.id = u.course_id
WHERE c.slug = 'pool-practice'
  AND u.title = $1
  AND a.kind = $2
  AND a.retired_at IS NULL
ORDER BY a.position ASC;

-- name: ListUnseenPoolActivitiesForSlot :many
SELECT a.id, a.lesson_id, a.position, a.kind, a.content_version_id, a.config, a.weight
FROM learn.activities a
JOIN learn.lessons l ON l.id = a.lesson_id
JOIN learn.course_units u ON u.id = l.unit_id
JOIN learn.courses c ON c.id = u.course_id
LEFT JOIN learn.item_exposures e ON e.activity_id = a.id AND e.user_id = $3
WHERE c.slug = 'pool-practice'
  AND u.title = $1
  AND a.kind = $2
  AND a.retired_at IS NULL
  AND e.activity_id IS NULL
ORDER BY random();

-- name: ListSeenPoolActivitiesForSlotOldestFirst :many
SELECT a.id, a.lesson_id, a.position, a.kind, a.content_version_id, a.config, a.weight, e.first_served_at
FROM learn.activities a
JOIN learn.lessons l ON l.id = a.lesson_id
JOIN learn.course_units u ON u.id = l.unit_id
JOIN learn.courses c ON c.id = u.course_id
JOIN learn.item_exposures e ON e.activity_id = a.id AND e.user_id = $3
WHERE c.slug = 'pool-practice'
  AND u.title = $1
  AND a.kind = $2
  AND a.retired_at IS NULL
ORDER BY e.first_served_at ASC;

-- name: HasActiveUserWithFewUnseenItems :one
WITH active_users AS (
    SELECT DISTINCT user_id
    FROM learn.attempts
    WHERE created_at >= now() - interval '14 days'
),
slot_activities AS (
    SELECT a.id
    FROM learn.activities a
    JOIN learn.lessons l ON l.id = a.lesson_id
    JOIN learn.course_units u ON u.id = l.unit_id
    JOIN learn.courses c ON c.id = u.course_id
    WHERE c.slug = 'pool-practice'
      AND u.title = $1
      AND a.kind = $2
      AND a.retired_at IS NULL
),
unseen_counts AS (
    SELECT u.user_id,
           (SELECT count(*) FROM slot_activities sa
            WHERE NOT EXISTS (
                SELECT 1 FROM learn.item_exposures e
                WHERE e.user_id = u.user_id AND e.activity_id = sa.id
            )) AS unseen_count
    FROM active_users u
)
SELECT EXISTS (
    SELECT 1 FROM unseen_counts WHERE unseen_count < 10
) AS has_few_unseen;

-- name: GetPoolLessonID :one
SELECT l.id
FROM learn.lessons l
JOIN learn.course_units u ON u.id = l.unit_id
JOIN learn.courses c ON c.id = u.course_id
WHERE c.slug = 'pool-practice'
  AND u.title = $1
  AND l.title = $2
LIMIT 1;

-- name: ListActivitiesByIDs :many
SELECT id, lesson_id, position, kind, content_version_id, config, weight
FROM learn.activities
WHERE id = ANY(@activity_ids::uuid[]);
