CREATE TABLE etl_checkpoints (
    dataset_name TEXT PRIMARY KEY,
    last_skip    INTEGER NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
