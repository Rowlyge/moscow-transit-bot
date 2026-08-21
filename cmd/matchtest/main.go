package main

import (
"context"
"encoding/json"
"log"

"github.com/jackc/pgx/v5/pgxpool"

"github.com/Rowlyge/moscow-transit-bot/internal/config"
db "github.com/Rowlyge/moscow-transit-bot/internal/db"
"github.com/Rowlyge/moscow-transit-bot/internal/matching"
)

func main() {
cfg, err := config.Load()
if err != nil {
log.Fatalf("loading config: %v", err)
}

ctx := context.Background()
pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
if err != nil {
log.Fatalf("connecting to database: %v", err)
}
defer pool.Close()

queries := db.New(pool)

results, err := matching.FindNearbyArrivals(ctx, queries, matching.NearbyArrivalsParams{
Lat:                55.566212,
Lon:                37.406062,
MaxStops:           5,
MaxArrivalsPerStop: 3,
})
if err != nil {
log.Fatalf("matching failed: %v", err)
}

out, _ := json.MarshalIndent(results, "", "  ")
log.Println(string(out))
}
