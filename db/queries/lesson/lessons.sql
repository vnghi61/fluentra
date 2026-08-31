-- GetLessonByID is the authoring read: any state.
-- name: GetLessonByID :one
SELECT *
FROM learn.lessons
WHERE id = $1
LIMIT 1;

-- GetPublishedLessonByID is the learner read. It walks up to the course as well
-- as checking the lesson, because a published lesson inside a draft course is
-- not published material — the same join content's learner queries make against
-- content_items rather than trusting the version status alone.
-- name: GetPublishedLessonByID :one
SELECT l.*
FROM learn.lessons l
JOIN learn.course_units u ON u.id = l.unit_id
JOIN learn.courses c ON c.id = u.course_id
WHERE l.id = $1
  AND l.status = 'published'
  AND c.status = 'published'
LIMIT 1;

-- name: ListLessonsByUnitID :many
SELECT *
FROM learn.lessons
WHERE unit_id = $1
ORDER BY position ASC;

-- name: ListLessonsByCourseID :many
SELECT l.*
FROM learn.lessons l
JOIN learn.course_units u ON u.id = l.unit_id
WHERE u.course_id = $1
ORDER BY u.position ASC, l.position ASC;

-- name: ListPublishedLessonsByCourseID :many
SELECT l.*
FROM learn.lessons l
JOIN learn.course_units u ON u.id = l.unit_id
WHERE u.course_id = $1
  AND l.status = 'published'
ORDER BY u.position ASC, l.position ASC;

-- name: CreateLesson :one
INSERT INTO learn.lessons (
    unit_id,
    position,
    title,
    skill_focus,
    estimated_minutes,
    status
) VALUES (
    $1, $2, $3, $4, $5, $6
) RETURNING *;

-- name: UpdateLesson :one
UPDATE learn.lessons
SET title = $2,
    skill_focus = $3,
    estimated_minutes = $4,
    status = $5,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateLessonStatus :one
UPDATE learn.lessons
SET status = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateLessonDuration :one
UPDATE learn.lessons
SET estimated_minutes = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: GetNextPublishedLesson :one
-- The lesson that follows this one, for the "next lesson" control.
--
-- Two candidates, in order: the next position in the same unit, then the first
-- lesson of the next unit in the same course. The `ORDER BY` puts the
-- same-unit candidate first because a learner finishing lesson 2 of unit 1
-- expects lesson 3, not the start of unit 2 — and the LIMIT then takes it.
--
-- Published only. Offering a learner a draft lesson is offering them a 404.
SELECT l.*
FROM learn.lessons l
JOIN learn.course_units u ON u.id = l.unit_id
WHERE l.status = 'published'
  AND u.course_id = (
        SELECT cu.course_id FROM learn.course_units cu WHERE cu.id = $1
      )
  AND (
        (l.unit_id = $1 AND l.position > $2)
        OR u.position > (
             SELECT cu.position FROM learn.course_units cu WHERE cu.id = $1
           )
      )
ORDER BY (l.unit_id = $1) DESC, u.position, l.position
LIMIT 1;

-- name: UpsertLesson :one
-- Keyed on (unit_id, position), for the same reason UpsertUnit is keyed on
-- (course_id, position): the position is the lesson's identity within its unit.
INSERT INTO learn.lessons (unit_id, position, title, skill_focus, estimated_minutes, status)
VALUES ($1, $2, $3, $4, $5, 'published')
ON CONFLICT (unit_id, position) DO UPDATE
SET title             = EXCLUDED.title,
    skill_focus       = EXCLUDED.skill_focus,
    estimated_minutes = EXCLUDED.estimated_minutes,
    status            = 'published',
    updated_at        = now()
RETURNING *;
