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

-- name: UpsertUnit :one
-- Keyed on (course_id, position), the unique constraint the seed already relies
-- on. Position is the unit's identity within a course, so regenerating unit 2
-- rewrites unit 2 rather than appending a second one.
INSERT INTO learn.course_units (course_id, position, title, description)
VALUES ($1, $2, $3, $4)
ON CONFLICT (course_id, position) DO UPDATE
SET title       = EXCLUDED.title,
    description = EXCLUDED.description,
    updated_at  = now()
RETURNING *;
