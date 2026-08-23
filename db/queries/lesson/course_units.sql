-- name: ListUnitsByCourseID :many
SELECT *
FROM learn.course_units
WHERE course_id = $1
ORDER BY position ASC;

-- name: GetUnitByID :one
SELECT *
FROM learn.course_units
WHERE id = $1
LIMIT 1;

-- name: CreateUnit :one
INSERT INTO learn.course_units (
    course_id,
    position,
    title,
    description
) VALUES (
    $1, $2, $3, $4
) RETURNING *;
