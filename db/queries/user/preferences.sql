-- name: CreateUserPreferences :one
-- Registration writes this row with the column defaults, so a user always has
-- preferences and every read is a plain SELECT rather than a nullable join.
INSERT INTO core.user_preferences (id, user_id)
VALUES ($1, $2)
RETURNING id, user_id, locale, theme, daily_goal_minutes, notification_channels,
          quiet_hours_start, quiet_hours_end, ai_processing_opt_out, created_at, updated_at;

-- name: GetUserPreferences :one
SELECT id, user_id, locale, theme, daily_goal_minutes, notification_channels,
       quiet_hours_start, quiet_hours_end, ai_processing_opt_out, created_at, updated_at
FROM core.user_preferences
WHERE user_id = $1;

-- name: ReplaceUserPreferences :one
-- `PUT /me/preferences` replaces the whole resource, so every column is
-- written unconditionally — no COALESCE, or the PUT would behave like a PATCH.
UPDATE core.user_preferences
SET locale                = $2,
    theme                 = $3,
    daily_goal_minutes    = $4,
    notification_channels = $5,
    quiet_hours_start     = $6,
    quiet_hours_end       = $7,
    ai_processing_opt_out = $8,
    updated_at            = now()
WHERE user_id = $1
RETURNING id, user_id, locale, theme, daily_goal_minutes, notification_channels,
          quiet_hours_start, quiet_hours_end, ai_processing_opt_out, created_at, updated_at;
