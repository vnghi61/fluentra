-- name: ListExamVersions :many
SELECT * FROM assess.exam_versions
ORDER BY is_current DESC, code ASC;

-- name: ListCurrentExamVersions :many
SELECT * FROM assess.exam_versions
WHERE is_current = true
ORDER BY code ASC;

-- name: GetExamVersionByID :one
SELECT * FROM assess.exam_versions
WHERE id = $1;

-- name: GetExamVersionByCode :one
SELECT * FROM assess.exam_versions
WHERE code = $1;

-- name: ListExamPartsByVersionID :many
SELECT * FROM assess.exam_parts
WHERE version_id = $1
ORDER BY section ASC, part_number ASC;

-- name: GetExamPartByID :one
SELECT * FROM assess.exam_parts
WHERE id = $1;

-- name: ListBlueprintsByVersionID :many
SELECT * FROM assess.blueprints
WHERE version_id = $1
ORDER BY name ASC;

-- name: GetBlueprintByID :one
SELECT * FROM assess.blueprints
WHERE id = $1;

-- name: GetBlueprintByName :one
SELECT * FROM assess.blueprints
WHERE version_id = $1 AND name = $2;
