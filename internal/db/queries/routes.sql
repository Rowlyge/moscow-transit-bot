-- name: UpsertRoute :exec
INSERT INTO routes (route_id, route_short_name, route_long_name, route_type)
VALUES ($1, $2, $3, $4)
ON CONFLICT (route_id) DO UPDATE SET
    route_short_name = EXCLUDED.route_short_name,
    route_long_name   = EXCLUDED.route_long_name,
    route_type        = EXCLUDED.route_type;
