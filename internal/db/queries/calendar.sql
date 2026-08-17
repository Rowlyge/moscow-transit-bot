-- name: UpsertCalendar :exec
INSERT INTO calendar (service_id, monday, tuesday, wednesday, thursday, friday, saturday, sunday, start_date, end_date)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (service_id) DO UPDATE SET
    monday     = EXCLUDED.monday,
    tuesday    = EXCLUDED.tuesday,
    wednesday  = EXCLUDED.wednesday,
    thursday   = EXCLUDED.thursday,
    friday     = EXCLUDED.friday,
    saturday   = EXCLUDED.saturday,
    sunday     = EXCLUDED.sunday,
    start_date = EXCLUDED.start_date,
    end_date   = EXCLUDED.end_date;

-- name: GetActiveServiceIDsToday :many
SELECT service_id FROM calendar
WHERE start_date <= sqlc.arg(today)::date
  AND end_date >= sqlc.arg(today)::date
  AND (
    (sqlc.arg(weekday)::text = 'monday' AND monday) OR
    (sqlc.arg(weekday)::text = 'tuesday' AND tuesday) OR
    (sqlc.arg(weekday)::text = 'wednesday' AND wednesday) OR
    (sqlc.arg(weekday)::text = 'thursday' AND thursday) OR
    (sqlc.arg(weekday)::text = 'friday' AND friday) OR
    (sqlc.arg(weekday)::text = 'saturday' AND saturday) OR
    (sqlc.arg(weekday)::text = 'sunday' AND sunday)
  );
