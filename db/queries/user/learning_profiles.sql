-- name: CreateLearningProfile :one
INSERT INTO core.learning_profiles (id, user_id, declared_level, target_level, target_exam,
                                    weekly_minutes_goal, motivations)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, user_id, declared_level, target_level, target_exam, weekly_minutes_goal,
          motivations, created_at, updated_at;

-- name: GetLearningProfileByUserID :one
SELECT id, user_id, declared_level, target_level, target_exam, weekly_minutes_goal,
       motivations, created_at, updated_at
FROM core.learning_profiles
WHERE user_id = $1;

-- name: ReplaceLearningProfile :one
UPDATE core.learning_profiles
SET declared_level      = $2,
    target_level        = $3,
    target_exam         = $4,
    weekly_minutes_goal = $5,
    motivations         = $6,
    updated_at          = now()
WHERE user_id = $1
RETURNING id, user_id, declared_level, target_level, target_exam, weekly_minutes_goal,
          motivations, created_at, updated_at;
