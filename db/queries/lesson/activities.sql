-- name: ListActivitiesByLessonID :many
SELECT *
FROM learn.activities
WHERE lesson_id = $1
ORDER BY position ASC;

-- name: ListActivitiesByLessonIDs :many
SELECT *
FROM learn.activities
WHERE lesson_id = ANY($1::uuid[])
ORDER BY lesson_id, position ASC;

-- name: DeleteActivitiesByLessonID :exec
DELETE FROM learn.activities
WHERE lesson_id = $1;

-- name: CreateActivity :one
INSERT INTO learn.activities (
    lesson_id,
    position,
    kind,
    content_version_id,
    config,
    weight
) VALUES (
    $1, $2, $3, $4, $5, $6
) RETURNING *;

-- name: ListLessonIDsByContentVersionID :many
SELECT DISTINCT lesson_id
FROM learn.activities
WHERE content_version_id = $1;

-- name: ResolveActivityHierarchy :one
SELECT
    a.id AS activity_id,
    a.lesson_id,
    a.kind AS activity_kind,
    a.content_version_id,
    a.config AS activity_config,
    a.weight AS activity_weight,
    l.unit_id,
    l.skill_focus AS lesson_skill_focus,
    u.course_id
FROM learn.activities a
JOIN learn.lessons l ON l.id = a.lesson_id
JOIN learn.course_units u ON u.id = l.unit_id
WHERE a.id = $1;

