-- name: UpsertStopTime :exec
INSERT INTO stop_times (global_id, stop_times_id_raw, trip_id, stop_id, arrival_time, departure_time, stop_sequence)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (global_id) DO UPDATE SET
    stop_times_id_raw = EXCLUDED.stop_times_id_raw,
    trip_id            = EXCLUDED.trip_id,
    stop_id             = EXCLUDED.stop_id,
    arrival_time        = EXCLUDED.arrival_time,
    departure_time       = EXCLUDED.departure_time,
    stop_sequence         = EXCLUDED.stop_sequence;

-- name: GetUpcomingArrivalsForStop :many
-- arrival_time is stored as TEXT (HH:MM:SS, zero-padded, can exceed 24:00:00
-- for trips spanning midnight per GTFS spec), so lexicographic comparison
-- and ordering work correctly here without casting to a time type.
SELECT
    st.stop_id,
    st.arrival_time,
    st.departure_time,
    r.route_id,
    r.route_short_name,
    t.trip_headsign,
    t.direction_id
FROM stop_times st
JOIN trips t  ON t.trip_id = st.trip_id
JOIN routes r ON r.route_id = t.route_id
WHERE st.stop_id = sqlc.arg(stop_id)
  AND t.service_id = ANY(sqlc.arg(active_service_ids)::text[])
  AND st.arrival_time >= sqlc.arg(current_time_str)
ORDER BY st.arrival_time
LIMIT sqlc.arg(limit_count);
