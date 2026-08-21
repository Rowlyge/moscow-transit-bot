package db

import (
"context"
"errors"
"fmt"

"github.com/jackc/pgx/v5"
"github.com/jackc/pgx/v5/pgxpool"
)

// GetCheckpoint returns the last saved $skip offset for a dataset, or
// 0 if no checkpoint exists yet (i.e. this is a fresh run).
func GetCheckpoint(ctx context.Context, pool *pgxpool.Pool, datasetName string) (int, error) {
var skip int
err := pool.QueryRow(ctx, "SELECT last_skip FROM etl_checkpoints WHERE dataset_name = $1", datasetName).Scan(&skip)
if errors.Is(err, pgx.ErrNoRows) {
return 0, nil
}
if err != nil {
return 0, fmt.Errorf("reading checkpoint for %s: %w", datasetName, err)
}
return skip, nil
}

// SaveCheckpoint upserts the last successfully processed $skip offset
// for a dataset, so a crashed run can resume from here instead of
// starting over.
func SaveCheckpoint(ctx context.Context, pool *pgxpool.Pool, datasetName string, skip int) error {
_, err := pool.Exec(ctx, `
INSERT INTO etl_checkpoints (dataset_name, last_skip, updated_at)
VALUES ($1, $2, now())
ON CONFLICT (dataset_name) DO UPDATE SET
last_skip = EXCLUDED.last_skip,
updated_at = now()
`, datasetName, skip)
if err != nil {
return fmt.Errorf("saving checkpoint for %s: %w", datasetName, err)
}
return nil
}

// ClearCheckpoint removes a dataset's checkpoint, used after a fully
// successful run so the next run starts fresh from skip=0.
func ClearCheckpoint(ctx context.Context, pool *pgxpool.Pool, datasetName string) error {
_, err := pool.Exec(ctx, "DELETE FROM etl_checkpoints WHERE dataset_name = $1", datasetName)
if err != nil {
return fmt.Errorf("clearing checkpoint for %s: %w", datasetName, err)
}
return nil
}
