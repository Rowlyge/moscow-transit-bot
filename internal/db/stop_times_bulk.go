package db

import (
"context"
"fmt"

"github.com/jackc/pgx/v5"
"github.com/jackc/pgx/v5/pgxpool"
)

// StopTimeStagingRow mirrors one row destined for the stop_times_staging
// table during bulk load.
type StopTimeStagingRow struct {
GlobalID       int64
StopTimesIDRaw string
TripID         string
StopID         string
ArrivalTime    string
DepartureTime  string
StopSequence   int32
}

// BulkInsertStopTimesStaging loads rows into stop_times_staging using
// COPY, which is dramatically faster than row-by-row INSERTs for large
// batches. It does not deduplicate or validate foreign keys — that
// happens in FlushStopTimesStaging.
func BulkInsertStopTimesStaging(ctx context.Context, pool *pgxpool.Pool, rows []StopTimeStagingRow) (int64, error) {
source := pgx.CopyFromSlice(len(rows), func(i int) ([]interface{}, error) {
r := rows[i]
return []interface{}{
r.GlobalID, r.StopTimesIDRaw, r.TripID, r.StopID,
r.ArrivalTime, r.DepartureTime, r.StopSequence,
}, nil
})

n, err := pool.CopyFrom(
ctx,
pgx.Identifier{"stop_times_staging"},
[]string{"global_id", "stop_times_id_raw", "trip_id", "stop_id", "arrival_time", "departure_time", "stop_sequence"},
source,
)
if err != nil {
return 0, fmt.Errorf("copying into stop_times_staging: %w", err)
}
return n, nil
}

// FlushStopTimesStaging moves all currently-staged rows into the real
// stop_times table (upserting on global_id, and silently dropping rows
// whose trip_id/stop_id don't exist yet — the known data-quality gap in
// the source), then empties the staging table for the next batch.
func FlushStopTimesStaging(ctx context.Context, pool *pgxpool.Pool) (inserted int64, dropped int64, err error) {
var stagedCount int64
if err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM stop_times_staging").Scan(&stagedCount); err != nil {
return 0, 0, fmt.Errorf("counting staged rows: %w", err)
}

tag, err := pool.Exec(ctx, `
INSERT INTO stop_times (global_id, stop_times_id_raw, trip_id, stop_id, arrival_time, departure_time, stop_sequence)
SELECT s.global_id, s.stop_times_id_raw, s.trip_id, s.stop_id, s.arrival_time, s.departure_time, s.stop_sequence
FROM stop_times_staging s
JOIN trips t ON t.trip_id = s.trip_id
JOIN stops st ON st.stop_id = s.stop_id
ON CONFLICT (global_id) DO UPDATE SET
stop_times_id_raw = EXCLUDED.stop_times_id_raw,
trip_id = EXCLUDED.trip_id,
stop_id = EXCLUDED.stop_id,
arrival_time = EXCLUDED.arrival_time,
departure_time = EXCLUDED.departure_time,
stop_sequence = EXCLUDED.stop_sequence
`)
if err != nil {
return 0, 0, fmt.Errorf("flushing staging into stop_times: %w", err)
}
inserted = tag.RowsAffected()
dropped = stagedCount - inserted

if _, err = pool.Exec(ctx, "TRUNCATE stop_times_staging"); err != nil {
return inserted, dropped, fmt.Errorf("truncating staging: %w", err)
}

return inserted, dropped, nil
}
