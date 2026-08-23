-- name: UpsertFavorite :exec
INSERT INTO favorites (telegram_user_id, stop_id, updated_at)
VALUES ($1, $2, now())
ON CONFLICT (telegram_user_id) DO UPDATE SET
    stop_id    = EXCLUDED.stop_id,
    updated_at = now();

-- name: GetFavorite :one
SELECT stop_id FROM favorites WHERE telegram_user_id = $1;
