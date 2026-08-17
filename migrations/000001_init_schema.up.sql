CREATE EXTENSION IF NOT EXISTS postgis;

-- Остановки
CREATE TABLE stops (
    stop_id         TEXT PRIMARY KEY,
    stop_name       TEXT NOT NULL,
    stop_lat        DOUBLE PRECISION NOT NULL,
    stop_lon        DOUBLE PRECISION NOT NULL,
    transport_type  TEXT,
    street          TEXT,
    geom            GEOGRAPHY(Point, 4326)
        GENERATED ALWAYS AS (ST_MakePoint(stop_lon, stop_lat)::geography) STORED
);

CREATE INDEX idx_stops_geom ON stops USING GIST (geom);

-- Маршруты
CREATE TABLE routes (
    route_id         TEXT PRIMARY KEY,
    route_short_name TEXT NOT NULL,
    route_long_name  TEXT,
    route_type       TEXT
);

-- Календарь маршрутов
CREATE TABLE calendar (
    service_id  TEXT PRIMARY KEY,
    monday      BOOLEAN NOT NULL DEFAULT FALSE,
    tuesday     BOOLEAN NOT NULL DEFAULT FALSE,
    wednesday   BOOLEAN NOT NULL DEFAULT FALSE,
    thursday    BOOLEAN NOT NULL DEFAULT FALSE,
    friday      BOOLEAN NOT NULL DEFAULT FALSE,
    saturday    BOOLEAN NOT NULL DEFAULT FALSE,
    sunday      BOOLEAN NOT NULL DEFAULT FALSE,
    start_date  DATE NOT NULL,
    end_date    DATE NOT NULL
);

-- Рейсы маршрутов
CREATE TABLE trips (
    trip_id        TEXT PRIMARY KEY,
    route_id       TEXT NOT NULL REFERENCES routes(route_id),
    service_id     TEXT NOT NULL REFERENCES calendar(service_id),
    trip_headsign  TEXT,
    direction_id   SMALLINT
);

CREATE INDEX idx_trips_route_id ON trips (route_id);
CREATE INDEX idx_trips_service_id ON trips (service_id);

-- Расписание рейсов
CREATE TABLE stop_times (
    stop_times_id   TEXT PRIMARY KEY,
    trip_id         TEXT NOT NULL REFERENCES trips(trip_id),
    stop_id         TEXT NOT NULL REFERENCES stops(stop_id),
    arrival_time    TEXT NOT NULL,   -- GTFS-время может быть >24:00:00, храним как TEXT
    departure_time  TEXT NOT NULL,
    stop_sequence   INTEGER NOT NULL
);

CREATE INDEX idx_stop_times_stop_id ON stop_times (stop_id);
CREATE INDEX idx_stop_times_trip_id ON stop_times (trip_id);
