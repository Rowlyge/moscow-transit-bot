package main

import (
"context"
"log"

"github.com/jackc/pgx/v5/pgxpool"

"github.com/Rowlyge/moscow-transit-bot/internal/config"
db "github.com/Rowlyge/moscow-transit-bot/internal/db"
"github.com/Rowlyge/moscow-transit-bot/internal/mosru"
)

const (
datasetIDRoutes   = 60664
datasetIDCalendar = 60666
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
client := mosru.NewClient(cfg.MosAPIKey)

if err := syncRoutes(ctx, client, queries); err != nil {
log.Fatalf("syncing routes: %v", err)
}

if err := syncCalendar(ctx, client, queries); err != nil {
log.Fatalf("syncing calendar: %v", err)
}

log.Println("ETL run complete")
}

func syncRoutes(ctx context.Context, client *mosru.Client, queries *db.Queries) error {
log.Println("fetching routes...")

rows, err := client.FetchRows(datasetIDRoutes)
if err != nil {
return err
}
log.Printf("fetched %d raw rows", len(rows))

parsed, err := mosru.ParseRoutes(rows)
if err != nil {
return err
}
log.Printf("parsed %d routes", len(parsed))

for _, r := range parsed {
err := queries.UpsertRoute(ctx, db.UpsertRouteParams{
RouteID:        r.RouteID,
RouteShortName: r.RouteShortName,
RouteLongName:  r.RouteLongName,
RouteType:      r.RouteType,
})
if err != nil {
return err
}
}

log.Printf("upserted %d routes", len(parsed))
return nil
}

func syncCalendar(ctx context.Context, client *mosru.Client, queries *db.Queries) error {
log.Println("fetching calendar...")

rows, err := client.FetchRows(datasetIDCalendar)
if err != nil {
return err
}
log.Printf("fetched %d raw rows", len(rows))

parsed, err := mosru.ParseCalendar(rows)
if err != nil {
return err
}
log.Printf("parsed %d calendar entries", len(parsed))

for _, c := range parsed {
err := queries.UpsertCalendar(ctx, db.UpsertCalendarParams{
ServiceID: c.ServiceID,
Monday:    c.Monday,
Tuesday:   c.Tuesday,
Wednesday: c.Wednesday,
Thursday:  c.Thursday,
Friday:    c.Friday,
Saturday:  c.Saturday,
Sunday:    c.Sunday,
StartDate: c.StartDate,
EndDate:   c.EndDate,
})
if err != nil {
return err
}
}

log.Printf("upserted %d calendar entries", len(parsed))
return nil
}
