-- name: InsertReviewLog :one
INSERT INTO learn.review_logs (
    card_id, user_id, grade, elapsed_ms, stability_before, stability_after, difficulty_before, difficulty_after, scheduled_days, scheduler_version, reviewed_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
) RETURNING *;

-- name: ListReviewLogsByCard :many
SELECT * FROM learn.review_logs
WHERE card_id = $1 AND user_id = $2
ORDER BY reviewed_at DESC
LIMIT $3;

-- SumRecentReviewElapsedMs adds up the thinking time the learner actually spent
-- on the last `limit` cards they answered. It is bounded below by `reviewed_at`
-- so the planner touches only the partitions the session can have landed in.
-- name: SumRecentReviewElapsedMs :one
SELECT COALESCE(SUM(elapsed_ms), 0)::bigint AS total_ms
FROM (
    SELECT elapsed_ms
    FROM learn.review_logs
    WHERE user_id = $1
      AND reviewed_at >= $2
    ORDER BY reviewed_at DESC
    LIMIT $3
) AS recent;

-- name: UpsertReviewDailyStats :one
INSERT INTO learn.review_daily_stats (
    user_id, stat_date, reviews_completed, new_cards_learned, total_minutes, updated_at
) VALUES (
    $1, $2, $3, $4, $5, now()
)
ON CONFLICT (user_id, stat_date) DO UPDATE SET
    reviews_completed = learn.review_daily_stats.reviews_completed + EXCLUDED.reviews_completed,
    new_cards_learned = learn.review_daily_stats.new_cards_learned + EXCLUDED.new_cards_learned,
    total_minutes = learn.review_daily_stats.total_minutes + EXCLUDED.total_minutes,
    updated_at = now()
RETURNING *;
