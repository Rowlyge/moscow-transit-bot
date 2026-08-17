-- name: UpsertStop :exec
INSERT INTO stops (stop_id, stop_name, stop_lat, stop_lon, transport_type, street)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (stop_id) DO UPDATE SET
    stop_name      = EXCLUDED.stop_name,
    stop_lat       = EXCLUDED.stop_lat,
    stop_lon       = EXCLUDED.stop_lon,
    transport_type = EXCLUDED.transport_type,
    street         = EXCLUDED.street;

-- name: GetStopByID :one
SELECT * FROM stops WHERE stop_id = $1;

-- name: FindNearestStops :many
SELECT
    stop_id,
    stop_name,
    stop_lat,
    stop_lon,
    transport_type,
    street,
    ST_Distance(geom, ST_MakePoint(sqlc.arg(lon)::float8, sqlc.arg(lat)::float8)::geography) AS distance_meters
FROM stops
WHERE transport_type = 'Автобус'
ORDER BY geom <-> ST_MakePoint(sqlc.arg(lon)::float8, sqlc.arg(lat)::float8)::geography
LIMIT sqlc.arg(limit_count);
