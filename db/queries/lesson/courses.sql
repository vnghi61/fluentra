-- The catalogue's `level` filter asks "is this course suitable for a learner at
-- level L", which is true when L falls inside [cefr_from, cefr_to]. CEFR is an
-- ordered scale and text comparison happens to agree with it for A1..C2, but
-- only by accident of the alphabet; array_position states the order outright so
-- a future level name cannot break the comparison silently.
-- name: ListPublishedCourses :many
SELECT *
FROM learn.courses
WHERE status = 'published'
  AND (
      sqlc.narg('level')::text IS NULL
      OR array_position(ARRAY['A1', 'A2', 'B1', 'B2', 'C1', 'C2'], sqlc.narg('level')::text)
         BETWEEN array_position(ARRAY['A1', 'A2', 'B1', 'B2', 'C1', 'C2'], cefr_from)
             AND array_position(ARRAY['A1', 'A2', 'B1', 'B2', 'C1', 'C2'], cefr_to)
  )
ORDER BY cefr_from ASC, title ASC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountPublishedCourses :one
SELECT count(*)
FROM learn.courses
WHERE status = 'published'
  AND (
      sqlc.narg('level')::text IS NULL
      OR array_position(ARRAY['A1', 'A2', 'B1', 'B2', 'C1', 'C2'], sqlc.narg('level')::text)
         BETWEEN array_position(ARRAY['A1', 'A2', 'B1', 'B2', 'C1', 'C2'], cefr_from)
             AND array_position(ARRAY['A1', 'A2', 'B1', 'B2', 'C1', 'C2'], cefr_to)
  );

-- GetCourseBySlug is the authoring read: it returns a course in any state.
-- Learner-facing reads use GetPublishedCourseBySlug.
-- name: GetCourseBySlug :one
SELECT *
FROM learn.courses
WHERE slug = $1
LIMIT 1;

-- name: GetPublishedCourseBySlug :one
SELECT *
FROM learn.courses
WHERE slug = $1
  AND status = 'published'
LIMIT 1;

-- name: GetCourseByID :one
SELECT *
FROM learn.courses
WHERE id = $1
LIMIT 1;

-- name: CreateCourse :one
INSERT INTO learn.courses (
    slug,
    title,
    description,
    cefr_from,
    cefr_to,
    status,
    estimated_hours
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING *;

-- name: UpdateCourse :one
UPDATE learn.courses
SET title = $2,
    description = $3,
    cefr_from = $4,
    cefr_to = $5,
    status = $6,
    estimated_hours = $7,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpsertCourse :one
-- The generator's course. Keyed on the slug, which is what makes re-running the
-- job idempotent.
--
-- A separate query from CreateCourse on purpose: a human author creating a
-- course whose slug is taken should be told so, not have their title silently
-- overwrite somebody else's course.
INSERT INTO learn.courses (slug, title, description, cefr_from, cefr_to, status, estimated_hours)
VALUES ($1, $2, $3, $4, $5, 'published', $6)
ON CONFLICT (slug) DO UPDATE
SET title           = EXCLUDED.title,
    description     = EXCLUDED.description,
    cefr_from       = EXCLUDED.cefr_from,
    cefr_to         = EXCLUDED.cefr_to,
    estimated_hours = EXCLUDED.estimated_hours,
    updated_at      = now()
RETURNING *;
