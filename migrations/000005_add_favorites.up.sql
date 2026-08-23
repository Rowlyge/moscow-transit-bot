CREATE TABLE favorites (
    telegram_user_id BIGINT PRIMARY KEY,
    stop_id          TEXT NOT NULL REFERENCES stops(stop_id),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
