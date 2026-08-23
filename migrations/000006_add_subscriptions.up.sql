CREATE TABLE subscriptions (
    id                BIGSERIAL PRIMARY KEY,
    telegram_user_id  BIGINT NOT NULL,
    stop_id           TEXT NOT NULL REFERENCES stops(stop_id),
    route_id          TEXT NOT NULL REFERENCES routes(route_id),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (telegram_user_id, stop_id, route_id)
);

CREATE INDEX idx_subscriptions_user ON subscriptions (telegram_user_id);
