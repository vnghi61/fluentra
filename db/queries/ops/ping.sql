-- name: Ping :one
-- The infrastructure ping endpoint deliberately uses this trivial query to
-- verify PostgreSQL connectivity. Keeping it in sqlc makes the initial
-- generator configuration executable before a business module owns tables.
SELECT 1;
