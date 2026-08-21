CREATE UNLOGGED TABLE stop_times_staging (
    global_id       BIGINT,
    stop_times_id_raw TEXT,
    trip_id         TEXT,
    stop_id         TEXT,
    arrival_time    TEXT,
    departure_time  TEXT,
    stop_sequence   INTEGER
);
