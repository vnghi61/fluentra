-- name: UpsertClassification :one
INSERT INTO resource.classifications (
    resource_id,
    cefr_estimate,
    skill,
    node_codes,
    prompt_version,
    model,
    ai_request_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
ON CONFLICT (resource_id) DO UPDATE
SET cefr_estimate = EXCLUDED.cefr_estimate,
    skill = EXCLUDED.skill,
    node_codes = EXCLUDED.node_codes,
    prompt_version = EXCLUDED.prompt_version,
    model = EXCLUDED.model,
    ai_request_id = EXCLUDED.ai_request_id,
    created_at = now()
RETURNING resource_id, cefr_estimate, skill, node_codes, prompt_version, model, ai_request_id, created_at;

-- name: GetClassificationByResourceID :one
SELECT resource_id, cefr_estimate, skill, node_codes, prompt_version, model, ai_request_id, created_at
FROM resource.classifications
WHERE resource_id = $1;
