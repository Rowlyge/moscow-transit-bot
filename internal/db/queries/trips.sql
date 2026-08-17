-- name: UpsertTrip :exec
INSERT INTO trips (trip_id, route_id, service_id, trip_headsign, direction_id)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (trip_id) DO UPDATE SET
    route_id       = EXCLUDED.route_id,
    service_id     = EXCLUDED.service_id,
    trip_headsign  = EXCLUDED.trip_headsign,
    direction_id   = EXCLUDED.direction_id;
