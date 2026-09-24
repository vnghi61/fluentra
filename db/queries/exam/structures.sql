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

-- name: CreateMockTest :one
INSERT INTO assess.mock_tests (id, blueprint_id, mode, number, seed, composition, owner_id, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetMockTestByID :one
SELECT * FROM assess.mock_tests
WHERE id = $1;

-- name: ListMockTestsByOwner :many
SELECT * FROM assess.mock_tests
WHERE owner_id = $1 OR owner_id IS NULL
ORDER BY created_at DESC;

-- name: ListFixedMockTests :many
-- The numbered fixed tests of one blueprint, in order (WO 22 Stage J).
SELECT * FROM assess.mock_tests
WHERE blueprint_id = $1 AND mode = 'fixed'
ORDER BY number;

-- name: GetExamByVersionID :one
SELECT * FROM assess.exams
WHERE version_id = $1
LIMIT 1;
