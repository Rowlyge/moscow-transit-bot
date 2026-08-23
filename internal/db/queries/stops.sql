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
-- transport_type can hold combined values like "Автобус, Трамвай", so we
-- match with ILIKE rather than equality (see ETL notes on dataset 60662).
SELECT
    stop_id,
    stop_name,
    stop_lat,
    stop_lon,
    transport_type,
    street,
    ST_Distance(geom, ST_MakePoint(sqlc.arg(lon)::float8, sqlc.arg(lat)::float8)::geography)::float8 AS distance_meters
FROM stops
WHERE transport_type ILIKE '%' || sqlc.arg(transport_type_filter)::text || '%'
ORDER BY geom <-> ST_MakePoint(sqlc.arg(lon)::float8, sqlc.arg(lat)::float8)::geography
LIMIT sqlc.arg(limit_count);

-- name: SearchStopsByName :many
-- Case-insensitive substring match on stop_name. Exact matches (after
-- lowercasing) are ranked first, then shorter names (more likely to be
-- what the person meant when they typed a partial name), then alphabetically
-- for stable ordering among ties.
SELECT
    stop_id,
    stop_name,
    stop_lat,
    stop_lon,
    transport_type,
    street
FROM stops
WHERE transport_type ILIKE '%' || sqlc.arg(transport_type_filter)::text || '%'
  AND stop_name ILIKE '%' || sqlc.arg(name_query)::text || '%'
ORDER BY
    (lower(stop_name) = lower(sqlc.arg(name_query)::text)) DESC,
    length(stop_name) ASC,
    stop_name ASC
LIMIT sqlc.arg(limit_count);
