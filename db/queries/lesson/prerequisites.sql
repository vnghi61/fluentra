-- name: ListPrerequisitesByLessonID :many
SELECT lp.lesson_id, lp.requires_lesson_id, lp.min_score, l.title AS requires_lesson_title
FROM learn.lesson_prerequisites lp
JOIN learn.lessons l ON l.id = lp.requires_lesson_id
WHERE lp.lesson_id = $1;

-- name: ListPrerequisitesForLessons :many
SELECT lp.lesson_id, lp.requires_lesson_id, lp.min_score, l.title AS requires_lesson_title
FROM learn.lesson_prerequisites lp
JOIN learn.lessons l ON l.id = lp.requires_lesson_id
WHERE lp.lesson_id = ANY($1::uuid[]);

-- name: ListAllPrerequisitesInCourse :many
SELECT lp.lesson_id, lp.requires_lesson_id
FROM learn.lesson_prerequisites lp
JOIN learn.lessons l ON l.id = lp.lesson_id
JOIN learn.course_units u ON u.id = l.unit_id
WHERE u.course_id = $1;

-- name: AddPrerequisite :exec
INSERT INTO learn.lesson_prerequisites (
    lesson_id,
    requires_lesson_id,
    min_score
) VALUES (
    $1, $2, $3
);

-- name: DeletePrerequisitesByLessonID :exec
DELETE FROM learn.lesson_prerequisites
WHERE lesson_id = $1;
