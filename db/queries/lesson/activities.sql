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
