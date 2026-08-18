package db

import (
"context"

"github.com/jackc/pgx/v5/pgxpool"
)

// LoadExistingIDs is a small helper for ETL data-quality filtering:
// it returns a set of all values of idColumn in table, so callers can
// pre-filter incoming records that reference IDs missing from the
// current snapshot of a dependent table.
func LoadExistingIDs(ctx context.Context, pool *pgxpool.Pool, table, idColumn string) (map[string]bool, error) {
rows, err := pool.Query(ctx, "SELECT "+idColumn+" FROM "+table)
if err != nil {
return nil, err
}
defer rows.Close()

ids := make(map[string]bool)
for rows.Next() {
var id string
if err := rows.Scan(&id); err != nil {
return nil, err
}
ids[id] = true
}
return ids, rows.Err()
}
