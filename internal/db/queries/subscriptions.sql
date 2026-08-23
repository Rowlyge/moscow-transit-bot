-- name: UpsertSubscription :exec
INSERT INTO subscriptions (telegram_user_id, stop_id, route_id)
VALUES ($1, $2, $3)
ON CONFLICT (telegram_user_id, stop_id, route_id) DO NOTHING;

-- name: DeleteSubscription :exec
DELETE FROM subscriptions
WHERE telegram_user_id = $1 AND stop_id = $2 AND route_id = $3;

-- name: ListSubscriptionsForUser :many
SELECT
    s.stop_id,
    st.stop_name,
    s.route_id,
    r.route_short_name
FROM subscriptions s
JOIN stops st  ON st.stop_id = s.stop_id
JOIN routes r  ON r.route_id = s.route_id
WHERE s.telegram_user_id = $1
ORDER BY st.stop_name, r.route_short_name;
